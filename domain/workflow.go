package domain

type WorkflowDefinition struct {
	Name  string   `json:"name"`
	Nodes []string `json:"nodes"`
}

type WorkflowContext struct {
	Document     *ContentDocument
	ArtifactPath string
	Payload      map[string]any
	TraceID      string
	Command      string
}

func DefaultWorkflowDefinition() *WorkflowDefinition {
	return &WorkflowDefinition{
		Name:  "default",
		Nodes: []string{"automation_dispatch", "automation_snapshot"},
	}
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMOptions struct {
	Model       string
	Temperature float64
	MaxTokens   int
}

type LLMResponse struct {
	Content          string `json:"content"`
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	FinishReason     string `json:"finish_reason"`
}

type LLMGenerateRequest struct {
	Messages []ChatMessage
	Options  LLMOptions
	Metadata map[string]any
}

type LLMGenerateResponse struct {
	Response *LLMResponse
}
