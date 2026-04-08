package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewContentDocument_Valid(t *testing.T) {
	doc, err := NewContentDocument("Test Title", "Body", "markdown")
	assert.NoError(t, err)
	assert.Equal(t, "Test Title", doc.Title)
	assert.Equal(t, "Body", doc.Body)
	assert.Equal(t, "markdown", doc.Format)
	assert.NotEmpty(t, doc.ID)
}

func TestNewContentDocument_EmptyTitle(t *testing.T) {
	_, err := NewContentDocument("", "Body", "markdown")
	assert.Error(t, err)
	assert.Equal(t, ErrValidation, err.(*AppError).Code)
}

func TestNewContentDocument_DefaultFormat(t *testing.T) {
	doc, err := NewContentDocument("Title", "Body", "")
	assert.NoError(t, err)
	assert.Equal(t, "markdown", doc.Format)
}
