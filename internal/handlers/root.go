// Package handlers provides HTTP handlers for the iFlow2API proxy server.
package handlers

import (
	"fmt"
	"net/http"

	"iflow2api-go/internal/config"
)

// Root returns an HTTP handler that serves the root endpoint with available API documentation.
func Root(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "iflow2api-go - iFlow API Proxy\n\n")
		fmt.Fprintf(w, "Available endpoints:\n")
		fmt.Fprintf(w, "  GET  /health           - Health check\n")
		fmt.Fprintf(w, "  GET  /v1/models        - List available models\n")
		fmt.Fprintf(w, "  POST /v1/chat/completions - Chat completions (OpenAI compatible)\n")
		fmt.Fprintf(w, "  POST /v1/messages      - Messages (Anthropic compatible)\n")
	}
}
