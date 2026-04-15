package llm

import (
	"fmt"
	"regexp"
)

var promptPlaceholderPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)

func RenderPrompt(template string, vars map[string]any) (string, error) {
	missing := ""

	rendered := promptPlaceholderPattern.ReplaceAllStringFunc(template, func(match string) string {
		parts := promptPlaceholderPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}

		value, ok := vars[parts[1]]
		if !ok {
			if missing == "" {
				missing = parts[1]
			}
			return match
		}

		return fmt.Sprint(value)
	})

	if missing != "" {
		return "", fmt.Errorf("unresolved placeholder: %s", missing)
	}

	return rendered, nil
}
