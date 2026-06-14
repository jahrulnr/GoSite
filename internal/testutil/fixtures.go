package testutil

// SampleNginxSiteConfig is a minimal static vhost used in tests.
const SampleNginxSiteConfig = `server {
    listen 80;
    server_name example.test;
    root /www/example;
    index index.html;
}
`

// SampleNginxProxyConfig is a minimal reverse-proxy vhost used in tests.
const SampleNginxProxyConfig = `server {
    listen 80;
    server_name proxy.test;
    location / {
        proxy_pass http://127.0.0.1:9000;
    }
}
`

// SamplePEMCert is a tiny self-signed PEM block for SSL parser tests.
const SamplePEMCert = `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKexampleTESTCERT0MA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNVBAMMCWxv
Y2FsaG9zdDAeFw0yNDAxMDEwMDAwMDBaFw0yNTAxMDEwMDAwMDBaMBQxEjAQBgNVBAMM
CWxvY2FsaG9zdDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABExamplePublicKeyData
-----END CERTIFICATE-----
`

// SampleAccessLogLines are nginx combined-log lines for metrics/parser tests.
var SampleAccessLogLines = []string{
	`127.0.0.1 - - [14/Jun/2026:10:00:01 +0000] "GET / HTTP/1.1" 200 1234 "-" "curl/8.0"`,
	`127.0.0.1 - - [14/Jun/2026:10:00:02 +0000] "GET /api HTTP/1.1" 404 89 "-" "curl/8.0"`,
}

// LegacyAdminEmail is the default seeded admin account from BangunSite.
const LegacyAdminEmail = "admin@demo.com"

// LegacyAdminPassword is the default seeded admin password from BangunSite.
const LegacyAdminPassword = "123456"
