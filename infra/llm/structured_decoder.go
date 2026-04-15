package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func DecodeJSONMap(data []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("expected JSON object")
	}

	var out map[string]any
	if err := json.Unmarshal(trimmed, &out); err != nil {
		return nil, fmt.Errorf("decode JSON object: %w", err)
	}

	if out == nil {
		return nil, fmt.Errorf("expected JSON object")
	}

	return out, nil
}
