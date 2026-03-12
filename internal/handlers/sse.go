package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"iflow2api-go/internal/config"
	"iflow2api-go/internal/proxy"
)

// ChatCompletionsStream handles streaming chat completion requests.
func ChatCompletionsStream(cfg *config.Config, request map[string]interface{}, w http.ResponseWriter) error {
	proxyClient := proxy.NewProxy(cfg.APIKey.BaseURL, cfg.APIKey.APIKey)

	req, err := http.NewRequest("POST", cfg.APIKey.BaseURL+"/chat/completions", nil)
	if err != nil {
		return err
	}

	req.Header = proxyClient.GenerateHeaders()

	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req.Body = io.NopCloser(strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		sendStreamError(w, fmt.Sprintf("API request failed: %s", string(bodyBytes)))
		return nil
	}

	if !isSSEStream(resp.Header.Get("Content-Type")) {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		errorMsg := extractErrorMessage(bodyStr)
		sendStreamError(w, errorMsg)
		return nil
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			fmt.Fprint(w, "\n")
			flusher.Flush()
			continue
		}

		if strings.HasPrefix(line, "data:") {
			dataStr := strings.TrimSpace(line[5:])
			if dataStr == "[DONE]" {
				fmt.Fprint(w, "data: [DONE]\n\n")
				flusher.Flush()
				continue
			}

			var chunkData map[string]interface{}
			if err := json.Unmarshal([]byte(dataStr), &chunkData); err == nil {
				normalized := proxy.NormalizeStreamChunk(chunkData, false)
				fmt.Fprintf(w, "data: %s\n\n", toJSON(normalized))
				flusher.Flush()
			} else {
				fmt.Fprint(w, line+"\n")
				flusher.Flush()
			}
		} else {
			fmt.Fprint(w, line+"\n")
			flusher.Flush()
		}
	}

	return scanner.Err()
}

func isSSEStream(contentType string) bool {
	return strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "application/octet-stream")
}

func extractErrorMessage(body string) string {
	var errorData map[string]interface{}
	if err := json.Unmarshal([]byte(body), &errorData); err == nil {
		if msg, ok := errorData["msg"].(string); ok {
			return msg
		}
		if errObj, ok := errorData["error"].(map[string]interface{}); ok {
			if msg, ok := errObj["message"].(string); ok {
				return msg
			}
		}
	}

	if len(body) > 200 {
		return body[:200]
	}
	return body
}

func sendStreamError(w http.ResponseWriter, message string) {
	errorChunk := map[string]interface{}{
		"id":      "error",
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   "unknown",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"content": fmt.Sprintf("[API Error] %s", message),
				},
				"finish_reason": "stop",
			},
		},
	}

	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", toJSON(errorChunk))
}

func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
