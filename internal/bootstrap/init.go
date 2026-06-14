package bootstrap

import (
	"context"
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
	defaultCronPayload = "certbot renew --post-hook 'supervisorctl restart nginx'"
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

	return nil
}

func createStorageLayout(cfg config.Config) error {
	for _, dir := range cfg.StorageLayout() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	nginxDir := filepath.Join(cfg.Storage, "nginx")
	if err := os.MkdirAll(nginxDir, 0o755); err != nil {
		return fmt.Errorf("create nginx directory: %w", err)
	}

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

		if _, err := os.Stat(item.dst); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat destination %s: %w", item.dst, err)
		}

		if err := copyTree(src, item.dst); err != nil {
			return fmt.Errorf("copy %s to %s: %w", src, item.dst, err)
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
