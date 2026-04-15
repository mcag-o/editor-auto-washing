package llm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptRendererReplacesVariables(t *testing.T) {
	rendered, err := RenderPrompt("Title: {{title}}", map[string]any{"title": "Hello"})
	require.NoError(t, err)
	require.Equal(t, "Title: Hello", rendered)
}

func TestPromptRendererReturnsErrorForUnresolvedPlaceholder(t *testing.T) {
	_, err := RenderPrompt("Title: {{title}}\nBody: {{body}}", map[string]any{"title": "Hello"})
	require.Error(t, err)
	require.ErrorContains(t, err, "unresolved placeholder")
	require.ErrorContains(t, err, "body")
}

func TestPromptRendererStringifiesNonStringValues(t *testing.T) {
	rendered, err := RenderPrompt("Count: {{count}}", map[string]any{"count": 3})
	require.NoError(t, err)
	require.Equal(t, "Count: 3", rendered)
}
