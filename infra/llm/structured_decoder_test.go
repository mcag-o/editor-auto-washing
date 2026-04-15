package llm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStructuredDecoderParsesJSONObject(t *testing.T) {
	out, err := DecodeJSONMap([]byte(`{"headline":"hello"}`))
	require.NoError(t, err)
	require.Equal(t, "hello", out["headline"])
}

func TestStructuredDecoderReturnsErrorForJSONArray(t *testing.T) {
	_, err := DecodeJSONMap([]byte(`["headline"]`))
	require.Error(t, err)
	require.ErrorContains(t, err, "JSON object")
}

func TestStructuredDecoderReturnsErrorForInvalidJSON(t *testing.T) {
	_, err := DecodeJSONMap([]byte(`{"headline":}`))
	require.Error(t, err)
}
