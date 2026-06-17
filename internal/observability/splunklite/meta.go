package splunklite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jahrulnr/gosite/internal/service/logs"
)

type SiteLister interface {
	ListSites(ctx context.Context) ([]logs.SiteOption, error)
}

type QueryOption struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Hint     string `json:"hint,omitempty"`
	OffsetMs int64  `json:"offset_ms,omitempty"`
}

type QueryFieldMeta struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder,omitempty"`
}

type QuerySourcePayload struct {
	Source string `json:"source"`
	Site   string `json:"site,omitempty"`
}

type QuerySourceMeta struct {
	ID           string             `json:"id"`
	Label        string             `json:"label"`
	Group        string             `json:"group"`
	Description  string             `json:"description"`
	LogPath      string             `json:"log_path,omitempty"`
	Query        QuerySourcePayload `json:"query"`
	Fields       []QueryFieldMeta   `json:"fields"`
	QuickFilters []QueryOption      `json:"quick_filters"`
	Examples     []string           `json:"examples"`
}

type QueryMeta struct {
	SyntaxHint string            `json:"syntax_hint"`
	TimeRanges []QueryOption     `json:"time_ranges"`
	Sources    []QuerySourceMeta `json:"sources"`
}

type MetaService struct {
	sites  SiteLister
	logDir string
}

func NewMetaService(sites SiteLister, logDir string) *MetaService {
	return &MetaService{sites: sites, logDir: logDir}
}

func (s *MetaService) Meta(ctx context.Context) (QueryMeta, error) {
	out := QueryMeta{
		SyntaxHint: "Type plain words or field:value, or /regex/ (Go regexp) with field: prefix. Use * as wildcard. Space means AND.",
		TimeRanges: []QueryOption{
			{Value: "1h", Label: "Last hour", OffsetMs: 3_600_000},
			{Value: "6h", Label: "Last 6 hours", OffsetMs: 21_600_000},
			{Value: "24h", Label: "Last 24 hours", OffsetMs: 86_400_000},
			{Value: "7d", Label: "Last 7 days", OffsetMs: 604_800_000},
			{Value: "30d", Label: "Last 30 days", OffsetMs: 2_592_000_000},
			{Value: "all", Label: "All time"},
		},
	}
	out.Sources = append(out.Sources, systemSources()...)
	sites, err := s.listSites(ctx)
	if err != nil {
		return QueryMeta{}, err
	}
	out.Sources = append(out.Sources, s.nginxSources(sites)...)
	return out, nil
}

func (s *MetaService) listSites(ctx context.Context) ([]logs.SiteOption, error) {
	seen := map[string]logs.SiteOption{}
	if s.sites != nil {
		sites, err := s.sites.ListSites(ctx)
		if err != nil {
			return nil, err
		}
		for _, site := range sites {
			domain := strings.TrimSpace(site.Domain)
			if domain == "" {
				continue
			}
			seen[domain] = site
		}
	}
	for _, site := range s.scanLogSites() {
		if _, ok := seen[site.Domain]; !ok {
			seen[site.Domain] = site
		}
	}
	if _, ok := seen["default"]; !ok {
		seen["default"] = logs.SiteOption{Domain: "default", Name: "Default"}
	}
	out := make([]logs.SiteOption, 0, len(seen))
	for _, site := range seen {
		out = append(out, site)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Domain == "default" {
			return true
		}
		if out[j].Domain == "default" {
			return false
		}
		return out[i].Domain < out[j].Domain
	})
	return out, nil
}

func (s *MetaService) scanLogSites() []logs.SiteOption {
	entries, err := os.ReadDir(s.logDir)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		domain := ""
		switch {
		case name == "access.log" || name == "error.log":
			domain = "default"
		case strings.HasPrefix(name, "access-") && strings.HasSuffix(name, ".log"):
			domain = strings.TrimSuffix(strings.TrimPrefix(name, "access-"), ".log")
		case strings.HasPrefix(name, "error-") && strings.HasSuffix(name, ".log"):
			domain = strings.TrimSuffix(strings.TrimPrefix(name, "error-"), ".log")
		}
		if domain != "" {
			seen[domain] = true
		}
	}
	out := make([]logs.SiteOption, 0, len(seen))
	for domain := range seen {
		out = append(out, logs.SiteOption{Domain: domain})
	}
	return out
}

func systemSources() []QuerySourceMeta {
	return []QuerySourceMeta{
		{
			ID: "audit", Label: "audit_logs", Group: "system",
			Description: "Panel actions such as sign-ins, website changes, nginx reloads, and settings updates.",
			Query:       QuerySourcePayload{Source: SourceAudit},
			Fields: []QueryFieldMeta{
				{Name: "action", Label: "Action", Placeholder: "login"},
				{Name: "user", Label: "User", Placeholder: "admin@example.com"},
				{Name: "status", Label: "Status", Placeholder: "ok"},
				{Name: "domain", Label: "Domain", Placeholder: "example.com"},
				{Name: "message", Label: "Message", Placeholder: "created"},
			},
			QuickFilters: []QueryOption{
				{Value: "action:login", Label: "Sign-ins"},
				{Value: "action:logout", Label: "Sign-outs"},
				{Value: "action:integration_token.created", Label: "Integration tokens"},
				{Value: "action:integration_token.used", Label: "Token API use"},
				{Value: "status:failed", Label: "Failed"},
			},
			Examples: []string{"action:login", "user:admin@demo.com", "domain:example.com"},
		},
		{
			ID: "job", Label: "job_runs", Group: "system",
			Description: "Background job runs such as SSL, cron, and command execution.",
			Query:       QuerySourcePayload{Source: SourceJob},
			Fields: []QueryFieldMeta{
				{Name: "job_type", Label: "Job type", Placeholder: "ssl.certbot"},
				{Name: "name", Label: "Name", Placeholder: "example.com"},
				{Name: "status", Label: "Status", Placeholder: "ok"},
				{Name: "message", Label: "Message", Placeholder: "renew"},
			},
			QuickFilters: []QueryOption{{Value: "job_type:ssl.*", Label: "SSL jobs"}, {Value: "job_type:cron.*", Label: "Cron runs"}, {Value: "status:failed", Label: "Failed"}},
			Examples:     []string{"ssl.certbot", "job_type:cron.*", "status:failed"},
		},
		{
			ID: "all", Label: "all_sources", Group: "system",
			Description:  "Search audit logs, job runs, and nginx logs together.",
			Query:        QuerySourcePayload{Source: SourceAll},
			Fields:       []QueryFieldMeta{{Name: "_text", Label: "Any word", Placeholder: "login"}},
			QuickFilters: []QueryOption{},
			Examples:     []string{},
		},
	}
}

func (s *MetaService) nginxSources(sites []logs.SiteOption) []QuerySourceMeta {
	out := make([]QuerySourceMeta, 0, len(sites)*2)
	for _, site := range sites {
		domain := strings.TrimSpace(site.Domain)
		if domain == "" {
			continue
		}
		name := site.Name
		if strings.TrimSpace(name) == "" {
			name = domain
		}
		for _, kind := range []string{SourceAccess, SourceError} {
			id := fmt.Sprintf("%s:%s", kind, domain)
			label := logFileName(kind, domain)
			desc := fmt.Sprintf("Nginx %s log for %s.", kind, name)
			out = append(out, QuerySourceMeta{
				ID:           id,
				Label:        label,
				Group:        "nginx",
				Description:  desc,
				LogPath:      filepath.Join(s.logDir, label),
				Query:        QuerySourcePayload{Source: kind, Site: domain},
				Fields:       nginxFields(kind, domain),
				QuickFilters: nginxQuickFilters(kind),
				Examples:     nginxExamples(kind, domain),
			})
		}
	}
	return out
}

func logFileName(kind, domain string) string {
	if domain == "default" {
		return kind + ".log"
	}
	return fmt.Sprintf("%s-%s.log", kind, domain)
}

func nginxFields(kind, domain string) []QueryFieldMeta {
	fields := []QueryFieldMeta{
		{Name: "site", Label: "Site", Placeholder: domain},
		{Name: "message", Label: "Message", Placeholder: "GET /"},
	}
	if kind == SourceAccess {
		fields = append([]QueryFieldMeta{{Name: "status_code", Label: "Status code", Placeholder: "404"}}, fields...)
	}
	return fields
}

func nginxQuickFilters(kind string) []QueryOption {
	if kind == SourceAccess {
		return []QueryOption{{Value: "status_code:404", Label: "404s"}, {Value: "status_code:500", Label: "500s"}, {Value: "GET", Label: "GET requests"}}
	}
	return []QueryOption{{Value: "error", Label: "Errors"}, {Value: "timeout", Label: "Timeouts"}, {Value: "failed", Label: "Failed"}}
}

func nginxExamples(kind, domain string) []string {
	if kind == SourceAccess {
		return []string{fmt.Sprintf("site:%s", domain), "status_code:5\\d\\d", "/GET|POST/"}
	}
	return []string{fmt.Sprintf("site:%s", domain), "timeout", "/open\\(\\) .* failed/"}
}
