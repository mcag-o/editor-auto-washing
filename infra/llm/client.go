package llm

import "context"

type GenerateRequest struct {
	SystemPrompt string
	UserPrompt   string
	Model        string
	TimeoutMS    int
	Metadata     map[string]any
}

type GenerateResponse struct {
	Raw   []byte
	Model string
}

type Client interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}

type StaticClient struct {
	Response []byte
	Model    string
	Err      error
}

func (c StaticClient) Generate(_ context.Context, _ GenerateRequest) (*GenerateResponse, error) {
	if c.Err != nil {
		return nil, c.Err
	}

	return &GenerateResponse{
		Raw:   append([]byte(nil), c.Response...),
		Model: c.Model,
	}, nil
}
