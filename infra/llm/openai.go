package llm

import (
	"bufio"
	"bytes"
	"content-hub/domain"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Provider struct {
	baseURL    string
	apiKey     string
	model      string
	timeout    time.Duration
	httpClient *http.Client
}

func NewProvider(baseURL, apiKey, model string, timeout time.Duration) *Provider {
	return &Provider{
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		timeout:    timeout,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (p *Provider) Generate(ctx context.Context, genReq GenerateRequest) (*GenerateResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("LLM API key not configured")
	}

	reqBody := chatRequest{
		Model:       p.requestModel(genReq.Options),
		Messages:    genReq.Messages,
		Temperature: genReq.Options.Temperature,
		MaxTokens:   genReq.Options.MaxTokens,
		Stream:      false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LLM error %d: %s", resp.StatusCode, string(b))
	}

	var apiResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &GenerateResponse{
		Response: &domain.LLMResponse{
			Content:          apiResp.Choices[0].Message.Content,
			Model:            reqBody.Model,
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
			FinishReason:     apiResp.Choices[0].FinishReason,
		},
	}, nil
}

func (p *Provider) GenerateStream(ctx context.Context, genReq GenerateRequest, onChunk func(string) error) error {
	if p.apiKey == "" {
		return fmt.Errorf("LLM API key not configured")
	}

	reqBody := chatRequest{
		Model:       p.requestModel(genReq.Options),
		Messages:    genReq.Messages,
		Temperature: genReq.Options.Temperature,
		MaxTokens:   genReq.Options.MaxTokens,
		Stream:      true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("LLM stream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LLM stream error %d: %s", resp.StatusCode, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line == "data: [DONE]" {
			continue
		}

		const prefix = "data: "
		if len(line) < len(prefix) {
			continue
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(line[len(prefix):]), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if err := onChunk(chunk.Choices[0].Delta.Content); err != nil {
				return fmt.Errorf("stream callback error: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}

	return nil
}

func (p *Provider) BaseURL() string        { return p.baseURL }
func (p *Provider) Model() string          { return p.model }
func (p *Provider) Name() string           { return "openai-compatible" }
func (p *Provider) Timeout() time.Duration { return p.timeout }

func (p *Provider) requestModel(opts domain.LLMOptions) string {
	if opts.Model != "" {
		return opts.Model
	}

	return p.model
}

type chatRequest struct {
	Model       string               `json:"model"`
	Messages    []domain.ChatMessage `json:"messages"`
	Temperature float64              `json:"temperature"`
	MaxTokens   int                  `json:"max_tokens,omitempty"`
	Stream      bool                 `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}
