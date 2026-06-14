package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is a structured application error with HTTP status and stable code.
type Error struct {
	Code       Code   `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Cause      error  `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Response is the JSON envelope returned by HTTP handlers.
type Response struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the public error payload.
type ErrorBody struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

// Body returns the API error envelope.
func (e *Error) Body() Response {
	return Response{
		Error: ErrorBody{
			Code:    e.Code,
			Message: e.Message,
		},
	}
}

// New creates a structured error with defaults for the code.
func New(code Code, message string) *Error {
	status, msg := defaultsFor(code)
	if message != "" {
		msg = message
	}
	return &Error{
		Code:       code,
		Message:    msg,
		HTTPStatus: status,
	}
}

// Wrap attaches an underlying cause while preserving the structured error.
func Wrap(code Code, message string, cause error) *Error {
	err := New(code, message)
	err.Cause = cause
	return err
}

// From returns a structured error or wraps a generic error as internal.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Wrap(CodeInternal, "internal server error", err)
}

func defaultsFor(code Code) (int, string) {
	switch code {
	case CodeUnauthorized, CodeAuthInvalidCredentials, CodeBasicAuthRequired:
		return http.StatusUnauthorized, "unauthorized"
	case CodeForbidden:
		return http.StatusForbidden, "forbidden"
	case CodeNotFound, CodeDockerNotFound, CodeFileNotFound:
		return http.StatusNotFound, "not found"
	case CodeConflict, CodePathDuplicate:
		return http.StatusConflict, "conflict"
	case CodeValidation, CodeInvalidInput, CodeDomainInvalid, CodePathInvalid,
		CodePathIsFile, CodePathTraversal, CodeSSLInvalid, CodeCronInvalid,
		CodeQueryInvalid, CodeTimeRangeInvalid:
		return http.StatusBadRequest, "invalid request"
	case CodeNginxTestFailed, CodeNginxReloadFailed, CodeSSLExpired, CodeMountFailed,
		CodeJobFailed, CodeFileExecuteDisabled:
		return http.StatusUnprocessableEntity, "operation failed"
	case CodeDatabase:
		return http.StatusInternalServerError, "database error"
	case CodeConfig:
		return http.StatusInternalServerError, "configuration error"
	case CodeSessionExpired:
		return http.StatusUnauthorized, "session expired"
	default:
		return http.StatusInternalServerError, "internal server error"
	}
}
