package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type APIClient struct {
	baseURL string
	client  *http.Client
}

func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *APIClient) Get(path string, result any) error {
	resp, err := c.client.Get(c.baseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s: %d %s", path, resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(result)
}

func (c *APIClient) Post(path string, body, result any) error {
	data, _ := json.Marshal(body)
	resp, err := c.client.Post(c.baseURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("POST %s: %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(result)
}
