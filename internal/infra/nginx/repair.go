package nginx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jahrulnr/gosite/pkg/apperror"
)

// RepairAction records an automatic nginx config fix.
type RepairAction struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Reason string `json:"reason"`
	Fix    string `json:"fix"`
}

// RepairConfig controls automatic nginx -t recovery.
type RepairConfig struct {
	DefaultCert   string
	DefaultKey    string
	AllowPrefixes []string
	MaxAttempts   int
}

var nginxErrorLocation = regexp.MustCompile(`in ([^:\s]+):(\d+)`)
var nginxCertPath = regexp.MustCompile(`cannot load certificate "([^"]+)"`)

// TestAndRepair runs nginx -t and applies safe automatic fixes until the test passes
// or no further repair is possible.
func (r *Runner) TestAndRepair(ctx context.Context, cfg RepairConfig) ([]RepairAction, error) {
	max := cfg.MaxAttempts
	if max <= 0 {
		max = 8
	}

	var actions []RepairAction
	for attempt := 0; attempt < max; attempt++ {
		stdout, stderr, ok, err := r.runTest(ctx, r.cfg.NginxConf)
		if err != nil {
			return actions, apperror.Wrap(apperror.CodeNginxTestFailed, "nginx test failed", err)
		}
		if ok {
			return actions, nil
		}

		parsed, parseOK := parseNginxError(stderr + stdout)
		if !parseOK {
			msg := strings.TrimSpace(stderr + stdout)
			return actions, apperror.Wrap(apperror.CodeNginxTestFailed, msg, nil)
		}

		action, applied, repairErr := applyRepair(parsed, cfg)
		if repairErr != nil {
			return actions, repairErr
		}
		if !applied {
			msg := strings.TrimSpace(stderr + stdout)
			return actions, apperror.Wrap(apperror.CodeNginxTestFailed, msg, nil)
		}
		actions = append(actions, action)
	}

	return actions, apperror.New(apperror.CodeNginxTestFailed, "nginx repair exceeded max attempts")
}

func (r *Runner) runTest(ctx context.Context, confPath string) (stdout, stderr string, ok bool, err error) {
	result, runErr := r.cmd.Run(ctx, r.cfg.NginxBin, "-t", "-c", confPath)
	if runErr != nil {
		return "", "", false, runErr
	}
	combined := result.Stdout + result.Stderr
	ok = result.ExitCode == 0 && strings.Contains(combined, "syntax is ok")
	return result.Stdout, result.Stderr, ok, nil
}

type parsedNginxError struct {
	Message string
	File    string
	Line    int
}

func parseNginxError(output string) (parsedNginxError, bool) {
	output = strings.TrimSpace(output)
	if output == "" {
		return parsedNginxError{}, false
	}
	loc := nginxErrorLocation.FindStringSubmatch(output)
	if len(loc) != 3 {
		return parsedNginxError{}, false
	}
	line := 0
	_, _ = fmt.Sscanf(loc[2], "%d", &line)
	if line <= 0 {
		return parsedNginxError{}, false
	}
	return parsedNginxError{
		Message: output,
		File:    loc[1],
		Line:    line,
	}, true
}

func applyRepair(parsed parsedNginxError, cfg RepairConfig) (RepairAction, bool, error) {
	target, resolveErr := resolveRepairTarget(parsed.File)
	if resolveErr != nil {
		return RepairAction{}, false, resolveErr
	}
	if !isRepairAllowed(target, cfg.AllowPrefixes) {
		return RepairAction{}, false, nil
	}

	content, readErr := os.ReadFile(target)
	if readErr != nil {
		return RepairAction{}, false, fmt.Errorf("read config %s: %w", target, readErr)
	}

	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	msg := strings.ToLower(parsed.Message)

	var (
		updated []string
		action  RepairAction
		applied bool
	)

	switch {
	case strings.Contains(msg, "cannot load certificate"),
		strings.Contains(msg, "bio_new_file() failed"),
		strings.Contains(msg, "no such file or directory") && strings.Contains(msg, "certificate"):
		updated, applied = repairMissingCertificate(lines, parsed.Line, cfg, parsed.Message)
		action = RepairAction{File: target, Line: parsed.Line, Reason: "missing ssl certificate file", Fix: "use default self-signed certificate"}

	case strings.Contains(msg, `no "ssl_certificate" is defined`):
		updated, applied = repairUndefinedSSL(lines, parsed.Line, cfg)
		action = RepairAction{File: target, Line: parsed.Line, Reason: "ssl listen without certificate", Fix: "insert default self-signed certificate directives"}

	case strings.Contains(msg, "unknown directive"),
		strings.Contains(msg, "unknown \""),
		strings.Contains(msg, "invalid number of arguments"),
		strings.Contains(msg, "directive is not allowed here"),
		strings.Contains(msg, "unexpected"):
		updated, applied = commentOutLine(lines, parsed.Line)
		action = RepairAction{File: target, Line: parsed.Line, Reason: "unsupported or invalid directive", Fix: "comment out line"}

	case strings.Contains(msg, "duplicate listen"):
		updated, applied = commentOutLine(lines, parsed.Line)
		action = RepairAction{File: target, Line: parsed.Line, Reason: "duplicate listen options", Fix: "comment out duplicate listen line"}

	default:
		return RepairAction{}, false, nil
	}

	if !applied {
		return RepairAction{}, false, nil
	}

	if writeErr := os.WriteFile(target, []byte(strings.Join(updated, "\n")), 0o644); writeErr != nil {
		return RepairAction{}, false, fmt.Errorf("write config %s: %w", target, writeErr)
	}
	return action, true, nil
}

func resolveRepairTarget(path string) (string, error) {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved, nil
	}
	return path, nil
}

func isRepairAllowed(path string, prefixes []string) bool {
	clean, err := filepath.Abs(path)
	if err != nil {
		clean = filepath.Clean(path)
	}
	for _, prefix := range prefixes {
		prefixClean, err := filepath.Abs(prefix)
		if err != nil {
			prefixClean = filepath.Clean(prefix)
		}
		if clean == prefixClean || strings.HasPrefix(clean, prefixClean+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func repairMissingCertificate(lines []string, lineNum int, cfg RepairConfig, message string) ([]string, bool) {
	if lineNum < 1 || lineNum > len(lines) {
		return lines, false
	}
	if !certFilesExist(cfg.DefaultCert, cfg.DefaultKey) {
		return lines, false
	}

	missingPath := ""
	if m := nginxCertPath.FindStringSubmatch(message); len(m) == 2 {
		missingPath = m[1]
	}

	idx := lineNum - 1
	trimmed := strings.TrimSpace(lines[idx])
	if strings.HasPrefix(trimmed, "ssl_certificate ") && !strings.HasPrefix(trimmed, "ssl_certificate_key") {
		lines[idx] = fmt.Sprintf("\tssl_certificate %s;", cfg.DefaultCert)
		lines = setSSLKeyNear(lines, idx, cfg.DefaultKey)
		return lines, true
	}

	if missingPath != "" {
		if updated, ok := replaceSSLPath(lines, missingPath, cfg.DefaultCert, cfg.DefaultKey); ok {
			return updated, true
		}
	}

	start, end := serverBlockBounds(lines, idx)
	if start < 0 {
		return lines, false
	}
	return replaceSSLInRange(lines, start, end, cfg.DefaultCert, cfg.DefaultKey), true
}

func repairUndefinedSSL(lines []string, lineNum int, cfg RepairConfig) ([]string, bool) {
	if lineNum < 1 || lineNum > len(lines) || !certFilesExist(cfg.DefaultCert, cfg.DefaultKey) {
		return lines, false
	}
	idx := lineNum - 1
	start, end := serverBlockBounds(lines, idx)
	if start < 0 {
		return lines, false
	}
	for i := start; i <= end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "ssl_certificate ") {
			return lines, false
		}
	}
	insertAt := idx + 1
	insert := []string{
		fmt.Sprintf("\tssl_certificate %s;", cfg.DefaultCert),
		fmt.Sprintf("\tssl_certificate_key %s;", cfg.DefaultKey),
	}
	out := append([]string{}, lines[:insertAt]...)
	out = append(out, insert...)
	out = append(out, lines[insertAt:]...)
	return out, true
}

func commentOutLine(lines []string, lineNum int) ([]string, bool) {
	if lineNum < 1 || lineNum > len(lines) {
		return lines, false
	}
	idx := lineNum - 1
	trimmed := strings.TrimSpace(lines[idx])
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return lines, false
	}
	indent := lines[idx][:len(lines[idx])-len(strings.TrimLeft(lines[idx], " \t"))]
	lines[idx] = indent + "# gosite-repair: " + trimmed
	return lines, true
}

func certFilesExist(cert, key string) bool {
	if _, err := os.Stat(cert); err != nil {
		return false
	}
	if _, err := os.Stat(key); err != nil {
		return false
	}
	return true
}

func replaceSSLPath(lines []string, missingCert, defaultCert, defaultKey string) ([]string, bool) {
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, missingCert) && strings.HasPrefix(trimmed, "ssl_certificate ") && !strings.HasPrefix(trimmed, "ssl_certificate_key") {
			lines[i] = replaceIndent(line, fmt.Sprintf("ssl_certificate %s;", defaultCert))
			lines = setSSLKeyNear(lines, i, defaultKey)
			changed = true
		}
	}
	return lines, changed
}

func replaceSSLInRange(lines []string, start, end int, cert, key string) []string {
	hasCert := false
	for i := start; i <= end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "ssl_certificate ") && !strings.HasPrefix(trimmed, "ssl_certificate_key") {
			lines[i] = replaceIndent(lines[i], fmt.Sprintf("ssl_certificate %s;", cert))
			hasCert = true
		}
		if strings.HasPrefix(trimmed, "ssl_certificate_key ") {
			lines[i] = replaceIndent(lines[i], fmt.Sprintf("ssl_certificate_key %s;", key))
		}
	}
	if !hasCert {
		indent := replaceIndent(lines[start], "")
		insert := []string{
			indent + fmt.Sprintf("ssl_certificate %s;", cert),
			indent + fmt.Sprintf("ssl_certificate_key %s;", key),
		}
		out := append([]string{}, lines[:start+1]...)
		out = append(out, insert...)
		out = append(out, lines[start+1:]...)
		return out
	}
	return setSSLKeyNear(lines, start, key)
}

func setSSLKeyNear(lines []string, certIdx int, keyPath string) []string {
	_, end := serverBlockBounds(lines, certIdx)
	searchEnd := certIdx + 4
	if end >= 0 && end < searchEnd {
		searchEnd = end
	}
	for i := certIdx; i <= searchEnd && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "ssl_certificate_key ") {
			lines[i] = replaceIndent(lines[i], fmt.Sprintf("ssl_certificate_key %s;", keyPath))
			return lines
		}
	}
	insertLine := replaceIndent(lines[certIdx], fmt.Sprintf("ssl_certificate_key %s;", keyPath))
	out := append([]string{}, lines[:certIdx+1]...)
	out = append(out, insertLine)
	out = append(out, lines[certIdx+1:]...)
	return out
}

func serverBlockBounds(lines []string, idx int) (start, end int) {
	start = -1
	for i := idx; i >= 0; i-- {
		if strings.Contains(lines[i], "server") && strings.Contains(lines[i], "{") {
			start = i
			break
		}
	}
	if start < 0 {
		return -1, -1
	}
	depth := 0
	end = len(lines) - 1
	for i := start; i < len(lines); i++ {
		depth += braceDelta(lines[i])
		if i >= idx && depth <= 0 {
			end = i
			break
		}
	}
	return start, end
}

func braceDelta(line string) int {
	inSingle := false
	inDouble := false
	delta := 0
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '{':
			if !inSingle && !inDouble {
				delta++
			}
		case '}':
			if !inSingle && !inDouble {
				delta--
			}
		}
	}
	return delta
}

func replaceIndent(line, content string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	return indent + content
}
