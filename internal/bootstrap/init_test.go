package bootstrap_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/jahrulnr/gosite/internal/bootstrap"
	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	storage := filepath.Join(root, "storage")
	etc := filepath.Join(root, "etc")
	www := filepath.Join(root, "www")
	letsencrypt := filepath.Join(root, "letsencrypt")
	templates := filepath.Join(root, "templates")

	require.NoError(t, os.MkdirAll(templates, 0o755))
	require.NoError(t, copyTestTemplates(t, templates))

	migrations := migrationsDir(t)

	return config.Config{
		Storage:        storage,
		WebPath:        www,
		Database:       filepath.Join(storage, "db.sqlite"),
		TemplatesDir:   templates,
		MigrationsDir:  migrations,
		EtcDir:         etc,
		LetsEncryptDir: letsencrypt,
	}
}

func copyTestTemplates(t *testing.T, dst string) error {
	t.Helper()
	src := filepath.Clean(filepath.Join("..", "..", "config"))
	return copyTree(src, dst)
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
		if err := copyTree(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", "migrations"))
}

func TestBootstrap_CreatesStorageLayout(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	require.NoError(t, bootstrap.Init(cfg))

	for _, dir := range cfg.StorageLayout() {
		info, err := os.Stat(dir)
		require.NoError(t, err, "missing %s", dir)
		assert.True(t, info.IsDir())
	}

	nginxDir := filepath.Join(cfg.Storage, "nginx")
	info, err := os.Stat(nginxDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	fstabPath := filepath.Join(cfg.Storage, "fstab")
	_, err = os.Stat(fstabPath)
	require.NoError(t, err)
}

func TestBootstrap_IdempotentSecondRun(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	require.NoError(t, bootstrap.Init(cfg))
	require.NoError(t, bootstrap.Init(cfg))

	db, err := sqlite.Open(cfg.Database)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var userCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(1) FROM users`).Scan(&userCount))
	assert.Equal(t, 1, userCount)

	var cronCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(1) FROM cronjobs`).Scan(&cronCount))
	assert.Equal(t, 1, cronCount)

	var migrationCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&migrationCount))
	assert.Equal(t, 10, migrationCount)
}

func TestBootstrap_Symlinks(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	require.NoError(t, bootstrap.Init(cfg))

	assertSymlink(t, filepath.Join(cfg.EtcDir, "nginx"), filepath.Join(cfg.Storage, "nginx"))
	assertSymlink(t, filepath.Join(cfg.EtcDir, "fstab"), filepath.Join(cfg.Storage, "fstab"))
	assertSymlink(t, cfg.WebPath, filepath.Join(cfg.Storage, "www"))
	assertSymlink(t, cfg.LetsEncryptDir, filepath.Join(cfg.Storage, "webconfig", "ssl"))
}

func TestBootstrap_SeedsAdminWhenEmpty(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	require.NoError(t, bootstrap.Init(cfg))

	db, err := sqlite.Open(cfg.Database)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := sqlite.NewUserRepository(db)
	user, err := repo.FindByEmail(context.Background(), "admin@demo.com")
	require.NoError(t, err)
	assert.Equal(t, "Admin", user.Name)
	assert.True(t, testutil.VerifyLaravelBcrypt("123456", user.Password))

	var cronName, cronPayload, cronEvery string
	require.NoError(t, db.QueryRow(`
		SELECT name, payload, run_every FROM cronjobs LIMIT 1
	`).Scan(&cronName, &cronPayload, &cronEvery))
	assert.Equal(t, "Lets Encrypt Renewal", cronName)
	assert.Equal(t, "certbot renew --post-hook 'nginx -s reload'", cronPayload)
	assert.Equal(t, "day", cronEvery)
}

func assertSymlink(t *testing.T, link, expectedTarget string) {
	t.Helper()

	info, err := os.Lstat(link)
	require.NoError(t, err, "missing symlink %s", link)
	require.True(t, info.Mode()&os.ModeSymlink != 0, "%s is not a symlink", link)

	target, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, expectedTarget, target)
}

// TestBootstrap_NginxConfigSurvivesRestart reproduces the bug where
// /etc/nginx becomes a symlink to /storage/nginx on container restart,
// but /storage/nginx contains stale config from the first init instead
// of the config that was moved from /var/setup/nginx by start.sh.
func TestBootstrap_NginxConfigSurvivesRestart(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	storage := filepath.Join(root, "storage")
	etc := filepath.Join(root, "etc")
	www := filepath.Join(root, "www")
	letsencrypt := filepath.Join(root, "letsencrypt")
	templates := filepath.Join(root, "templates")

	require.NoError(t, os.MkdirAll(templates, 0o755))
	require.NoError(t, copyTestTemplates(t, templates))
	// Healing copies missing template files into storage on every init, so
	// the persisted nginx.conf already matches what start.sh stages from the
	// same /var/setup source; a differing storage copy would now win as a
	// conflict instead of being overwritten by migration.
	require.NoError(t, os.WriteFile(filepath.Join(templates, "nginx", "nginx.conf"), []byte("server { listen 80; }"), 0o644))

	migrations := migrationsDir(t)

	cfg := config.Config{
		Storage:        storage,
		WebPath:        www,
		Database:       filepath.Join(storage, "db.sqlite"),
		TemplatesDir:   templates,
		MigrationsDir:  migrations,
		EtcDir:         etc,
		LetsEncryptDir: letsencrypt,
	}

	// === FIRST BOOT ===
	require.NoError(t, bootstrap.Init(cfg))

	// Simulate start.sh: /var/setup/nginx is staged, then moved to /etc/nginx
	// (This is what happens in the current buggy start.sh)
	stagedNginx := filepath.Join(root, "var", "setup", "nginx")
	require.NoError(t, os.MkdirAll(stagedNginx, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stagedNginx, "nginx.conf"), []byte("server { listen 80; }"), 0o644))

	// start.sh does: rm -rf /etc/nginx && mv /var/setup/nginx /etc/nginx
	etcNginx := filepath.Join(etc, "nginx")
	require.NoError(t, os.RemoveAll(etcNginx))
	require.NoError(t, os.Rename(stagedNginx, etcNginx))

	// Verify /etc/nginx is now a real directory with the staged config
	info, err := os.Lstat(etcNginx)
	require.NoError(t, err)
	require.False(t, info.Mode()&os.ModeSymlink != 0, "/etc/nginx should be real dir after first boot")

	data, err := os.ReadFile(filepath.Join(etcNginx, "nginx.conf"))
	require.NoError(t, err)
	assert.Equal(t, "server { listen 80; }", string(data))

	// === RESTART (second boot) ===
	// bootstrap.Init runs again. It sees /etc/nginx is a real directory.
	// With the fix: it migrates contents to /storage/nginx first, then
	// creates the symlink. This preserves the config across restarts.
	require.NoError(t, bootstrap.Init(cfg))

	// After fix: /etc/nginx should be a symlink to /storage/nginx, and
	// the config should be preserved (migrated from the real directory).
	info, err = os.Lstat(etcNginx)
	require.NoError(t, err)
	require.True(t, info.Mode()&os.ModeSymlink != 0, "/etc/nginx should be a symlink after fix")

	target, err := os.Readlink(etcNginx)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(storage, "nginx"), target)

	// Verify the config was migrated to /storage/nginx
	data2, err := os.ReadFile(filepath.Join(etcNginx, "nginx.conf"))
	require.NoError(t, err)
	assert.Equal(t, "server { listen 80; }", string(data2), "/etc/nginx/nginx.conf should preserve config across restarts")
}

func TestBootstrap_NginxMigrationPreservesConflictsAndSymlinks(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	require.NoError(t, bootstrap.Init(cfg))

	storageNginx := filepath.Join(cfg.Storage, "nginx")
	existingConfig := filepath.Join(storageNginx, "nginx.conf")
	require.NoError(t, os.WriteFile(existingConfig, []byte("persistent config"), 0o644))

	etcNginx := filepath.Join(cfg.EtcDir, "nginx")
	require.NoError(t, os.Remove(etcNginx))
	require.NoError(t, os.MkdirAll(etcNginx, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(etcNginx, "nginx.conf"), []byte("legacy config"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(etcNginx, "mime.types"), []byte("mime types"), 0o644))
	require.NoError(t, os.Symlink("mime.types", filepath.Join(etcNginx, "mime-link")))
	require.NoError(t, os.Symlink("missing.conf", filepath.Join(etcNginx, "dangling-link")))

	require.NoError(t, bootstrap.Init(cfg))

	assertSymlink(t, etcNginx, storageNginx)

	persisted, err := os.ReadFile(existingConfig)
	require.NoError(t, err)
	assert.Equal(t, "persistent config", string(persisted))

	conflictCopy, err := os.ReadFile(filepath.Join(cfg.Storage, "nginx.migrated-conflicts", "nginx.conf.migrated"))
	require.NoError(t, err)
	assert.Equal(t, "legacy config", string(conflictCopy))

	mimeTypes, err := os.ReadFile(filepath.Join(storageNginx, "mime.types"))
	require.NoError(t, err)
	assert.Equal(t, "mime types", string(mimeTypes))

	for name, target := range map[string]string{"mime-link": "mime.types", "dangling-link": "missing.conf"} {
		linkPath := filepath.Join(storageNginx, name)
		info, err := os.Lstat(linkPath)
		require.NoError(t, err)
		assert.True(t, info.Mode()&os.ModeSymlink != 0, "%s should remain a symlink", name)
		gotTarget, err := os.Readlink(linkPath)
		require.NoError(t, err)
		assert.Equal(t, target, gotTarget)
	}
}

// TestBootstrap_HealsMissingTemplateFiles verifies that init fills an existing
// but incomplete storage tree from TEMPLATES_DIR, without overwriting files
// that are already persisted.
func TestBootstrap_HealsMissingTemplateFiles(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)

	storageNginx := filepath.Join(cfg.Storage, "nginx")
	require.NoError(t, os.MkdirAll(storageNginx, 0o755))
	persisted := filepath.Join(storageNginx, "nginx.conf")
	require.NoError(t, os.WriteFile(persisted, []byte("user edits win"), 0o644))

	require.NoError(t, bootstrap.Init(cfg))

	data, err := os.ReadFile(persisted)
	require.NoError(t, err)
	assert.Equal(t, "user edits win", string(data), "persisted file must not be overwritten")

	for _, healed := range []string{
		filepath.Join(storageNginx, "http.d", "default.conf"),
		filepath.Join(storageNginx, "custom.d", "gzip.conf"),
	} {
		info, err := os.Stat(healed)
		require.NoError(t, err, "missing healed file %s", healed)
		assert.False(t, info.IsDir())
	}
}

// TestBootstrap_MigrationSkipsNonRegularEntries verifies that FIFO/socket/
// device entries in a real directory being migrated are skipped instead of
// failing init (which would abort container boot under set -e).
func TestBootstrap_MigrationSkipsNonRegularEntries(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	require.NoError(t, bootstrap.Init(cfg))

	etcNginx := filepath.Join(cfg.EtcDir, "nginx")
	require.NoError(t, os.Remove(etcNginx))
	require.NoError(t, os.MkdirAll(etcNginx, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(etcNginx, "keep.conf"), []byte("keep me"), 0o644))

	fifoPath := filepath.Join(etcNginx, "legacy.fifo")
	if err := syscall.Mkfifo(fifoPath, 0o644); err != nil {
		t.Skipf("mkfifo not supported in this environment: %v", err)
	}

	require.NoError(t, bootstrap.Init(cfg))

	storageNginx := filepath.Join(cfg.Storage, "nginx")
	_, err := os.Lstat(filepath.Join(storageNginx, "legacy.fifo"))
	assert.True(t, errors.Is(err, os.ErrNotExist), "fifo must not be migrated")

	data, err := os.ReadFile(filepath.Join(storageNginx, "keep.conf"))
	require.NoError(t, err)
	assert.Equal(t, "keep me", string(data))
	assertSymlink(t, etcNginx, storageNginx)
}

// TestBootstrap_MigrationExcludesTopLevelConfD verifies that an image-shipped
// conf.d directory never enters the live nginx tree (its default_server would
// collide with persisted site configs); it is kept under migrated-conflicts.
func TestBootstrap_MigrationExcludesTopLevelConfD(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	require.NoError(t, bootstrap.Init(cfg))

	etcNginx := filepath.Join(cfg.EtcDir, "nginx")
	require.NoError(t, os.Remove(etcNginx))
	require.NoError(t, os.MkdirAll(filepath.Join(etcNginx, "conf.d"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(etcNginx, "conf.d", "default.conf"), []byte("listen 80 default_server;"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(etcNginx, "nginx.conf"), []byte("migrated nginx conf"), 0o644))

	require.NoError(t, bootstrap.Init(cfg))

	storageNginx := filepath.Join(cfg.Storage, "nginx")
	assertSymlink(t, etcNginx, storageNginx)

	_, err := os.Stat(filepath.Join(storageNginx, "nginx.conf"))
	require.NoError(t, err, "nginx.conf must exist in the live tree")

	_, err = os.Lstat(filepath.Join(storageNginx, "conf.d"))
	assert.True(t, errors.Is(err, os.ErrNotExist), "conf.d must not enter the live tree")

	conflictRoot := filepath.Join(cfg.Storage, "nginx.migrated-conflicts")
	quarantined, err := os.ReadFile(filepath.Join(conflictRoot, "conf.d", "default.conf"))
	require.NoError(t, err)
	assert.Equal(t, "listen 80 default_server;", string(quarantined))

	divergent, err := os.ReadFile(filepath.Join(conflictRoot, "nginx.conf.migrated"))
	require.NoError(t, err)
	assert.Equal(t, "migrated nginx conf", string(divergent))
}

// TestBootstrap_MigrationComparesFileContents verifies migrated files are
// compared by content: identical files produce no conflict copy, differing
// files are retained under migrated-conflicts and the persisted file wins.
func TestBootstrap_MigrationComparesFileContents(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	require.NoError(t, bootstrap.Init(cfg))

	storageNginx := filepath.Join(cfg.Storage, "nginx")
	require.NoError(t, os.WriteFile(filepath.Join(storageNginx, "same.conf"), []byte("identical"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(storageNginx, "diff.conf"), []byte("storage version"), 0o644))

	etcNginx := filepath.Join(cfg.EtcDir, "nginx")
	require.NoError(t, os.Remove(etcNginx))
	require.NoError(t, os.MkdirAll(etcNginx, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(etcNginx, "same.conf"), []byte("identical"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(etcNginx, "diff.conf"), []byte("etc version"), 0o644))

	require.NoError(t, bootstrap.Init(cfg))

	conflictRoot := filepath.Join(cfg.Storage, "nginx.migrated-conflicts")
	_, err := os.Lstat(filepath.Join(conflictRoot, "same.conf.migrated"))
	assert.True(t, errors.Is(err, os.ErrNotExist), "identical file must not produce a conflict copy")

	conflict, err := os.ReadFile(filepath.Join(conflictRoot, "diff.conf.migrated"))
	require.NoError(t, err)
	assert.Equal(t, "etc version", string(conflict))

	live, err := os.ReadFile(filepath.Join(storageNginx, "diff.conf"))
	require.NoError(t, err)
	assert.Equal(t, "storage version", string(live), "persisted file must win")
}
