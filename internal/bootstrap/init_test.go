package bootstrap_test

import (
	"context"
	"os"
	"path/filepath"
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
	assert.Equal(t, 4, migrationCount)
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
