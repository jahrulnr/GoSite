package apperror

// Code identifies a stable machine-readable error for API responses.
type Code string

const (
	CodeInternal           Code = "INTERNAL_ERROR"
	CodeValidation         Code = "VALIDATION_FAILED"
	CodeInvalidInput       Code = "INVALID_INPUT"
	CodeUnauthorized       Code = "UNAUTHORIZED"
	CodeForbidden          Code = "FORBIDDEN"
	CodeNotFound           Code = "NOT_FOUND"
	CodeConflict           Code = "CONFLICT"
	CodeDatabase           Code = "DATABASE_ERROR"
	CodeConfig             Code = "CONFIG_ERROR"

	CodeAuthInvalidCredentials Code = "AUTH_INVALID_CREDENTIALS"
	CodeBasicAuthRequired      Code = "BASIC_AUTH_REQUIRED"
	CodeSessionExpired         Code = "SESSION_EXPIRED"

	CodeDomainInvalid  Code = "DOMAIN_INVALID"
	CodePathInvalid    Code = "PATH_INVALID"
	CodePathDuplicate  Code = "PATH_DUPLICATE"
	CodePathIsFile     Code = "PATH_IS_FILE"
	CodePathTraversal  Code = "PATH_TRAVERSAL"

	CodeNginxTestFailed   Code = "NGINX_TEST_FAILED"
	CodeNginxReloadFailed Code = "NGINX_RELOAD_FAILED"

	CodeSSLInvalid Code = "SSL_INVALID"
	CodeSSLExpired Code = "SSL_EXPIRED"

	CodeDockerNotFound Code = "DOCKER_NOT_FOUND"

	CodeFileNotFound        Code = "FILE_NOT_FOUND"
	CodeFileExecuteDisabled Code = "FILE_EXECUTE_DISABLED"

	CodeMountFailed Code = "MOUNT_FAILED"
	CodeCronInvalid Code = "CRON_INVALID"
	CodeJobFailed   Code = "JOB_FAILED"

	CodeQueryInvalid     Code = "QUERY_INVALID"
	CodeTimeRangeInvalid Code = "TIME_RANGE_INVALID"
)
