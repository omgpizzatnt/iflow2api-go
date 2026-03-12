package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"iflow2api-go/internal/config"
	"iflow2api-go/internal/logger"
	"iflow2api-go/internal/proxy"
)

type ChatCompletionRequest struct {
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	Stream           bool      `json:"stream,omitempty"`
	Temperature      float64   `json:"temperature,omitempty"`
	MaxTokens        int       `json:"max_tokens,omitempty"`
	TopP             float64   `json:"top_p,omitempty"`
	FrequencyPenalty float64   `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64   `json:"presence_penalty,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletions returns an HTTP handler that processes OpenAI-compatible chat completion requests.
func ChatCompletions(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.APIKey.APIKey == "" {
			logger.Error("API key not configured")
			sendErrorResponse(w, http.StatusServiceUnavailable, "iFlow API key not configured", "api_error")
			return
		}

		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Errorf("Invalid request body: %v", err)
			sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", "invalid_request_error")
			return
		}

		if req.Model == "" {
			logger.Error("Model is required")
			sendErrorResponse(w, http.StatusBadRequest, "model is required", "invalid_request_error")
			return
		}

		if len(req.Messages) == 0 {
			logger.Error("Messages is required")
			sendErrorResponse(w, http.StatusBadRequest, "messages is required", "invalid_request_error")
			return
		}

		requestMap := map[string]interface{}{
			"model":    req.Model,
			"messages": req.Messages,
		}

		if req.Stream {
			requestMap["stream"] = true
		}
		if req.Temperature > 0 {
			requestMap["temperature"] = req.Temperature
		}
		if req.MaxTokens > 0 {
			requestMap["max_tokens"] = req.MaxTokens
		}
		if req.TopP > 0 {
			requestMap["top_p"] = req.TopP
		}

		logger.Printf("Request: model=%s, messages=%d, stream=%v", req.Model, len(req.Messages), req.Stream)

		if req.Stream {
			handleStreamingRequest(w, r, cfg, requestMap)
		} else {
			handleNonStreamingRequest(w, r, cfg, requestMap)
		}
	}
}

func handleNonStreamingRequest(w http.ResponseWriter, r *http.Request, cfg *config.Config, requestMap map[string]interface{}) {
	AcquireLock()
	defer ReleaseLock()

	proxyClient := proxy.NewProxy(cfg.APIKey.BaseURL, cfg.APIKey.APIKey)

	model := requestMap["model"].(string)
	requestMap = proxyClient.AlignOfficialBodyDefaults(requestMap, false)
	requestMap = proxyClient.ConfigureModelRequest(requestMap, model)

	traceparent := proxyClient.GenerateHeaders().Get("traceparent")
	traceID := proxyClient.ExtractTraceID(traceparent)

	var parentObservationID string
	if proxyClient.EnableTelemetry {
		parentObservationID = proxyClient.EmitRunStarted(model, traceID)
	}

	result, err := proxyClient.MakeRequest(nil, requestMap)
	if err != nil {
		logger.Errorf("Proxy error: %v", err)
		if parentObservationID != "" {
			proxyClient.EmitRunError(model, traceID, parentObservationID, err.Error())
		}
		sendErrorResponse(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}

	logger.Printf("Response: success")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		logger.Errorf("Failed to encode response: %v", err)
	}
}

func handleStreamingRequest(w http.ResponseWriter, r *http.Request, cfg *config.Config, requestMap map[string]interface{}) {
	AcquireLock()
	defer ReleaseLock()

	proxyClient := proxy.NewProxy(cfg.APIKey.BaseURL, cfg.APIKey.APIKey)

	model := requestMap["model"].(string)
	requestMap = proxyClient.AlignOfficialBodyDefaults(requestMap, true)
	requestMap = proxyClient.ConfigureModelRequest(requestMap, model)

	traceparent := proxyClient.GenerateHeaders().Get("traceparent")
	traceID := proxyClient.ExtractTraceID(traceparent)

	var parentObservationID string
	if proxyClient.EnableTelemetry {
		parentObservationID = proxyClient.EmitRunStarted(model, traceID)
	}

	var errMsg string
	defer func() {
		if errMsg != "" && parentObservationID != "" {
			proxyClient.EmitRunError(model, traceID, parentObservationID, errMsg)
		}
	}()

	client := &http.Client{
		Timeout: 300 * time.Second,
	}

	reqBody, err := json.Marshal(requestMap)
	if err != nil {
		errMsg = fmt.Sprintf("marshal request: %v", err)
		logger.Errorf(errMsg)
		sendStreamError(w, errMsg)
		return
	}

	fullURL := cfg.APIKey.BaseURL + "/chat/completions"
	req, err := http.NewRequest("POST", fullURL, bytes.NewReader(reqBody))
	if err != nil {
		errMsg = fmt.Sprintf("create request: %v", err)
		logger.Errorf(errMsg)
		sendStreamError(w, errMsg)
		return
	}

	req.Header = proxyClient.GenerateHeaders()
	req.Header.Set("stream", "true")

	resp, err := client.Do(req)
	if err != nil {
		errMsg = fmt.Sprintf("make request: %v", err)
		logger.Errorf(errMsg)
		sendStreamError(w, errMsg)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg = fmt.Sprintf("API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
		logger.Errorf(errMsg)
		sendStreamError(w, errMsg)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") && !strings.Contains(contentType, "application/octet-stream") {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg = extractNonStreamError(string(bodyBytes))
		logger.Errorf("Non-stream response: %s", errMsg)
		sendStreamError(w, errMsg)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		errMsg = "streaming not supported"
		logger.Errorf(errMsg)
		sendStreamError(w, errMsg)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	bufioBuf := make([]byte, 4096)
	scanner.Buffer(bufioBuf, 1048576)

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

	if err := scanner.Err(); err != nil {
		errMsg = fmt.Sprintf("scan error: %v", err)
		logger.Errorf(errMsg)
	}
}

func extractNonStreamError(body string) string {
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

func sendErrorResponse(w http.ResponseWriter, status int, message string, errorType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errorType,
			"code":    status,
		},
	}); err != nil {
		logger.Errorf("Failed to encode error response: %v", err)
	}
}
