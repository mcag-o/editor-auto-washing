package domain

import "fmt"

type ErrorCode string

const (
	ErrValidation ErrorCode = "VALIDATION_ERROR"
	ErrNotFound   ErrorCode = "NOT_FOUND"
	ErrConflict   ErrorCode = "CONFLICT"
	ErrExternal   ErrorCode = "EXTERNAL_ERROR"
	ErrInternal   ErrorCode = "INTERNAL_ERROR"
)

type AppError struct {
	Code    ErrorCode
	Message string
	Cause   error
	TraceID string
}

func (e AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e AppError) Unwrap() error {
	return e.Cause
}

func (e AppError) HTTPStatus() int {
	switch e.Code {
	case ErrValidation:
		return 400
	case ErrNotFound:
		return 404
	case ErrConflict:
		return 409
	case ErrExternal:
		return 502
	default:
		return 500
	}
}

func NewValidationErr(message string, cause error) *AppError {
	return &AppError{Code: ErrValidation, Message: message, Cause: cause}
}

func NewNotFoundErr(resource, id string) *AppError {
	return &AppError{Code: ErrNotFound, Message: fmt.Sprintf("%s %s not found", resource, id)}
}

func NewConflictErr(message string) *AppError {
	return &AppError{Code: ErrConflict, Message: message}
}

func NewExternalErr(message string, cause error) *AppError {
	return &AppError{Code: ErrExternal, Message: message, Cause: cause}
}

func NewInternalErr(message string, cause error) *AppError {
	return &AppError{Code: ErrInternal, Message: message, Cause: cause}
}
