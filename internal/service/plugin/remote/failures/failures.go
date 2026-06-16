package failures

// Install-time failure_class values for remote distribution (wave G).
const (
	ResolveFailed         = "resolve_failed"
	FetchFailed           = "fetch_failed"
	FetchDigestMismatch   = "fetch_digest_mismatch"
	ReleaseIntegrity      = "release_integrity_failed"
	FetchTooLarge         = "fetch_too_large"
	PlatformUnsupported   = "platform_unsupported"
	AuthTokenExpired      = "auth_token_expired"
	ResolveStale          = "resolve_stale"
	OperationInProgress   = "operation_in_progress"
	RemoteInstallDisabled = "remote_install_disabled"
)
