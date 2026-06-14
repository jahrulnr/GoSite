package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/infra/nginx"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
)

type demoSiteSpec struct {
	Name     string
	Domain   string
	Slug     string
	Type     string
	Upstream string
	Active   bool
	SSL      bool
	SSLTTL   time.Duration
}

var demoSites = []demoSiteSpec{
	{Name: "Marketing Site", Domain: "marketing.demo.local", Slug: "marketing", Type: sqlite.WebsiteTypeStatic, Active: true, SSL: true, SSLTTL: 5 * 24 * time.Hour},
	{Name: "Company Blog", Domain: "blog.demo.local", Slug: "blog", Type: sqlite.WebsiteTypeStatic, Active: true},
	{Name: "API Gateway", Domain: "api.demo.local", Slug: "api", Type: sqlite.WebsiteTypeProxy, Upstream: "http://127.0.0.1:3000", Active: true},
	{Name: "Docs Portal", Domain: "docs.demo.local", Slug: "docs", Type: sqlite.WebsiteTypeStatic, Active: true},
	{Name: "Staging App", Domain: "staging.demo.local", Slug: "staging", Type: sqlite.WebsiteTypeStatic, Active: false},
}

func seedDemoIfNeeded(ctx context.Context, cfg config.Config, db *sql.DB) error {
	if !shouldSeedDemo(cfg) {
		return nil
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM websites`).Scan(&count); err != nil {
		return fmt.Errorf("count websites for demo seed: %w", err)
	}
	if count > 0 {
		if err := seedDemoLogsIfMissing(cfg); err != nil {
			return err
		}
		return seedDemoAuditIfEmpty(ctx, db)
	}

	logDir := cfg.LogsDir()
	webconfig := filepath.Join(cfg.Storage, "webconfig")
	siteD := filepath.Join(webconfig, "site.d")
	activeD := filepath.Join(webconfig, "active.d")
	if err := os.MkdirAll(siteD, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(activeD, 0o755); err != nil {
		return err
	}

	repo := sqlite.NewWebsiteRepository(db)
	auditRepo := sqlite.NewAuditRepository(db)
	metricsRepo := sqlite.NewTrafficMetricsRepository(db)

	for _, spec := range demoSites {
		sitePath := filepath.Join(cfg.WebPath, spec.Slug)
		if err := os.MkdirAll(sitePath, 0o755); err != nil {
			return fmt.Errorf("create web path %s: %w", sitePath, err)
		}
		if err := os.WriteFile(filepath.Join(sitePath, "index.html"), []byte(fmt.Sprintf("<html><body>%s</body></html>", spec.Name)), 0o644); err != nil {
			return err
		}

		conf := demoNginxConfig(spec, cfg)
		confPath := filepath.Join(siteD, spec.Domain+".conf")
		if err := os.WriteFile(confPath, []byte(conf), 0o644); err != nil {
			return err
		}
		if spec.Active {
			link := filepath.Join(activeD, spec.Domain+".conf")
			_ = os.Remove(link)
			if err := os.Symlink(filepath.Join("..", "site.d", spec.Domain+".conf"), link); err != nil {
				return fmt.Errorf("enable demo site %s: %w", spec.Domain, err)
			}
		}

		site, err := repo.Create(ctx, sqlite.Website{
			Name:     spec.Name,
			Domain:   spec.Domain,
			Path:     sitePath,
			Type:     spec.Type,
			Upstream: spec.Upstream,
			SSL:      spec.SSL,
			Active:   spec.Active,
			Config:   conf,
		})
		if err != nil {
			return fmt.Errorf("seed website %s: %w", spec.Domain, err)
		}

		if spec.SSL && spec.SSLTTL > 0 {
			if err := seedDemoSSL(cfg, spec.Domain, confPath, spec.SSLTTL); err != nil {
				return err
			}
			site.SSL = true
			if _, err := repo.Update(ctx, site); err != nil {
				return err
			}
		}
	}

	if err := seedDemoLogs(logDir); err != nil {
		return err
	}
	if err := seedDemoAudit(ctx, auditRepo); err != nil {
		return err
	}
	if err := seedDemoTraffic(ctx, metricsRepo); err != nil {
		return err
	}

	return nil
}

func shouldSeedDemo(cfg config.Config) bool {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("DEMO_SEED"))); v == "true" || v == "1" {
		return true
	}
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("DEMO_SEED"))); v == "false" || v == "0" {
		return false
	}
	return cfg.AppEnv == "local"
}

func demoNginxConfig(spec demoSiteSpec, cfg config.Config) string {
	if spec.Type == sqlite.WebsiteTypeProxy {
		return fmt.Sprintf(`server {
    listen 80;
    server_name %s;
    location / {
        proxy_pass %s;
    }
}
`, spec.Domain, spec.Upstream)
	}
	return fmt.Sprintf(`server {
    listen 80;
    server_name %s;
    root %s;
    index index.html;
}
`, spec.Domain, filepath.Join(cfg.WebPath, spec.Slug))
}

func seedDemoSSL(cfg config.Config, domain, confPath string, ttl time.Duration) error {
	certPEM, keyPEM, err := generateDemoCert(domain, ttl)
	if err != nil {
		return err
	}
	liveDir := filepath.Join(cfg.Storage, "webconfig", "ssl", "live", domain)
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		return err
	}
	certPath := filepath.Join(liveDir, "cert.pem")
	keyPath := filepath.Join(liveDir, "key.pem")
	if err := os.WriteFile(certPath, []byte(certPEM), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(keyPath, []byte(keyPEM), 0o600); err != nil {
		return err
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}
	updated := nginx.UpdateSSLDirectives(string(data), certPath, keyPath)
	return os.WriteFile(confPath, []byte(updated), 0o644)
}

func generateDemoCert(domain string, ttl time.Duration) (string, string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(ttl),
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", err
	}
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
	return certPEM, keyPEM, nil
}

func seedDemoLogsIfMissing(cfg config.Config) error {
	logDir := cfg.LogsDir()
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "access") && strings.HasSuffix(e.Name(), ".log") {
			info, ierr := e.Info()
			if ierr == nil && info.Size() > 200 {
				return nil
			}
		}
	}
	return seedDemoLogs(logDir)
}

func seedDemoLogs(logDir string) error {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	now := time.Now().UTC()
	var globalLines []string
	for _, spec := range demoSites {
		lines := demoAccessLines(spec.Domain, now, 40)
		globalLines = append(globalLines, lines...)
		path := filepath.Join(logDir, fmt.Sprintf("access-%s.log", spec.Domain))
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			return err
		}
		errLines := demoErrorLines(spec.Domain, 6)
		errPath := filepath.Join(logDir, fmt.Sprintf("error-%s.log", spec.Domain))
		if err := os.WriteFile(errPath, []byte(strings.Join(errLines, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	globalLines = append(globalLines, demoAccessLines("default", now, 20)...)
	if err := os.WriteFile(filepath.Join(logDir, "access.log"), []byte(strings.Join(globalLines, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(logDir, "error.log"), []byte(strings.Join(demoErrorLines("default", 4), "\n")+"\n"), 0o644)
}

func demoAccessLines(domain string, now time.Time, count int) []string {
	paths := []string{"/", "/assets/app.js", "/api/health", "/login", "/favicon.ico"}
	agents := []string{`"Mozilla/5.0"`, `"curl/8.0"`, `"Googlebot/2.1"`}
	statuses := []int{200, 200, 304, 404, 500}
	var lines []string
	for i := 0; i < count; i++ {
		ts := now.Add(-time.Duration(i*7) * time.Minute)
		stamp := ts.Format("02/Jan/2006:15:04:05 -0700")
		path := paths[i%len(paths)]
		status := statuses[i%len(statuses)]
		bytes := 512 + (i * 37)
		ip := fmt.Sprintf("203.0.113.%d", (i%200)+1)
		lines = append(lines, fmt.Sprintf(
			`%s - - [%s] "GET %s HTTP/1.1" %d %d "-" %s`,
			ip, stamp, path, status, bytes, agents[i%len(agents)],
		))
		_ = domain
	}
	return lines
}

func demoErrorLines(domain string, count int) []string {
	var lines []string
	for i := 0; i < count; i++ {
		lines = append(lines, fmt.Sprintf(
			`2026/06/14 10:%02d:01 [error] 1234#0: *%d open() "%s/index.html" failed (2: No such file or directory)`,
			i, i+1, domain,
		))
	}
	return lines
}

func seedDemoAuditIfEmpty(ctx context.Context, db *sql.DB) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM audit_logs`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return seedDemoAudit(ctx, sqlite.NewAuditRepository(db))
}

func seedDemoAudit(ctx context.Context, repo *sqlite.AuditRepository) error {
	events := []sqlite.AuditLog{
		{UserEmail: "admin@demo.com", Action: "login", Status: "ok", Message: "signed in"},
		{UserEmail: "admin@demo.com", Action: "website.create", ResourceType: "website", Domain: "marketing.demo.local", Status: "ok", Message: "seeded demo site"},
		{UserEmail: "admin@demo.com", Action: "nginx.reload", Status: "ok", Message: "configuration reloaded"},
		{UserEmail: "admin@demo.com", Action: "ssl.manual", Domain: "marketing.demo.local", Status: "ok", Message: "certificate uploaded"},
		{UserEmail: "admin@demo.com", Action: "docker.restart", ResourceType: "container", Status: "ok", Message: "container restarted"},
		{UserEmail: "admin@demo.com", Action: "cron.run", ResourceType: "cronjob", Status: "ok", Message: "manual run queued"},
		{UserEmail: "admin@demo.com", Action: "file.upload", Status: "ok", Message: "uploaded index.html"},
		{UserEmail: "admin@demo.com", Action: "settings.profile", Status: "ok", Message: "profile updated"},
		{UserEmail: "admin@demo.com", Action: "website.toggle", Domain: "staging.demo.local", Status: "ok", Message: "site disabled"},
		{UserEmail: "admin@demo.com", Action: "query.run", Status: "ok", Message: "audit search"},
	}
	now := time.Now().UTC()
	for i, ev := range events {
		ev.Timestamp = now.Add(-time.Duration(i*15) * time.Minute)
		if err := repo.Write(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

func seedDemoTraffic(ctx context.Context, repo *sqlite.TrafficMetricsRepository) error {
	now := time.Now().UTC().Truncate(5 * time.Minute)
	sites := []string{"default", "marketing.demo.local", "blog.demo.local", "api.demo.local"}
	for h := 0; h < 24; h++ {
		bucket := now.Add(-time.Duration(h*5) * time.Minute)
		for si, site := range sites {
			req := 20 + h*3 + si*5
			if err := repo.UpsertBucket(ctx, sqlite.TrafficBucket{
				BucketTS: bucket,
				Site:     site,
				Requests: req,
				Bytes:    req * 1200,
				S2xx:     req - 2,
				S3xx:     1,
				S4xx:     1,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
