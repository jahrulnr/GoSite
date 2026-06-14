package logs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/system"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// WebsiteLister returns website domains for log filters.
type WebsiteLister interface {
	List(ctx context.Context) ([]sqlite.Website, error)
}

// Service tails nginx access and error logs.
type Service struct {
	logDir   string
	websites WebsiteLister
	openFile func(path string) (*os.File, error)
}

// NewService returns a log viewer service.
func NewService(logDir string, websites WebsiteLister) *Service {
	return &Service{
		logDir:   logDir,
		websites: websites,
		openFile: os.Open,
	}
}

// SiteOption is a selectable domain in the log viewer.
type SiteOption struct {
	Domain string `json:"domain"`
	Name   string `json:"name,omitempty"`
}

// TailInput selects which log file to read.
type TailInput struct {
	Domain string
	Type   string
	Tail   int
}

// TailResult is the trailing log content.
type TailResult struct {
	Domain    string   `json:"domain"`
	Type      string   `json:"type"`
	Path      string   `json:"path"`
	Lines     []string `json:"lines"`
	LineCount int      `json:"line_count,omitempty"`
	Content   string   `json:"content,omitempty"`
}

// ListSites returns domains including the global default entry.
func (s *Service) ListSites(ctx context.Context) ([]SiteOption, error) {
	out := []SiteOption{{Domain: "default"}}
	sites, err := s.websites.List(ctx)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "list websites failed", err)
	}
	for _, site := range sites {
		out = append(out, SiteOption{
			Domain: site.Domain,
			Name:   site.Name,
		})
	}
	return out, nil
}

// Tail reads the last n lines from an access or error log.
func (s *Service) Tail(_ context.Context, in TailInput) (TailResult, error) {
	logType, err := normalizeLogType(in.Type)
	if err != nil {
		return TailResult{}, err
	}
	tail := in.Tail
	if tail <= 0 {
		tail = 1000
	}
	if tail > 10000 {
		tail = 10000
	}

	path := s.resolveLogPath(strings.TrimSpace(in.Domain), logType)
	f, err := s.openFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return TailResult{}, apperror.New(apperror.CodeNotFound, "log file not found")
		}
		return TailResult{}, apperror.Wrap(apperror.CodeInternal, "open log file failed", err)
	}
	defer f.Close()

	content, err := system.ReadTailLines(f, tail)
	if err != nil {
		return TailResult{}, apperror.Wrap(apperror.CodeInternal, "read log file failed", err)
	}

	domain := strings.TrimSpace(in.Domain)
	if domain == "" {
		domain = "default"
	}

	var entries []string
	if content != "" {
		trimmed := strings.TrimRight(content, "\n")
		if trimmed != "" {
			entries = strings.Split(trimmed, "\n")
		}
	}

	return TailResult{
		Domain:    domain,
		Type:      logType,
		Path:      path,
		Lines:     entries,
		LineCount: len(entries),
		Content:   content,
	}, nil
}

func (s *Service) resolveLogPath(domain, logType string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" || domain == "default" {
		return filepath.Join(s.logDir, fmt.Sprintf("%s.log", logType))
	}
	safeDomain := strings.ReplaceAll(domain, "..", "")
	safeDomain = strings.ReplaceAll(safeDomain, string(filepath.Separator), "")
	return filepath.Join(s.logDir, fmt.Sprintf("%s-%s.log", logType, safeDomain))
}

func normalizeLogType(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "access", "accesslog":
		return "access", nil
	case "error", "errorlog":
		return "error", nil
	default:
		return "", apperror.New(apperror.CodeInvalidInput, "type must be access or error")
	}
}
