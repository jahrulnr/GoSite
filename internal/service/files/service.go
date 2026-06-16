package files

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/infra/filesystem"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Entry describes a file or directory in a listing.
type Entry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Mode      string `json:"mode"`
	IsDir     bool   `json:"is_dir"`
	ModTime   string `json:"mod_time"`
	Kind      string `json:"kind"`
	MimeType  string `json:"mime_type"`
	Extension string `json:"extension"`
	Editable  bool   `json:"editable"`
	Viewable  bool   `json:"viewable"`
	Archive   bool   `json:"archive"`
	Symlink   bool   `json:"symlink"`
	Target    string `json:"target,omitempty"`
}

// ListResult describes a directory listing plus runtime capabilities.
type ListResult struct {
	Entries []Entry      `json:"entries"`
	Tools   ArchiveTools `json:"tools"`
}

// ArchiveTools reports archive helpers available on the host.
type ArchiveTools struct {
	Unzip bool `json:"unzip"`
	Tar   bool `json:"tar"`
	Gzip  bool `json:"gzip"`
}

// ContentResult describes file content and metadata.
type ContentResult struct {
	Content  string `json:"content"`
	Entry    Entry  `json:"entry"`
	Encoding string `json:"encoding"`
}

// Service manages file manager operations within allowed roots.
type Service struct {
	paths        *filesystem.Validator
	allowExecute bool
	cmd          contracts.CommandRunner
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
func (s *Service) Browse(ctx context.Context, rawPath string) (ListResult, error) {
	path, err := s.paths.Resolve(rawPath)
	if err != nil {
		return ListResult{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ListResult{}, apperror.Wrap(apperror.CodeFileNotFound, "path not found", err)
		}
		return ListResult{}, apperror.Wrap(apperror.CodeInternal, "stat path", err)
	}
	if !info.IsDir() {
		return ListResult{}, apperror.New(apperror.CodePathIsFile, "path is a file")
	}

	rows, err := os.ReadDir(path)
	if err != nil {
		return ListResult{}, apperror.Wrap(apperror.CodeInternal, "read directory", err)
	}

	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		fi, statErr := row.Info()
		if statErr != nil {
			continue
		}
		out = append(out, describeEntry(filepath.Join(path, row.Name()), fi, row.Type()&os.ModeSymlink != 0))
	}
	sortEntries(out)
	return ListResult{Entries: out, Tools: detectArchiveTools()}, nil
}

// Read returns file content as text.
func (s *Service) Read(ctx context.Context, rawPath string) (ContentResult, error) {
	path, err := s.paths.Resolve(rawPath)
	if err != nil {
		return ContentResult{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ContentResult{}, apperror.Wrap(apperror.CodeFileNotFound, "file not found", err)
		}
		return ContentResult{}, apperror.Wrap(apperror.CodeInternal, "stat file", err)
	}
	if info.IsDir() {
		return ContentResult{}, apperror.New(apperror.CodePathIsFile, "path is a directory")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ContentResult{}, apperror.Wrap(apperror.CodeInternal, "read file", err)
	}
	return ContentResult{Content: string(data), Entry: describeEntry(path, info, false), Encoding: "utf-8"}, nil
}

// ResolveFile returns a validated file path for raw serving.
func (s *Service) ResolveFile(ctx context.Context, rawPath string) (string, Entry, error) {
	path, err := s.paths.Resolve(rawPath)
	if err != nil {
		return "", Entry{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", Entry{}, apperror.Wrap(apperror.CodeFileNotFound, "file not found", err)
		}
		return "", Entry{}, apperror.Wrap(apperror.CodeInternal, "stat file", err)
	}
	if info.IsDir() {
		return "", Entry{}, apperror.New(apperror.CodePathIsFile, "path is a directory")
	}
	return path, describeEntry(path, info, false), nil
}

// Save overwrites a file with text content.
func (s *Service) Save(ctx context.Context, rawPath, content string) error {
	path, err := s.paths.Resolve(rawPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return apperror.Wrap(apperror.CodeFileNotFound, "file not found", err)
		}
		return apperror.Wrap(apperror.CodeInternal, "stat file", err)
	}
	if info.IsDir() {
		return apperror.New(apperror.CodePathIsFile, "path is a directory")
	}
	if err := os.WriteFile(path, []byte(content), info.Mode()); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "save file", err)
	}
	return nil
}

// SaveInput holds one batch save item.
type SaveInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// BatchSave writes multiple file contents.
func (s *Service) BatchSave(ctx context.Context, items []SaveInput) error {
	if len(items) == 0 {
		return apperror.New(apperror.CodeInvalidInput, "files required")
	}
	for _, item := range items {
		if err := s.Save(ctx, item.Path, item.Content); err != nil {
			return err
		}
	}
	return nil
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
	case "move", "rename":
		return s.movePath(ctx, path, in.ToPath)
	case "extract", "uncompress":
		return s.extractArchive(ctx, path, in.ToPath)
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
		if dst == src || strings.HasPrefix(dst, src+string(filepath.Separator)) {
			return apperror.New(apperror.CodeInvalidInput, "cannot copy directory into itself")
		}
		return copyDir(src, dst, info.Mode())
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

func copyDir(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(dst, mode); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "create destination dir", err)
	}
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return apperror.Wrap(apperror.CodeInternal, "walk source", err)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return apperror.Wrap(apperror.CodeInternal, "resolve source relative path", err)
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return apperror.Wrap(apperror.CodeInternal, "stat source", err)
		}
		if entry.IsDir() {
			if err := os.MkdirAll(target, info.Mode()); err != nil {
				return apperror.Wrap(apperror.CodeInternal, "create destination dir", err)
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return apperror.Wrap(apperror.CodeInternal, "read symlink", err)
			}
			if err := os.Symlink(link, target); err != nil {
				return apperror.Wrap(apperror.CodeInternal, "create symlink", err)
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return apperror.Wrap(apperror.CodeInternal, "read source", err)
		}
		if err := os.WriteFile(target, data, info.Mode()); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write destination", err)
		}
		return nil
	})
}

func (s *Service) movePath(ctx context.Context, src, dstRaw string) error {
	dst, err := s.paths.Resolve(dstRaw)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "create destination dir", err)
	}
	if err := os.Rename(src, dst); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "move path", err)
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

func (s *Service) extractArchive(ctx context.Context, path, destRaw string) error {
	info, err := os.Stat(path)
	if err != nil {
		return apperror.Wrap(apperror.CodeFileNotFound, "archive not found", err)
	}
	if info.IsDir() {
		return apperror.New(apperror.CodeInvalidInput, "archive must be a file")
	}
	dest := filepath.Dir(path)
	if strings.TrimSpace(destRaw) != "" {
		resolved, resolveErr := s.paths.Resolve(destRaw)
		if resolveErr != nil {
			return resolveErr
		}
		dest = resolved
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "create extract dir", err)
	}
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return runArchiveTool(ctx, s.cmd, "unzip", "-q", path, "-d", dest)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return runArchiveTool(ctx, s.cmd, "tar", "-xzf", path, "-C", dest)
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"):
		return runArchiveTool(ctx, s.cmd, "tar", "-xjf", path, "-C", dest)
	case strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		return runArchiveTool(ctx, s.cmd, "tar", "-xJf", path, "-C", dest)
	case strings.HasSuffix(lower, ".tar"):
		return runArchiveTool(ctx, s.cmd, "tar", "-xf", path, "-C", dest)
	case strings.HasSuffix(lower, ".gz"):
		return runArchiveTool(ctx, s.cmd, "gzip", "-dk", path)
	default:
		return apperror.New(apperror.CodeInvalidInput, "unsupported archive type")
	}
}

func runArchiveTool(ctx context.Context, cmd contracts.CommandRunner, name string, args ...string) error {
	if _, err := exec.LookPath(name); err != nil {
		return apperror.Wrap(apperror.CodeInvalidInput, name+" not available", err)
	}
	res, err := cmd.Run(ctx, name, args...)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "run "+name, err)
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(res.Stdout)
		}
		if msg == "" {
			msg = name + " failed"
		}
		return apperror.New(apperror.CodeInvalidInput, msg)
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

// BatchDelete removes multiple files or directories recursively.
func (s *Service) BatchDelete(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return apperror.New(apperror.CodeInvalidInput, "paths required")
	}
	for _, rawPath := range paths {
		if err := s.Delete(ctx, rawPath); err != nil {
			return err
		}
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

func describeEntry(path string, info os.FileInfo, symlink bool) Entry {
	ext := strings.ToLower(filepath.Ext(path))
	kind := detectKind(info, ext)
	target := ""
	if symlink {
		if link, err := os.Readlink(path); err == nil {
			target = link
		}
	}
	return Entry{
		Name:      filepath.Base(path),
		Path:      path,
		Size:      info.Size(),
		Mode:      info.Mode().String(),
		IsDir:     info.IsDir(),
		ModTime:   info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		Kind:      kind,
		MimeType:  mimeForKind(kind, ext),
		Extension: strings.TrimPrefix(ext, "."),
		Editable:  !info.IsDir() && isEditable(ext),
		Viewable:  !info.IsDir() && isViewable(ext),
		Archive:   !info.IsDir() && isArchive(path),
		Symlink:   symlink,
		Target:    target,
	}
}

func detectKind(info os.FileInfo, ext string) string {
	if info.IsDir() {
		return "directory"
	}
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".ico":
		return "image"
	case ".json", ".jsonc":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml", ".ini", ".conf", ".cfg", ".env", ".service", ".socket", ".timer":
		return "config"
	case ".html", ".htm", ".css", ".js", ".ts", ".tsx", ".jsx", ".go", ".php", ".py", ".sh", ".sql", ".md", ".txt", ".log", ".xml":
		return "text"
	default:
		if isArchiveExt(ext) {
			return "archive"
		}
		return "binary"
	}
}

func mimeForKind(kind, ext string) string {
	switch kind {
	case "image":
		if ext == ".svg" {
			return "image/svg+xml"
		}
		return "image/" + strings.TrimPrefix(ext, ".")
	case "json":
		return "application/json"
	case "yaml":
		return "application/yaml"
	case "archive":
		return "application/octet-stream"
	case "directory":
		return "inode/directory"
	default:
		return "text/plain"
	}
}

func isEditable(ext string) bool {
	switch ext {
	case ".json", ".jsonc", ".yaml", ".yml", ".toml", ".ini", ".conf", ".cfg", ".env", ".service", ".socket", ".timer", ".html", ".htm", ".css", ".js", ".ts", ".tsx", ".jsx", ".go", ".php", ".py", ".sh", ".sql", ".md", ".txt", ".log", ".xml":
		return true
	default:
		return false
	}
}

func isViewable(ext string) bool {
	return isEditable(ext) || isImageExt(ext)
}

func isImageExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".ico":
		return true
	default:
		return false
	}
}

func isArchive(path string) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tar.xz") {
		return true
	}
	return isArchiveExt(filepath.Ext(lower))
}

func isArchiveExt(ext string) bool {
	switch ext {
	case ".zip", ".tar", ".tgz", ".tbz2", ".txz", ".gz", ".bz2", ".xz":
		return true
	default:
		return false
	}
}

func detectArchiveTools() ArchiveTools {
	_, unzipErr := exec.LookPath("unzip")
	_, tarErr := exec.LookPath("tar")
	_, gzipErr := exec.LookPath("gzip")
	return ArchiveTools{Unzip: unzipErr == nil, Tar: tarErr == nil, Gzip: gzipErr == nil}
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		left := strings.ToLower(entries[i].Name)
		right := strings.ToLower(entries[j].Name)
		if left == right {
			return entries[i].Name < entries[j].Name
		}
		return left < right
	})
}
