package llm

import (
	"fmt"
	"regexp"
)

var promptPlaceholderPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)

func RenderPrompt(template string, vars map[string]any) (string, error) {
	rendered := promptPlaceholderPattern.ReplaceAllStringFunc(template, func(match string) string {
		parts := promptPlaceholderPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}

		value, ok := vars[parts[1]]
		if !ok {
			return match
		}

		return fmt.Sprint(value)
	})

	unresolved := promptPlaceholderPattern.FindStringSubmatch(rendered)
	if len(unresolved) == 2 {
		return "", fmt.Errorf("unresolved placeholder: %s", unresolved[1])
	}

	return rendered, nil
}
