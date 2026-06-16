package remote

import (
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/failures"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
)

// Re-export JSON/API types for handlers.
type (
	Source         = types.Source
	FetchPlan      = types.FetchPlan
	ResolvePreview = types.ResolvePreview
	Platform       = types.Platform
	InstallRequest = types.InstallRequest
	Config         = types.Config
)

// Failure class constants.
const (
	FailureResolveFailed         = failures.ResolveFailed
	FailureFetchFailed           = failures.FetchFailed
	FailureFetchDigestMismatch   = failures.FetchDigestMismatch
	FailureReleaseIntegrity      = failures.ReleaseIntegrity
	FailureFetchTooLarge         = failures.FetchTooLarge
	FailurePlatformUnsupported   = failures.PlatformUnsupported
	FailureAuthTokenExpired      = failures.AuthTokenExpired
	FailureResolveStale          = failures.ResolveStale
	FailureOperationInProgress   = failures.OperationInProgress
	FailureRemoteInstallDisabled = failures.RemoteInstallDisabled
)

// ConfigFromApp maps application config to remote install settings.
func ConfigFromApp(cfg config.Config) Config {
	trust := cfg.PluginTrustMode
	if trust == "" {
		if cfg.AppEnv == "production" {
			trust = "strict"
		} else {
			trust = "community"
		}
	}
	return Config{
		Enabled:         cfg.PluginRemoteInstall,
		AllowedHosts:    cfg.PluginInstallAllowedHosts,
		MaxBytes:        cfg.PluginFetchMaxBytes,
		Timeout:         cfg.PluginFetchTimeout,
		MaxRedirects:    cfg.PluginFetchMaxRedirects,
		GitHubToken:     cfg.GitHubToken,
		GitLabToken:     cfg.GitLabToken,
		TrustMode:       trust,
		ResolveTokenTTL: 15 * time.Minute,
		AllowUnsigned:         cfg.PluginAllowUnsigned,
		GitHubTokenConfigured: strings.TrimSpace(cfg.GitHubToken) != "",
		GitLabTokenConfigured: strings.TrimSpace(cfg.GitLabToken) != "",
	}
}
