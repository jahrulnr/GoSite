package bootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAdminName  = "Admin"
	defaultAdminEmail = "admin@demo.com"
	defaultAdminPass  = "123456"

	defaultCronName    = "Lets Encrypt Renewal"
	defaultCronPayload = "certbot renew --post-hook 'nginx -s reload'"
	defaultCronEvery   = "day"
)

// Init prepares the persistent storage layout, copies templates, creates
// symlinks, applies migrations, and seeds default records on first run.
func Init(cfg config.Config) error {
	if err := createStorageLayout(cfg); err != nil {
		return err
	}
	if err := copyTemplatesIfMissing(cfg); err != nil {
		return err
	}
	if err := ensureFstab(cfg); err != nil {
		return err
	}
	if err := createSymlinks(cfg); err != nil {
		return err
	}
	if err := ensureDefaultWWW(cfg); err != nil {
		return err
	}

	db, err := sqlite.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := sqlite.Migrate(db, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	if err := seedAdminIfEmpty(context.Background(), db); err != nil {
		return err
	}
	if err := seedDefaultCronIfEmpty(context.Background(), db); err != nil {
		return err
	}
	if err := seedDemoIfNeeded(context.Background(), cfg, db); err != nil {
		return err
	}
	if err := seedBundledPlugins(context.Background(), cfg, db); err != nil {
		return err
	}

	return nil
}

func createStorageLayout(cfg config.Config) error {
	for _, dir := range cfg.StorageLayout() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	if err := migrateLegacyLogs(cfg); err != nil {
		return err
	}

	nginxDir := filepath.Join(cfg.Storage, "nginx")
	if err := os.MkdirAll(nginxDir, 0o755); err != nil {
		return fmt.Errorf("create nginx directory: %w", err)
	}

	return nil
}

func migrateLegacyLogs(cfg config.Config) error {
	logsDir := cfg.LogsDir()
	legacyDir := filepath.Join(cfg.Storage, "laravel", "logs")
	info, err := os.Stat(legacyDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat legacy log dir: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		return fmt.Errorf("read legacy log dir: %w", err)
	}
	for _, entry := range entries {
		src := filepath.Join(legacyDir, entry.Name())
		dst := filepath.Join(logsDir, entry.Name())
		if _, err := os.Stat(dst); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat log file %s: %w", dst, err)
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("migrate log %s: %w", entry.Name(), err)
		}
	}

	_ = os.Remove(legacyDir)
	_ = os.Remove(filepath.Join(cfg.Storage, "laravel"))
	return nil
}

func copyTemplatesIfMissing(cfg config.Config) error {
	if cfg.TemplatesDir == "" {
		return nil
	}

	copies := []struct {
		srcName string
		dst     string
	}{
		{"webconfig", filepath.Join(cfg.Storage, "webconfig")},
		{"nginx", filepath.Join(cfg.Storage, "nginx")},
	}

	for _, item := range copies {
		src := filepath.Join(cfg.TemplatesDir, item.srcName)
		if _, err := os.Stat(src); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("stat template %s: %w", src, err)
		}

		// Merge rather than skip-if-exists: createStorageLayout always creates
		// the destination, so healing must fill in missing files. Existing
		// persisted edits always win.
		if err := copyTreeMissing(src, item.dst); err != nil {
			return fmt.Errorf("copy %s to %s: %w", src, item.dst, err)
		}
	}

	return nil
}

// copyTreeMissing merges src into dst, copying only entries that do not
// already exist in dst. Existing files, directories, and symlinks are never
// modified or overwritten; an empty/missing dst is populated wholesale.
func copyTreeMissing(src, dst string) error {
	dstInfo, err := os.Lstat(dst)
	if errors.Is(err, os.ErrNotExist) {
		return copyTree(src, dst)
	}
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() || !dstInfo.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := copyTreeMissing(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}

func ensureFstab(cfg config.Config) error {
	path := filepath.Join(cfg.Storage, "fstab")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat fstab: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create fstab: %w", err)
	}
	return f.Close()
}

func createSymlinks(cfg config.Config) error {
	links := []struct {
		target string
		link   string
	}{
		{filepath.Join(cfg.Storage, "nginx"), filepath.Join(cfg.EtcDir, "nginx")},
		{filepath.Join(cfg.Storage, "fstab"), filepath.Join(cfg.EtcDir, "fstab")},
		{filepath.Join(cfg.Storage, "www"), cfg.WebPath},
		{filepath.Join(cfg.Storage, "webconfig", "ssl"), cfg.LetsEncryptDir},
	}

	for _, item := range links {
		if err := ensureSymlink(item.target, item.link); err != nil {
			return err
		}
	}

	return nil
}

func ensureDefaultWWW(cfg config.Config) error {
	indexPath := filepath.Join(cfg.WebPath, "default", "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat default index: %w", err)
	}

	src := filepath.Join(cfg.Storage, "webconfig", "index.html")
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat webconfig index: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return fmt.Errorf("create default www dir: %w", err)
	}

	if err := copyFile(src, indexPath); err != nil {
		return fmt.Errorf("copy default index: %w", err)
	}

	return nil
}

func seedAdminIfEmpty(ctx context.Context, db *sql.DB) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users`).Scan(&count); err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := laravelBcrypt(defaultAdminPass)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	repo := sqlite.NewUserRepository(db)
	_, err = repo.Create(ctx, sqlite.User{
		Name:     defaultAdminName,
		Email:    defaultAdminEmail,
		Password: hash,
	})
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}

	return nil
}

func seedDefaultCronIfEmpty(ctx context.Context, db *sql.DB) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM cronjobs`).Scan(&count); err != nil {
		return fmt.Errorf("count cronjobs: %w", err)
	}
	if count > 0 {
		return nil
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO cronjobs (name, payload, run_every, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, defaultCronName, defaultCronPayload, defaultCronEvery)
	if err != nil {
		return fmt.Errorf("seed default cronjob: %w", err)
	}

	return nil
}

func ensureSymlink(target, link string) error {
	if target == "" || link == "" {
		return nil
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("abs target %s: %w", target, err)
	}
	absLink, err := filepath.Abs(link)
	if err != nil {
		return fmt.Errorf("abs link %s: %w", link, err)
	}
	if absTarget == absLink {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return fmt.Errorf("create parent for symlink %s: %w", link, err)
	}

	if info, err := os.Lstat(link); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			current, err := os.Readlink(link)
			if err != nil {
				return fmt.Errorf("read symlink %s: %w", link, err)
			}
			absTarget, err := filepath.Abs(target)
			if err != nil {
				return fmt.Errorf("abs target %s: %w", target, err)
			}
			absCurrent, err := filepath.Abs(current)
			if err != nil {
				absCurrent = current
			}
			if absCurrent == absTarget || current == target {
				return nil
			}
		} else if info.IsDir() {
			// Migrate real directory contents to target before creating symlink.
			// This preserves files from a previous run or external tool. Conflicts
			// are retained under <target>.migrated-conflicts instead of discarded.
			if err := migrateDirContents(link, target); err != nil {
				return fmt.Errorf("migrate %s to %s: %w", link, target, err)
			}
		}
		_ = os.RemoveAll(link)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("lstat %s: %w", link, err)
	}

	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", link, target, err)
	}

	return nil
}

// migrateDirContents merges srcDir into dstDir without overwriting existing
// files. Conflicting source entries are retained under a sibling
// .migrated-conflicts directory so replacing srcDir cannot silently lose data.
// A top-level conf.d entry never enters the live tree: nginx images ship
// conf.d/default.conf with a `default_server` listener that would collide
// with persisted site configs, so it is quarantined under .migrated-conflicts.
func migrateDirContents(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	conflictRoot := dstDir + ".migrated-conflicts"

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(dstDir, entry.Name())
		dstRoot := dstDir
		if entry.Name() == "conf.d" {
			dst = filepath.Join(conflictRoot, entry.Name())
			dstRoot = conflictRoot
			fmt.Fprintf(os.Stderr, "exclude %s from live tree; kept under %s\n", src, dst)
		}
		if err := migrateEntry(src, dst, dstRoot, conflictRoot); err != nil {
			return err
		}
	}

	return nil
}

func mergeDirContents(srcDir, dstDir, dstRoot, conflictRoot string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(dstDir, entry.Name())
		if err := migrateEntry(src, dst, dstRoot, conflictRoot); err != nil {
			return err
		}
	}

	return nil
}

func migrateEntry(src, dst, dstRoot, conflictRoot string) error {
	sourceInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}

	destinationInfo, err := os.Lstat(dst)
	if errors.Is(err, os.ErrNotExist) {
		return copyMigratedEntry(src, dst, dstRoot, conflictRoot)
	}
	if err != nil {
		return err
	}

	if sourceInfo.IsDir() && destinationInfo.IsDir() {
		return mergeDirContents(src, dst, dstRoot, conflictRoot)
	}

	equal, err := migratedEntriesEqual(src, dst, sourceInfo, destinationInfo)
	if err != nil {
		return err
	}
	if equal {
		return nil
	}

	return preserveMigrationConflict(src, dst, dstRoot, conflictRoot)
}

func migratedEntriesEqual(src, dst string, srcInfo, dstInfo os.FileInfo) (bool, error) {
	srcIsSymlink := srcInfo.Mode()&os.ModeSymlink != 0
	dstIsSymlink := dstInfo.Mode()&os.ModeSymlink != 0
	if srcIsSymlink || dstIsSymlink {
		if !srcIsSymlink || !dstIsSymlink {
			return false, nil
		}
		srcTarget, err := os.Readlink(src)
		if err != nil {
			return false, err
		}
		dstTarget, err := os.Readlink(dst)
		if err != nil {
			return false, err
		}
		return srcTarget == dstTarget, nil
	}

	if !srcInfo.Mode().IsRegular() || !dstInfo.Mode().IsRegular() {
		return false, nil
	}
	if srcInfo.Size() != dstInfo.Size() {
		return false, nil
	}
	srcSum, err := fileChecksum(src)
	if err != nil {
		return false, err
	}
	dstSum, err := fileChecksum(dst)
	if err != nil {
		return false, err
	}
	return bytes.Equal(srcSum, dstSum), nil
}

func fileChecksum(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return nil, err
	}
	return sum.Sum(nil), nil
}

func preserveMigrationConflict(src, dst, dstRoot, conflictRoot string) error {
	relative, err := filepath.Rel(dstRoot, dst)
	if err != nil {
		return err
	}
	base := filepath.Join(conflictRoot, relative) + ".migrated"

	for attempt := 0; ; attempt++ {
		candidate := base
		if attempt > 0 {
			candidate = fmt.Sprintf("%s.%d", base, attempt)
		}
		candidateInfo, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return copyMigratedEntry(src, candidate, dstRoot, conflictRoot)
		}
		if err != nil {
			return err
		}
		sourceInfo, err := os.Lstat(src)
		if err != nil {
			return err
		}
		equal, err := migratedEntriesEqual(src, candidate, sourceInfo, candidateInfo)
		if err != nil {
			return err
		}
		if equal {
			return nil
		}
	}
}

func copyMigratedEntry(src, dst, dstRoot, conflictRoot string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() && !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		// FIFO/socket/device entries cannot be recreated; skipping them must
		// not abort init or the container fails to boot.
		fmt.Fprintf(os.Stderr, "skip unsupported migration entry type %s: %s\n", info.Mode(), src)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}
	if info.IsDir() {
		if err := os.Mkdir(dst, info.Mode().Perm()); err != nil {
			return err
		}
		if err := mergeDirContents(src, dst, dstRoot, conflictRoot); err != nil {
			return err
		}
		if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
			return err
		}
		return os.Chtimes(dst, info.ModTime(), info.ModTime())
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst)
	}

	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyTree(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return nil
}

func laravelBcrypt(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return strings.Replace(string(hash), "$2a$", "$2y$", 1), nil
}
