package nginx

import (
	"fmt"
	"os"
	"strings"
)

// SiteTemplateData holds placeholder values for vhost templates.
type SiteTemplateData struct {
	Domain   string
	Path     string
	SSLCert  string
	SSLKey   string
	Upstream string
}

// RenderStatic renders the static site template (site.conf).
func RenderStatic(templatePath string, data SiteTemplateData) (string, error) {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read static template: %w", err)
	}
	return applyReplacements(string(content), data), nil
}

// RenderProxy renders the reverse-proxy template (site-proxy.conf).
func RenderProxy(templatePath string, data SiteTemplateData) (string, error) {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read proxy template: %w", err)
	}
	return applyReplacements(string(content), data), nil
}

func applyReplacements(template string, data SiteTemplateData) string {
	repl := map[string]string{
		"<domain>":   data.Domain,
		"<path>":     data.Path,
		"<ssl_cert>": data.SSLCert,
		"<ssl_key>":  data.SSLKey,
		"<upstream>": data.Upstream,
	}
	out := template
	for key, val := range repl {
		out = strings.ReplaceAll(out, key, val)
	}
	return out
}

// UpdateSSLDirectives replaces ssl_certificate and ssl_certificate_key lines.
func UpdateSSLDirectives(config, certPath, keyPath string) string {
	lines := strings.Split(config, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ssl_certificate ") && !strings.HasPrefix(trimmed, "ssl_certificate_key") {
			out = append(out, fmt.Sprintf("ssl_certificate %s;", certPath))
			continue
		}
		if strings.HasPrefix(trimmed, "ssl_certificate_key ") {
			out = append(out, fmt.Sprintf("ssl_certificate_key %s;", keyPath))
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// ParseCertPaths extracts ssl_certificate and ssl_certificate_key paths from config.
func ParseCertPaths(config string) (certPath, keyPath string, ok bool) {
	for _, line := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "ssl_certificate ") && !strings.HasPrefix(trimmed, "ssl_certificate_key") {
			certPath = strings.TrimSuffix(strings.TrimPrefix(trimmed, "ssl_certificate "), ";")
			certPath = strings.TrimSpace(certPath)
		}
		if strings.HasPrefix(trimmed, "ssl_certificate_key ") {
			keyPath = strings.TrimSuffix(strings.TrimPrefix(trimmed, "ssl_certificate_key "), ";")
			keyPath = strings.TrimSpace(keyPath)
		}
	}
	return certPath, keyPath, certPath != "" && keyPath != ""
}
