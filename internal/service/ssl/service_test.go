package ssl_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/ssl"
	"github.com/jahrulnr/gosite/internal/service/website"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSLManual_UpdatesConfig(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "ssl.example.com",
		Path:   filepath.Join(stack.WebRoot, "ssl"),
	})
	require.NoError(t, err)

	certPEM, keyPEM := generateTestCert(t)

	require.NoError(t, stack.SSLSvc.UpdateManual(ctx, site.ID, ssl.ManualInput{
		Public:  certPEM,
		Private: keyPEM,
	}))

	cfg, err := stack.Nginx.ReadSiteConfig(ctx, site.Domain)
	require.NoError(t, err)
	assert.Contains(t, cfg, "ssl_certificate")
	assert.Contains(t, cfg, "ssl_certificate_key")

	status, err := stack.SSLSvc.GetStatus(ctx, site.ID)
	require.NoError(t, err)
	assert.True(t, status.Enabled)
	assert.NotEmpty(t, status.PublicPEM)
}

func TestEnqueueCertbot_CreatesPendingJob(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "certbot.example.com",
		Path:   filepath.Join(stack.WebRoot, "certbot"),
	})
	require.NoError(t, err)

	job, err := stack.SSLSvc.EnqueueCertbot(ctx, site.ID)
	require.NoError(t, err)
	assert.Equal(t, "certbot", job.JobType)
	assert.Equal(t, "pending", job.Status)
	assert.Contains(t, job.Output, "certbot")
}

func TestEnqueueCertbot_RunsWorker(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "certbot-run.example.com",
		Path:   filepath.Join(stack.WebRoot, "certbot-run"),
	})
	require.NoError(t, err)

	stack.Cmd.Stdout = "certbot ok"
	job, err := stack.SSLSvc.EnqueueCertbot(ctx, site.ID)
	require.NoError(t, err)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stored, findErr := stack.JobRepo.FindByID(ctx, job.ID)
		require.NoError(t, findErr)
		if stored.Status != sqlite.JobStatusPending {
			assert.Equal(t, sqlite.JobStatusOK, stored.Status)
			assert.Contains(t, stored.Output, "certbot ok")
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("certbot job did not finish")
}

func TestParseCertExpiry_ValidCert(t *testing.T) {
	certPEM, _ := generateTestCert(t)
	exp, expired, err := ssl.ParseCertExpiry([]byte(certPEM))
	require.NoError(t, err)
	assert.False(t, expired)
	assert.True(t, exp.After(time.Now()))
}

func TestParseCertExpiry_InvalidPEM(t *testing.T) {
	_, _, err := ssl.ParseCertExpiry([]byte("not a cert"))
	require.Error(t, err)
}

func TestValidatePEM_RejectsBadInput(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "badssl.example.com",
		Path:   filepath.Join(stack.WebRoot, "badssl"),
	})
	require.NoError(t, err)

	err = stack.SSLSvc.UpdateManual(ctx, site.ID, ssl.ManualInput{
		Public:  "invalid",
		Private: "invalid",
	})
	require.Error(t, err)
}

func TestGetCertbotJob_ReturnsJob(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "job.example.com",
		Path:   filepath.Join(stack.WebRoot, "job"),
	})
	require.NoError(t, err)

	job, err := stack.SSLSvc.EnqueueCertbot(ctx, site.ID)
	require.NoError(t, err)

	got, err := stack.SSLSvc.GetCertbotJob(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, got.ID)
}

func TestGetCertbotJob_NotFound(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	_, err := stack.SSLSvc.GetCertbotJob(context.Background(), 99999)
	require.Error(t, err)
}

func TestListExpiring_FindsSoonExpiringCert(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "expire.example.com",
		Path:   filepath.Join(stack.WebRoot, "expire"),
	})
	require.NoError(t, err)

	certPEM, keyPEM := generateExpiringCert(t, 12*time.Hour)
	require.NoError(t, stack.SSLSvc.UpdateManual(ctx, site.ID, ssl.ManualInput{
		Public:  certPEM,
		Private: keyPEM,
	}))

	list, err := stack.SSLSvc.ListExpiring(ctx, 30)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, site.Domain, list[0].Domain)
}

func TestUpdateManual_NoExistingNginxConfig(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteRepo.Create(ctx, sqlite.Website{
		Name:   "raw",
		Domain: "rawssl.example.com",
		Path:   filepath.Join(stack.WebRoot, "rawssl"),
	})
	require.NoError(t, err)

	certPEM, keyPEM := generateTestCert(t)
	require.NoError(t, stack.SSLSvc.UpdateManual(ctx, site.ID, ssl.ManualInput{
		Public:  certPEM,
		Private: keyPEM,
	}))

	cfg, err := stack.Nginx.ReadSiteConfig(ctx, site.Domain)
	require.NoError(t, err)

	liveCert := filepath.Join(stack.Storage, "webconfig/ssl/live", site.Domain, "cert.pem")
	_, err = os.Stat(liveCert)
	require.NoError(t, err)
	_ = cfg
}

func TestEnqueueCertbot_SiteNotFound(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	_, err := stack.SSLSvc.EnqueueCertbot(context.Background(), 99999)
	require.Error(t, err)
}

func TestEnqueueCertbot_ClearsPlaceholderSSL(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "placeholder-clear.example.com",
		Path:   filepath.Join(stack.WebRoot, "placeholder-clear"),
		Active: true,
	})
	require.NoError(t, err)

	liveDir := filepath.Join(stack.Storage, "webconfig/ssl/live", site.Domain)
	_, err = os.Stat(filepath.Join(liveDir, "cert.pem"))
	require.NoError(t, err)

	_, err = stack.SSLSvc.EnqueueCertbot(ctx, site.ID)
	require.NoError(t, err)

	_, err = os.Stat(liveDir)
	assert.True(t, os.IsNotExist(err), "placeholder live dir must be removed before certbot")

	cfg, err := stack.Nginx.ReadSiteConfig(ctx, site.Domain)
	require.NoError(t, err)
	defaultCert := filepath.Join(stack.Nginx.Paths().SSLDefaultDir, "cert.pem")
	assert.Contains(t, cfg, defaultCert)
}

func TestListExpiring_DefaultWithinDays(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "expire-default.example.com",
		Path:   filepath.Join(stack.WebRoot, "expire-default"),
	})
	require.NoError(t, err)

	certPEM, keyPEM := generateExpiringCert(t, 6*time.Hour)
	require.NoError(t, stack.SSLSvc.UpdateManual(ctx, site.ID, ssl.ManualInput{
		Public:  certPEM,
		Private: keyPEM,
	}))

	list, err := stack.SSLSvc.ListExpiring(ctx, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, list)
}

func TestGetStatus_ReadsPrivateKey(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "keyread.example.com",
		Path:   filepath.Join(stack.WebRoot, "keyread"),
	})
	require.NoError(t, err)

	certPEM, keyPEM := generateTestCert(t)
	require.NoError(t, stack.SSLSvc.UpdateManual(ctx, site.ID, ssl.ManualInput{
		Public:  certPEM,
		Private: keyPEM,
	}))

	status, err := stack.SSLSvc.GetStatus(ctx, site.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, status.PrivatePEM)
}

func TestUpdateManual_SiteNotFound(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	certPEM, keyPEM := generateTestCert(t)
	err := stack.SSLSvc.UpdateManual(context.Background(), 99999, ssl.ManualInput{
		Public: certPEM, Private: keyPEM,
	})
	require.Error(t, err)
}

func TestGetStatus_SiteNotFound(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	_, err := stack.SSLSvc.GetStatus(context.Background(), 99999)
	require.Error(t, err)
}

func TestGetStatus_NoConfig(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteRepo.Create(ctx, sqlite.Website{
		Name:   "noconf",
		Domain: "noconf.example.com",
		Path:   filepath.Join(stack.WebRoot, "noconf"),
	})
	require.NoError(t, err)

	status, err := stack.SSLSvc.GetStatus(ctx, site.ID)
	require.NoError(t, err)
	assert.False(t, status.Enabled)
}

func generateTestCert(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test.local"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyBytes, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
	return certPEM, keyPEM
}

func generateExpiringCert(t *testing.T, ttl time.Duration) (certPEM, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "expire.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(ttl),
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyBytes, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
	return certPEM, keyPEM
}
