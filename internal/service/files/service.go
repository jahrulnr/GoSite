package files

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/infra/filesystem"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Entry describes a file or directory in a listing.
type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
}

// Service manages file manager operations within allowed roots.
type Service struct {
	paths         *filesystem.Validator
	allowExecute  bool
	cmd           contracts.CommandRunner
}

// NewService returns a file manager service.
func NewService(roots []string, allowExecute bool, cmd contracts.CommandRunner) *Service {
	return &Service{
		paths:        filesystem.NewValidator(roots...),
		allowExecute: allowExecute,
		cmd:          cmd,
	}
}

// Browse lists entries in a directory.
func (s *Service) Browse(ctx context.Context, rawPath string) ([]Entry, error) {
	path, err := s.paths.Resolve(rawPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperror.Wrap(apperror.CodeFileNotFound, "path not found", err)
		}
		return nil, apperror.Wrap(apperror.CodeInternal, "stat path", err)
	}
	if !info.IsDir() {
		return nil, apperror.New(apperror.CodePathIsFile, "path is a file")
	}

	rows, err := os.ReadDir(path)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "read directory", err)
	}

	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		fi, statErr := row.Info()
		if statErr != nil {
			continue
		}
		entryPath := filepath.Join(path, row.Name())
		out = append(out, Entry{
			Name:    row.Name(),
			Path:    entryPath,
			Size:    fi.Size(),
			Mode:    fi.Mode().String(),
			IsDir:   fi.IsDir(),
			ModTime: fi.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return out, nil
}

// Read returns file content as text.
func (s *Service) Read(ctx context.Context, rawPath string) (string, error) {
	path, err := s.paths.Resolve(rawPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", apperror.Wrap(apperror.CodeFileNotFound, "file not found", err)
		}
		return "", apperror.Wrap(apperror.CodeInternal, "stat file", err)
	}
	if info.IsDir() {
		return "", apperror.New(apperror.CodePathIsFile, "path is a directory")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeInternal, "read file", err)
	}
	return string(data), nil
}

// CreateInput holds create parameters.
type CreateInput struct {
	Type    string
	Name    string
	Path    string
	Content string
}

// Create makes a new file or directory.
func (s *Service) Create(ctx context.Context, in CreateInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return apperror.New(apperror.CodeInvalidInput, "name required")
	}
	if strings.Contains(in.Name, "/") || strings.Contains(in.Name, "..") {
		return apperror.New(apperror.CodePathTraversal, "path traversal rejected")
	}

	dir, err := s.paths.Resolve(in.Path)
	if err != nil {
		return err
	}
	target := filepath.Join(dir, in.Name)
	if _, validateErr := s.paths.Resolve(target); validateErr != nil {
		return validateErr
	}

	switch strings.ToLower(strings.TrimSpace(in.Type)) {
	case "directory", "dir":
		if err := os.MkdirAll(target, 0o755); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "create directory", err)
		}
	case "file":
		if err := os.WriteFile(target, []byte(in.Content), 0o644); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "create file", err)
		}
	default:
		return apperror.New(apperror.CodeInvalidInput, "invalid create type")
	}
	return nil
}

// Upload writes uploaded content to a destination path.
func (s *Service) Upload(ctx context.Context, rawPath, filename string, r io.Reader) error {
	if strings.TrimSpace(filename) == "" {
		return apperror.New(apperror.CodeInvalidInput, "filename required")
	}
	if strings.Contains(filename, "/") || strings.Contains(filename, "..") {
		return apperror.New(apperror.CodePathTraversal, "path traversal rejected")
	}
	dir, err := s.paths.Resolve(rawPath)
	if err != nil {
		return err
	}
	target := filepath.Join(dir, filename)
	if _, validateErr := s.paths.Resolve(target); validateErr != nil {
		return validateErr
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "read upload", err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "write upload", err)
	}
	return nil
}

// ActionInput holds file action parameters.
type ActionInput struct {
	Action string
	Path   string
	Mode   string
	ToPath string
}

// Action performs chmod, copy, or execute on a path.
func (s *Service) Action(ctx context.Context, in ActionInput) error {
	path, err := s.paths.Resolve(in.Path)
	if err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(in.Action)) {
	case "chmod":
		return s.chmod(ctx, path, in.Mode)
	case "copy":
		return s.copyPath(ctx, path, in.ToPath)
	case "execute":
		return s.execute(ctx, path)
	default:
		return apperror.New(apperror.CodeInvalidInput, "invalid action")
	}
}

func (s *Service) chmod(ctx context.Context, path, modeStr string) error {
	mode, err := parseFileMode(modeStr)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, mode); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "chmod", err)
	}
	return nil
}

func (s *Service) copyPath(ctx context.Context, src, dstRaw string) error {
	dst, err := s.paths.Resolve(dstRaw)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return apperror.Wrap(apperror.CodeFileNotFound, "source not found", err)
	}
	if info.IsDir() {
		return apperror.New(apperror.CodeInvalidInput, "copy source must be a file")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "create destination dir", err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "read source", err)
	}
	if err := os.WriteFile(dst, data, info.Mode()); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "write destination", err)
	}
	return nil
}

func (s *Service) execute(ctx context.Context, path string) error {
	if !s.allowExecute {
		return apperror.New(apperror.CodeFileExecuteDisabled, "file execute disabled")
	}
	info, err := os.Stat(path)
	if err != nil {
		return apperror.Wrap(apperror.CodeFileNotFound, "file not found", err)
	}
	if info.IsDir() {
		return apperror.New(apperror.CodeInvalidInput, "cannot execute directory")
	}
	res, runErr := s.cmd.Run(ctx, path)
	if runErr != nil || res.ExitCode != 0 {
		return apperror.From(runErr)
	}
	return nil
}

// Delete removes a file or directory recursively.
func (s *Service) Delete(ctx context.Context, rawPath string) error {
	path, err := s.paths.Resolve(rawPath)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "delete path", err)
	}
	return nil
}

func parseFileMode(modeStr string) (os.FileMode, error) {
	modeStr = strings.TrimSpace(modeStr)
	if modeStr == "" {
		return 0, apperror.New(apperror.CodeInvalidInput, "mode required")
	}
	v, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return 0, apperror.Wrap(apperror.CodeInvalidInput, "invalid mode", err)
	}
	if v < 0o600 || v > 0o777 {
		return 0, apperror.New(apperror.CodeInvalidInput, "mode out of range")
	}
	return os.FileMode(v), nil
}
