package website

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/jahrulnr/gosite/internal/infra/nginx"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Service manages website lifecycle with nginx side effects.
type Service struct {
	repo   *sqlite.WebsiteRepository
	nginx  *nginx.Service
	webRoot string
}

// NewService returns a website service.
func NewService(repo *sqlite.WebsiteRepository, ngx *nginx.Service, webRoot string) *Service {
	return &Service{repo: repo, nginx: ngx, webRoot: webRoot}
}

// CreateInput holds fields for website creation.
type CreateInput struct {
	Name     string
	Domain   string
	Path     string
	Type     string
	Upstream string
	Active   bool
}

// UpdateInput holds mutable website fields.
type UpdateInput struct {
	Name     string
	Domain   string
	Path     string
	Type     string
	Upstream string
	Active   bool
}

// ValidateResult is the outcome of domain/path validation.
type ValidateResult struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason,omitempty"`
}

// Create validates, persists, and provisions nginx config for a website.
func (s *Service) Create(ctx context.Context, in CreateInput) (sqlite.Website, error) {
	if err := s.validateCreateUpdate(ctx, in.Domain, in.Path, 0); err != nil {
		return sqlite.Website{}, err
	}

	siteType := in.Type
	if siteType == "" {
		siteType = sqlite.WebsiteTypeStatic
	}
	if siteType == sqlite.WebsiteTypeProxy && strings.TrimSpace(in.Upstream) == "" {
		return sqlite.Website{}, apperror.New(apperror.CodeValidation, "upstream required for proxy type")
	}

	name := in.Name
	if name == "" {
		name = in.Domain
	}

	site, err := s.repo.Create(ctx, sqlite.Website{
		Name:     name,
		Domain:   in.Domain,
		Path:     in.Path,
		Type:     siteType,
		Upstream: in.Upstream,
		Active:   in.Active,
	})
	if err != nil {
		return sqlite.Website{}, apperror.Wrap(apperror.CodeDatabase, "create website", err)
	}

	if err := s.provisionSite(ctx, site); err != nil {
		_ = s.repo.Delete(ctx, site.ID)
		return sqlite.Website{}, err
	}

	if site.Active {
		if err := s.nginx.EnableSite(ctx, site.Domain); err != nil {
			_ = s.repo.Delete(ctx, site.ID)
			return sqlite.Website{}, apperror.Wrap(apperror.CodeNginxReloadFailed, "enable site", err)
		}
		if err := s.nginx.Reload(ctx); err != nil {
			_ = s.nginx.DisableSite(ctx, site.Domain)
			_ = s.repo.Delete(ctx, site.ID)
			return sqlite.Website{}, err
		}
	}

	return site, nil
}

// Update validates and updates a website record and its nginx config.
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (sqlite.Website, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return sqlite.Website{}, apperror.Wrap(apperror.CodeNotFound, "website not found", err)
	}

	if err := s.validateCreateUpdate(ctx, in.Domain, in.Path, id); err != nil {
		return sqlite.Website{}, err
	}

	siteType := in.Type
	if siteType == "" {
		siteType = sqlite.WebsiteTypeStatic
	}
	if siteType == sqlite.WebsiteTypeProxy && strings.TrimSpace(in.Upstream) == "" {
		return sqlite.Website{}, apperror.New(apperror.CodeValidation, "upstream required for proxy type")
	}

	name := in.Name
	if name == "" {
		name = in.Domain
	}

	updated := existing
	updated.Name = name
	updated.Domain = in.Domain
	updated.Path = in.Path
	updated.Type = siteType
	updated.Upstream = in.Upstream
	updated.Active = in.Active

	site, err := s.repo.Update(ctx, updated)
	if err != nil {
		return sqlite.Website{}, apperror.Wrap(apperror.CodeDatabase, "update website", err)
	}

	if err := s.writeRenderedConfig(ctx, site); err != nil {
		return sqlite.Website{}, err
	}

	if site.Active {
		if err := s.nginx.EnableSite(ctx, site.Domain); err != nil {
			return sqlite.Website{}, err
		}
	} else {
		if err := s.nginx.DisableSite(ctx, site.Domain); err != nil {
			return sqlite.Website{}, err
		}
	}

	return site, nil
}

// Delete removes a website; clean explicitly removes document root when true.
func (s *Service) Delete(ctx context.Context, id int64, clean bool) error {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperror.Wrap(apperror.CodeNotFound, "website not found", err)
	}

	if err := s.nginx.DisableSite(ctx, site.Domain); err != nil {
		return apperror.Wrap(apperror.CodeNginxReloadFailed, "disable site", err)
	}
	if err := s.nginx.Reload(ctx); err != nil {
		return err
	}

	if clean {
		if err := os.RemoveAll(site.Path); err != nil && !os.IsNotExist(err) {
			return apperror.Wrap(apperror.CodeInternal, "remove site path", err)
		}
	}

	if err := s.nginx.RemoveSiteConfig(site.Domain); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "remove site config", err)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return apperror.Wrap(apperror.CodeDatabase, "delete website", err)
	}
	return nil
}

// Toggle flips active state with nginx reload; rolls back on reload failure.
func (s *Service) Toggle(ctx context.Context, id int64) (sqlite.Website, error) {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return sqlite.Website{}, apperror.Wrap(apperror.CodeNotFound, "website not found", err)
	}

	prevActive := site.Active
	site.Active = !site.Active

	if site.Active {
		if err := s.nginx.EnableSite(ctx, site.Domain); err != nil {
			return sqlite.Website{}, err
		}
	} else {
		if err := s.nginx.DisableSite(ctx, site.Domain); err != nil {
			return sqlite.Website{}, err
		}
	}

	if err := s.nginx.Reload(ctx); err != nil {
		if prevActive {
			_ = s.nginx.EnableSite(ctx, site.Domain)
		} else {
			_ = s.nginx.DisableSite(ctx, site.Domain)
		}
		return sqlite.Website{}, err
	}

	updated, err := s.repo.Update(ctx, site)
	if err != nil {
		if prevActive {
			_ = s.nginx.EnableSite(ctx, site.Domain)
		} else {
			_ = s.nginx.DisableSite(ctx, site.Domain)
		}
		_ = s.nginx.Reload(ctx)
		return sqlite.Website{}, apperror.Wrap(apperror.CodeDatabase, "update active flag", err)
	}
	return updated, nil
}

// Validate checks domain and path rules.
func (s *Service) Validate(ctx context.Context, domain, path string, excludeID int64) ValidateResult {
	if err := s.validateCreateUpdate(ctx, domain, path, excludeID); err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return ValidateResult{Valid: false, Reason: appErr.Message}
		}
		return ValidateResult{Valid: false, Reason: err.Error()}
	}
	return ValidateResult{Valid: true}
}

// Get returns a website by id.
func (s *Service) Get(ctx context.Context, id int64) (sqlite.Website, error) {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return sqlite.Website{}, apperror.Wrap(apperror.CodeNotFound, "website not found", err)
	}
	return site, nil
}

// List returns all websites.
func (s *Service) List(ctx context.Context) ([]sqlite.Website, error) {
	sites, err := s.repo.List(ctx)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "list websites", err)
	}
	return sites, nil
}

// UpdateNginxConfig updates raw site.d config with backup and test.
func (s *Service) UpdateNginxConfig(ctx context.Context, id int64, config string) error {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperror.Wrap(apperror.CodeNotFound, "website not found", err)
	}
	config = strings.ReplaceAll(config, "\r", "")
	if err := s.nginx.UpdateSiteConfig(ctx, site.Domain, config); err != nil {
		return err
	}
	return s.nginx.Reload(ctx)
}

// GetNginxConfig returns site.d config content.
func (s *Service) GetNginxConfig(ctx context.Context, id int64) (string, error) {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeNotFound, "website not found", err)
	}
	content, err := s.nginx.ReadSiteConfig(ctx, site.Domain)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeNotFound, "site config not found", err)
	}
	return content, nil
}

func (s *Service) validateCreateUpdate(ctx context.Context, domain, path string, excludeID int64) error {
	if !isValidDomain(domain) {
		return apperror.New(apperror.CodeDomainInvalid, "domain not valid")
	}
	if err := validatePath(path, s.webRoot); err != nil {
		return err
	}
	dup, err := s.repo.ExistsPathForOther(ctx, path, excludeID)
	if err != nil {
		return apperror.Wrap(apperror.CodeDatabase, "check path", err)
	}
	if dup {
		return apperror.New(apperror.CodePathDuplicate, "path used by other website")
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return apperror.New(apperror.CodePathIsFile, "path is file")
	}
	return nil
}

func (s *Service) provisionSite(ctx context.Context, site sqlite.Website) error {
	if site.Type == sqlite.WebsiteTypeStatic {
		if err := os.MkdirAll(site.Path, 0o755); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "create site path", err)
		}
		indexPath := filepath.Join(site.Path, "index.html")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			defaultIndex := filepath.Join(s.webRoot, "default", "index.html")
			if data, readErr := os.ReadFile(defaultIndex); readErr == nil {
				_ = os.WriteFile(indexPath, data, 0o644)
			} else {
				_ = os.WriteFile(indexPath, []byte("<html><body>Welcome</body></html>"), 0o644)
			}
		}
	}

	if err := s.nginx.EnsureDomainSSL(site.Domain); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "provision ssl", err)
	}

	return s.writeRenderedConfig(ctx, site)
}

func (s *Service) writeRenderedConfig(ctx context.Context, site sqlite.Website) error {
	cert, key := s.nginx.DefaultSSLPaths(site.Domain)
	data := nginx.SiteTemplateData{
		Domain:   site.Domain,
		Path:     site.Path,
		SSLCert:  cert,
		SSLKey:   key,
		Upstream: site.Upstream,
	}

	var content string
	var err error
	paths := s.nginx.Paths()
	if site.Type == sqlite.WebsiteTypeProxy {
		content, err = nginx.RenderProxy(paths.ProxyTpl, data)
	} else {
		content, err = nginx.RenderStatic(paths.StaticTpl, data)
	}
	if err != nil {
		return apperror.Wrap(apperror.CodeConfig, "render template", err)
	}

	if err := s.nginx.WriteSiteConfig(ctx, site.Domain, content); err != nil {
		return apperror.Wrap(apperror.CodeConfig, "write site config", err)
	}
	return nil
}

func isValidDomain(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}
	if strings.Contains(domain, "..") {
		return false
	}
	if _, err := net.LookupHost(domain); err == nil {
		return true
	}
	// Accept well-formed hostnames even when DNS lookup fails in tests.
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 63 {
			return false
		}
		for _, c := range p {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

func validatePath(path, webRoot string) error {
	if path == "" {
		return apperror.New(apperror.CodePathInvalid, "path not valid")
	}
	if strings.Contains(path, "..") {
		return apperror.New(apperror.CodePathTraversal, "path traversal rejected")
	}
	clean := filepath.Clean(path)
	if !strings.HasPrefix(clean, filepath.Clean(webRoot)) {
		return apperror.New(apperror.CodePathInvalid, "path outside web root")
	}
	return nil
}

// FormatToggleMessage returns legacy-compatible toggle message.
func FormatToggleMessage(active bool) string {
	if active {
		return "Site actived successfully"
	}
	return "Site disabled successfully"
}

// DeleteMessage is the legacy delete success message.
const DeleteMessage = "Site deleted successfully"
