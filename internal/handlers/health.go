package handlers

import (
	"encoding/json"
	"net/http"
	"runtime"

	"iflow2api-go/internal/config"
	"iflow2api-go/internal/logger"
)

type HealthResponse struct {
	Status        string `json:"status"`
	IFlowLoggedIn bool   `json:"iflow_logged_in"`
	OS            string `json:"os"`
	Platform      string `json:"platform"`
}

// Health returns an HTTP handler that serves health check information.
func Health(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isLoggedIn := cfg.APIKey.APIKey != ""

		resp := HealthResponse{
			Status:        getStatus(isLoggedIn),
			IFlowLoggedIn: isLoggedIn,
			OS:            runtime.GOOS,
			Platform:      runtime.GOARCH,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			logger.Errorf("Failed to encode health response: %v", err)
		}
	}
}

func getStatus(isLoggedIn bool) string {
	if isLoggedIn {
		return "healthy"
	}
	return "degraded"
}
