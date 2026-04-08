package handlers

import (
	"content-hub/infra/config"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ConfigHandler struct {
	loader *config.Loader
}

func NewConfigHandler(loader *config.Loader) *ConfigHandler {
	return &ConfigHandler{loader: loader}
}

func (h *ConfigHandler) Get(c *gin.Context) {
	cfg := h.loader.Get()
	if cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config not loaded"})
		return
	}
	redacted := cfg.Redacted()
	c.JSON(http.StatusOK, redacted)
}

func (h *ConfigHandler) Patch(c *gin.Context) {
	var patches map[string]any
	if err := c.ShouldBindJSON(&patches); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := h.loader.Get()
	if cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config not loaded"})
		return
	}

	updated := *cfg

	if llm, ok := patches["llm"].(map[string]any); ok {
		if provider, ok := llm["provider"].(string); ok {
			updated.LLM.Provider = provider
		}
		if model, ok := llm["model"].(string); ok {
			updated.LLM.Model = model
		}
		if apiKey, ok := llm["api_key"].(string); ok {
			updated.LLM.APIKey = apiKey
		}
		if baseURL, ok := llm["base_url"].(string); ok {
			updated.LLM.BaseURL = baseURL
		}
		if temp, ok := llm["temperature"].(float64); ok {
			updated.LLM.Temperature = temp
		}
		if maxTokens, ok := llm["max_tokens"].(float64); ok {
			updated.LLM.MaxTokens = int(maxTokens)
		}
	}

	if workflow, ok := patches["workflow"].(map[string]any); ok {
		if maxJobs, ok := workflow["max_concurrent_jobs"].(float64); ok {
			updated.Workflow.MaxConcurrentJobs = int(maxJobs)
		}
		if retryAttempts, ok := workflow["retry_max_attempts"].(float64); ok {
			updated.Workflow.RetryMaxAttempts = int(retryAttempts)
		}
		if timeout, ok := workflow["timeout_sec"].(float64); ok {
			updated.Workflow.TimeoutSec = int(timeout)
		}
	}

	if err := h.loader.Save(updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated.Redacted())
}

func (h *ConfigHandler) MarshalJSON() ([]byte, error) {
	cfg := h.loader.Get()
	if cfg == nil {
		return json.Marshal(map[string]string{"error": "config not loaded"})
	}
	return json.Marshal(cfg.Redacted())
}
