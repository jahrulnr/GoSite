package mount

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Entry is one fstab row.
type Entry struct {
	Device  string    `json:"device"`
	Dir     string    `json:"dir"`
	Type    string    `json:"type"`
	Options string    `json:"options"`
	Dump    string    `json:"dump"`
	Fsck    string    `json:"fsck"`
	Mounted bool      `json:"mounted"`
	S3      *S3Config `json:"s3,omitempty"`
}

// Service manages fstab entries and mount operations.
type Service struct {
	fstabPath string
	secretsDir string
	cmd       contracts.CommandRunner
}

// NewService returns a mount manager service.
func NewService(fstabPath, secretsDir string, cmd contracts.CommandRunner) *Service {
	if fstabPath == "" {
		fstabPath = "/etc/fstab"
	}
	if secretsDir == "" {
		secretsDir = "/storage/mount-secrets"
	}
	return &Service{fstabPath: fstabPath, secretsDir: secretsDir, cmd: cmd}
}

// List reads fstab entries and checks mount status.
func (s *Service) List(ctx context.Context) ([]Entry, error) {
	lines, err := s.readLines()
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(lines))
	for _, line := range lines {
		entry, parseErr := parseFstabLine(line)
		if parseErr != nil {
			continue
		}
		entry.Mounted = s.isMounted(ctx, entry.Dir)
		entry = enrichS3ForList(entry)
		out = append(out, entry)
	}
	return out, nil
}

// Add appends a new fstab entry.
func (s *Service) Add(ctx context.Context, entry Entry) error {
	if IsS3Type(entry.Type) {
		prepared, err := s.applyS3Entry(entry, "", true)
		if err != nil {
			return err
		}
		entry = prepared
	}
	entry, err := normalizeEntry(entry)
	if err != nil {
		return err
	}
	lines, err := s.readLines()
	if err != nil {
		return err
	}
	for _, line := range lines {
		existing, parseErr := parseFstabLine(line)
		if parseErr != nil {
			continue
		}
		if existing.Device == entry.Device && existing.Dir == entry.Dir {
			return apperror.New(apperror.CodeConflict, "mount entry already exists")
		}
	}
	lines = append(lines, formatEntry(entry))
	return s.writeLines(lines)
}

// Update replaces an entry matched by old device and dir.
func (s *Service) Update(ctx context.Context, oldDevice, oldDir string, entry Entry) error {
	if err := validateLookupFields(oldDevice, oldDir); err != nil {
		return err
	}
	var oldEntry Entry
	lines, err := s.readLines()
	if err != nil {
		return err
	}
	for _, line := range lines {
		existing, parseErr := parseFstabLine(line)
		if parseErr != nil {
			continue
		}
		if existing.Device == oldDevice && existing.Dir == oldDir {
			oldEntry = existing
			break
		}
	}
	if IsS3Type(entry.Type) {
		keepPasswd := parsePasswdFileOption(oldEntry.Options)
		prepared, prepErr := s.applyS3Entry(entry, keepPasswd, false)
		if prepErr != nil {
			return prepErr
		}
		entry = prepared
	}
	entry, err = normalizeEntry(entry)
	if err != nil {
		return err
	}
	replaced := false
	for i, line := range lines {
		existing, parseErr := parseFstabLine(line)
		if parseErr != nil {
			continue
		}
		if existing.Device == oldDevice && existing.Dir == oldDir {
			lines[i] = formatEntry(entry)
			replaced = true
			break
		}
	}
	if !replaced {
		return apperror.New(apperror.CodeNotFound, "mount entry not found")
	}
	return s.writeLines(lines)
}

// Delete removes an entry and attempts umount.
func (s *Service) Delete(ctx context.Context, device, dir string) error {
	if err := validateLookupFields(device, dir); err != nil {
		return err
	}
	lines, err := s.readLines()
	if err != nil {
		return err
	}
	filtered := make([]string, 0, len(lines))
	removed := false
	for _, line := range lines {
		entry, parseErr := parseFstabLine(line)
		if parseErr != nil {
			filtered = append(filtered, line)
			continue
		}
		if entry.Device == device && entry.Dir == dir {
			removed = true
			if IsS3Type(entry.Type) {
				removePasswdFromOptions(entry.Options)
			}
			continue
		}
		filtered = append(filtered, line)
	}
	if !removed {
		return apperror.New(apperror.CodeNotFound, "mount entry not found")
	}
	_, _ = s.cmd.Run(ctx, "umount", dir)
	return s.writeLines(filtered)
}

// Enable mounts a directory defined in fstab.
func (s *Service) Enable(ctx context.Context, device, dir string) error {
	if err := validateLookupFields(device, dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "create mount point", err)
	}
	res, err := s.cmd.Run(ctx, "mount", dir)
	if err != nil {
		return apperror.Wrap(apperror.CodeMountFailed, "mount failed", err)
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(res.Stdout)
		}
		return apperror.New(apperror.CodeMountFailed, msg)
	}
	if device != "" {
		_ = device
	}
	return nil
}

func (s *Service) readLines() ([]string, error) {
	data, err := os.ReadFile(s.fstabPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, apperror.Wrap(apperror.CodeInternal, "read fstab", err)
	}
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

func (s *Service) writeLines(lines []string) error {
	if err := os.MkdirAll(filepath.Dir(s.fstabPath), 0o755); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "create fstab dir", err)
	}
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(s.fstabPath, []byte(content), 0o644); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "write fstab", err)
	}
	return nil
}

func (s *Service) isMounted(ctx context.Context, dir string) bool {
	res, err := s.cmd.Run(ctx, "mountpoint", "-q", dir)
	return err == nil && res.ExitCode == 0
}

func parseFstabLine(line string) (Entry, error) {
	fields := strings.Fields(line)
	if len(fields) != 6 {
		return Entry{}, fmt.Errorf("invalid fstab columns")
	}
	return Entry{
		Device:  fields[0],
		Dir:     fields[1],
		Type:    fields[2],
		Options: fields[3],
		Dump:    fields[4],
		Fsck:    fields[5],
	}, nil
}

func formatEntry(entry Entry) string {
	return fmt.Sprintf("%s %s %s %s %s %s",
		entry.Device, entry.Dir, entry.Type, entry.Options, entry.Dump, entry.Fsck)
}

func normalizeEntry(entry Entry) (Entry, error) {
	entry.Device = strings.TrimSpace(entry.Device)
	entry.Dir = strings.TrimSpace(entry.Dir)
	entry.Type = strings.TrimSpace(entry.Type)
	entry.Options = strings.TrimSpace(entry.Options)
	entry.Dump = strings.TrimSpace(entry.Dump)
	entry.Fsck = strings.TrimSpace(entry.Fsck)

	if entry.Device == "" || entry.Dir == "" {
		return Entry{}, apperror.New(apperror.CodeInvalidInput, "device and dir required")
	}
	entry.Type = normalizeS3Type(entry.Type)
	if entry.Type == "" {
		return Entry{}, apperror.New(apperror.CodeInvalidInput, "type required")
	}
	if entry.Options == "" {
		entry.Options = "defaults"
	}
	if entry.Dump == "" {
		entry.Dump = "0"
	}
	if entry.Fsck == "" {
		entry.Fsck = "0"
	}
	for _, field := range []string{entry.Device, entry.Dir, entry.Type, entry.Options, entry.Dump, entry.Fsck} {
		if strings.ContainsAny(field, " \t\r\n") {
			return Entry{}, apperror.New(apperror.CodeInvalidInput, "mount fields must not contain whitespace")
		}
	}
	if !filepath.IsAbs(entry.Dir) {
		return Entry{}, apperror.New(apperror.CodeInvalidInput, "mount dir must be absolute")
	}
	if _, err := strconv.Atoi(entry.Dump); err != nil {
		return Entry{}, apperror.New(apperror.CodeInvalidInput, "dump must be numeric")
	}
	if _, err := strconv.Atoi(entry.Fsck); err != nil {
		return Entry{}, apperror.New(apperror.CodeInvalidInput, "fsck must be numeric")
	}
	return entry, nil
}

func validateLookupFields(device, dir string) error {
	if strings.TrimSpace(dir) == "" {
		return apperror.New(apperror.CodeInvalidInput, "dir required")
	}
	for _, field := range []string{device, dir} {
		if strings.ContainsAny(field, " \t\r\n") {
			return apperror.New(apperror.CodeInvalidInput, "mount fields must not contain whitespace")
		}
	}
	if !filepath.IsAbs(dir) {
		return apperror.New(apperror.CodeInvalidInput, "mount dir must be absolute")
	}
	return nil
}
