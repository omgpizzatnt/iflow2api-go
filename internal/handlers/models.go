package handlers

import (
	"encoding/json"
	"net/http"

	"iflow2api-go/internal/config"
	"iflow2api-go/internal/logger"
	"iflow2api-go/internal/proxy"
)

type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// Models returns an HTTP handler that serves the list of available AI models.
func Models(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.APIKey.APIKey == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"message": "iFlow API key not configured",
					"type":    "api_error",
					"code":    "503",
				},
			}); err != nil {
				logger.Errorf("Failed to encode models error response: %v", err)
			}
			return
		}

		availableModels := proxy.GetAvailableModels()
		models := make([]Model, len(availableModels))
		for i, model := range availableModels {
			models[i] = Model{
				ID:      model.ID,
				Object:  "model",
				Created: 0,
				OwnedBy: "iflow",
			}
		}

		resp := ModelsResponse{
			Object: "list",
			Data:   models,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			logger.Errorf("Failed to encode models response: %v", err)
		}
	}
}
