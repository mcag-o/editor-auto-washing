package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_HTTPStatus(t *testing.T) {
	assert.Equal(t, 400, NewValidationErr("test", nil).HTTPStatus())
	assert.Equal(t, 404, NewNotFoundErr("article", "123").HTTPStatus())
	assert.Equal(t, 409, NewConflictErr("conflict").HTTPStatus())
	assert.Equal(t, 502, NewExternalErr("timeout", nil).HTTPStatus())
	assert.Equal(t, 500, NewInternalErr("panic", nil).HTTPStatus())
}

func TestAppError_Error(t *testing.T) {
	err := NewValidationErr("title required", nil)
	assert.Equal(t, "[VALIDATION_ERROR] title required", err.Error())

	err = NewExternalErr("timeout", errors.New("context deadline exceeded"))
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestAppError_Unwrap(t *testing.T) {
	cause := errors.New("cause")
	err := NewInternalErr("msg", cause)
	assert.True(t, errors.Is(err, cause))
}
