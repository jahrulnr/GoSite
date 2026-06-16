package types

import "time"

// Source describes where to obtain a plugin artifact.
type Source struct {
	Type         string `json:"type"`
	URL          string `json:"url,omitempty"`
	Repo         string `json:"repo,omitempty"`
	Tag          string `json:"tag,omitempty"`
	Asset        string `json:"asset,omitempty"`
	SHA256       string `json:"sha256,omitempty"`
	InstallPath  string `json:"installPath,omitempty"`
	ResolveToken string `json:"resolveToken,omitempty"`
}

// FetchPlan is the outcome of resolving a source to a concrete download target.
type FetchPlan struct {
	URL            string `json:"url"`
	SHA256         string `json:"sha256"`
	Size           int64  `json:"size,omitempty"`
	SourceType     string `json:"source_type"`
	SourceRef      string `json:"source_ref"`
	InstallPath    string `json:"install_path"`
	ResolvedDigest string `json:"resolved_digest"`
	SourceCommit   string `json:"source_commit,omitempty"`
	SourceRepository string `json:"source_repository,omitempty"`
}

// ResolvePreview is returned by POST /plugins/install/resolve.
type ResolvePreview struct {
	PluginID           string     `json:"plugin_id,omitempty"`
	Version            string     `json:"version,omitempty"`
	Tier               int        `json:"tier,omitempty"`
	Signed             bool       `json:"signed"`
	KeyID              string     `json:"keyId,omitempty"`
	SHA256             string     `json:"sha256"`
	Size               int64      `json:"size,omitempty"`
	URL                string     `json:"url,omitempty"`
	MinGoSiteVersion   string     `json:"minGoSiteVersion,omitempty"`
	SourceType         string     `json:"source_type"`
	SourceRef          string     `json:"source_ref"`
	InstallPath        string     `json:"install_path"`
	SourceCommit       string     `json:"sourceCommit,omitempty"`
	SourceRepository   string     `json:"sourceRepository,omitempty"`
	Permissions        []string   `json:"permissions,omitempty"`
	Hooks              []string   `json:"hooks,omitempty"`
	SupportedPlatforms []Platform `json:"supportedPlatforms,omitempty"`
	ResolveToken       string     `json:"resolveToken,omitempty"`
	ResolveExpiresAt   string     `json:"resolveExpiresAt,omitempty"`
}

// Platform is one GOOS/GOARCH pair advertised by an index.
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// InstallRequest is the JSON body for remote install.
type InstallRequest struct {
	Source         Source `json:"source"`
	PermissionsAck bool   `json:"permissions_ack"`
	ResolveToken   string `json:"resolveToken,omitempty"`
}

// Config holds remote install host settings.
type Config struct {
	Enabled         bool
	AllowedHosts    []string
	MaxBytes        int64
	Timeout         time.Duration
	MaxRedirects    int
	GitHubToken     string
	GitLabToken     string
	TrustMode       string
	ResolveTokenTTL time.Duration
}
