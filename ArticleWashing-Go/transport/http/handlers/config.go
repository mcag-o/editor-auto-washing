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
	if isZeroConfig(cfg) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config not loaded"})
		return
	}
	redacted := cfg.Redacted()
	c.JSON(http.StatusOK, redacted)
}

func (h *ConfigHandler) MarshalJSON() ([]byte, error) {
	cfg := h.loader.Get()
	if isZeroConfig(cfg) {
		return json.Marshal(map[string]string{"error": "config not loaded"})
	}
	return json.Marshal(cfg.Redacted())
}

func isZeroConfig(cfg config.Config) bool {
	return cfg.HTTP.Host == "" && cfg.HTTP.Port == 0
}
