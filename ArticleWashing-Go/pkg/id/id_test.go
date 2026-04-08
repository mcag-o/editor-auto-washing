package id

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	id := New()
	assert.NotEmpty(t, id)
	assert.Len(t, id, 36)
}
