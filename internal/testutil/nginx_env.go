package testutil

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jahrulnr/gosite/internal/infra/nginx"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/ssl"
	"github.com/jahrulnr/gosite/internal/service/website"
	"github.com/stretchr/testify/require"
)

// TestStack bundles SA-4 services for integration tests.
type TestStack struct {
	Root        string
	Storage     string
	WebRoot     string
	DB          *sql.DB
	WebsiteRepo *sqlite.WebsiteRepository
	JobRepo     *sqlite.JobRepository
	Cmd         *MockCommander
	Nginx       *nginx.Service
	Runner      *nginx.Runner
	WebsiteSvc  *website.Service
	SSLSvc      *ssl.Service
}

// SetupTestStack creates temp storage, migrates DB, and wires nginx/website/ssl services.
func SetupTestStack(t *testing.T) *TestStack {
	t.Helper()

	root := t.TempDir()
	storage := filepath.Join(root, "storage")
	webRoot := filepath.Join(root, "www")
	webconfig := filepath.Join(storage, "webconfig")

	require.NoError(t, os.MkdirAll(filepath.Join(webRoot, "default"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(webRoot, "default", "index.html"), []byte("<html>ok</html>"), 0o644))

	configRoot := configDir()
	require.NoError(t, copyDir(filepath.Join(configRoot, "webconfig"), webconfig))
	require.NoError(t, os.MkdirAll(filepath.Join(webconfig, "site.d"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(webconfig, "active.d"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(webconfig, "backups"), 0o755))

	sslDefault := filepath.Join(webconfig, "ssl/live/default")
	require.NoError(t, os.MkdirAll(sslDefault, 0o755))
	if _, err := os.Stat(filepath.Join(sslDefault, "cert.pem")); os.IsNotExist(err) {
		require.NoError(t, os.WriteFile(filepath.Join(sslDefault, "cert.pem"), []byte(SamplePEMCert), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(sslDefault, "key.pem"), []byte(samplePEMKey), 0o600))
	}

	nginxEtc := filepath.Join(root, "etc/nginx")
	require.NoError(t, os.MkdirAll(filepath.Join(nginxEtc, "http.d"), 0o755))
	require.NoError(t, copyDir(filepath.Join(configRoot, "nginx"), nginxEtc))

	dbPath := filepath.Join(storage, "db.sqlite")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join(configDir(), "..", "migrations"))))

	cmd := NewMockCommander()
	paths := nginx.Paths{
		Storage:       storage,
		SiteD:         filepath.Join(webconfig, "site.d"),
		ActiveD:       filepath.Join(webconfig, "active.d"),
		Backups:       filepath.Join(webconfig, "backups"),
		StaticTpl:     filepath.Join(webconfig, "site.conf"),
		ProxyTpl:      filepath.Join(webconfig, "site-proxy.conf"),
		NginxConf:     filepath.Join(webconfig, "nginx.conf"),
		GlobalConf:    filepath.Join(nginxEtc, "nginx.conf"),
		DefaultConf:   filepath.Join(nginxEtc, "http.d/default.conf"),
		SSLDefaultDir: sslDefault,
	}
	runner := nginx.NewRunner(cmd, nginx.RunnerConfig{
		SiteDDir:   paths.SiteD,
		BackupsDir: paths.Backups,
		NginxConf:  paths.NginxConf,
	})
	ngx := nginx.NewService(runner, cmd, paths)
	websiteRepo := sqlite.NewWebsiteRepository(db)
	jobRepo := sqlite.NewJobRepository(db)

	return &TestStack{
		Root:        root,
		Storage:     storage,
		WebRoot:     webRoot,
		DB:          db,
		WebsiteRepo: websiteRepo,
		JobRepo:     jobRepo,
		Cmd:         cmd,
		Nginx:       ngx,
		Runner:      runner,
		WebsiteSvc:  website.NewService(websiteRepo, ngx, webRoot),
		SSLSvc:      ssl.NewService(websiteRepo, jobRepo, ngx),
	}
}

func configDir() string {
	_, file, _, _ := runtime.Caller(1)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "config"))
}

// ConfigPath returns an absolute path under the module config directory.
func ConfigPath(rel string) string {
	return filepath.Join(configDir(), rel)
}

const samplePEMKey = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIExamplePrivateKeyMaterialForTestsOnly
-----END PRIVATE KEY-----
`

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
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
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
