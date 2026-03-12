package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"iflow2api-go/internal/config"
	"iflow2api-go/internal/logger"
	"iflow2api-go/internal/proxy"
	"iflow2api-go/internal/vision"

	"github.com/google/uuid"
)

const (
	defaultIFlowModel = "glm-5"
)

type AnthropicMessage struct {
	Model string `json:"model"`
}

type AnthropicContent struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	Source   *AnthropicSource `json:"source,omitempty"`
	Input    interface{}      `json:"input,omitempty"`
	Thinking string           `json:"thinking,omitempty"`
}

type AnthropicSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type AnthropictUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Messages returns an HTTP handler that processes Anthropic-compatible message requests.
func Messages(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.APIKey.APIKey == "" {
			sendAnthropicErrorResponse(w, http.StatusServiceUnavailable, "iFlow API key not configured", "iflow_not_configured")
			return
		}

		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			sendAnthropicErrorResponse(w, http.StatusBadRequest, "Invalid request body", "invalid_request_error")
			return
		}

		var req map[string]interface{}
		if err := json.Unmarshal(reqBody, &req); err != nil {
			sendAnthropicErrorResponse(w, http.StatusBadRequest, "Invalid JSON", "invalid_request_error")
			return
		}

		if _, ok := req["messages"]; !ok {
			sendAnthropicErrorResponse(w, http.StatusUnprocessableEntity, "Field 'messages' is required", "invalid_request_error")
			return
		}

		stream := false
		if s, ok := req["stream"].(bool); ok {
			stream = s
		}

		originalModel := "unknown"
		if m, ok := req["model"].(string); ok {
			originalModel = m
		} else {
			originalModel = defaultIFlowModel
		}

		hasImages := detectImagesInMessages(req["messages"])
		mappedModel := getMappedModel(originalModel, hasImages)

		openaiReq := anthropicToOpenaiRequest(req, mappedModel)

		if stream {
			streamAnthropicMessages(w, cfg, openaiReq, mappedModel)
		} else {
			sendAnthropicMessagesNonStream(w, cfg, openaiReq, mappedModel)
		}
	}
}

func detectImagesInMessages(messages interface{}) bool {
	msgList, ok := messages.([]interface{})
	if !ok {
		return false
	}

	for _, msg := range msgList {
		if msgMap, ok := msg.(map[string]interface{}); ok {
			content, _ := msgMap["content"]
			if images := vision.DetectImageContent(content); len(images) > 0 {
				return true
			}
		}
	}
	return false
}

func getMappedModel(anthropicModel string, _hasImages bool) string {
	knownIFlowModels := []string{
		"glm-4.6", "glm-4.7", "glm-5",
		"iFlow-ROME-30BA3B", "deepseek-v3.2-chat",
		"qwen3-coder-plus",
		"kimi-k2", "kimi-k2-thinking", "kimi-k2.5", "kimi-k2-0905",
		"minimax-m2.5",
		"qwen-vl-max",
		"glm-4v", "glm-4v-plus", "glm-4v-flash", "glm-4.5v", "glm-4.6v",
		"qwen-vl-plus", "qwen2.5-vl", "qwen3-vl",
	}

	anthropicModelLower := strings.ToLower(anthropicModel)
	for _, model := range knownIFlowModels {
		if strings.ToLower(model) == anthropicModelLower {
			return model
		}
	}

	return defaultIFlowModel
}

func anthropicToOpenaiRequest(req map[string]interface{}, mappedModel string) map[string]interface{} {
	openaiReq := map[string]interface{}{
		"model": mappedModel,
	}

	if maxTokens, ok := req["max_tokens"].(float64); ok && maxTokens > 0 {
		openaiReq["max_tokens"] = int(maxTokens)
	}

	if temperature, ok := req["temperature"].(float64); ok && temperature > 0 {
		openaiReq["temperature"] = temperature
	}

	messages := []map[string]interface{}{}

	if sys, ok := req["system"]; ok && sys != nil {
		systemText := ""
		switch v := sys.(type) {
		case string:
			systemText = v
		case []interface{}:
			textParts := []string{}
			for _, block := range v {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if typ, ok := blockMap["type"].(string); ok && typ == "text" {
						if text, ok := blockMap["text"].(string); ok {
							textParts = append(textParts, text)
						}
					}
				}
			}
			systemText = strings.Join(textParts, " ")
		}

		if systemText != "" {
			messages = append(messages, map[string]interface{}{
				"role":    "system",
				"content": systemText,
			})
		}
	}

	if msgs, ok := req["messages"].([]interface{}); ok {
		for _, msg := range msgs {
			role := "user"
			if msgMap, ok := msg.(map[string]interface{}); ok {
				if r, ok := msgMap["role"].(string); ok {
					role = mapRole(r)
				}
			} else {
				role = "user"
			}

			content := getMsgContent(msg)
			processedMsg := processMessageContent(content, role)
			messages = append(messages, processedMsg)
		}
	}

	openaiReq["messages"] = messages

	return openaiReq
}

func mapRole(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	case "system":
		return "system"
	case "tool":
		return "tool"
	default:
		return "user"
	}
}

func getMsgContent(msg interface{}) interface{} {
	if msgMap, ok := msg.(map[string]interface{}); ok {
		return msgMap["content"]
	}
	return ""
}

func processMessageContent(content interface{}, role string) map[string]interface{} {
	var textParts []string
	var toolCalls []map[string]interface{}
	var toolResults []map[string]interface{}

	switch c := content.(type) {
	case string:
		textParts = append(textParts, c)
	case []interface{}:
		for _, block := range c {
			if blockStr, ok := block.(string); ok {
				textParts = append(textParts, blockStr)
			} else if blockMap, ok := block.(map[string]interface{}); ok {
				blockType, _ := blockMap["type"].(string)

				if blockType == "text" {
					if text, ok := blockMap["text"].(string); ok {
						textParts = append(textParts, text)
					}
				} else if blockType == "image" || blockType == "image_url" {
					for _, imgData := range vision.DetectImageContent(block) {
						if imgData.IsURL {
							textParts = append(textParts, fmt.Sprintf("[Image: %s]", imgData.Data))
						} else {
							textParts = append(textParts, fmt.Sprintf("[Base64 Image: %d bytes]", len(imgData.Data)))
						}
					}
				} else if blockType == "tool_use" {
					toolID := ""
					if id, ok := blockMap["id"].(string); ok && id != "" {
						toolID = id
					} else {
						toolID = fmt.Sprintf("call_%s", uuid.New().String()[:24])
					}

					name := ""
					if n, ok := blockMap["name"].(string); ok {
						name = n
					}

					input := blockMap["input"]

					inputJSON, _ := json.Marshal(input)

					toolCalls = append(toolCalls, map[string]interface{}{
						"id":   toolID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      name,
							"arguments": string(inputJSON),
						},
					})
				} else if blockType == "tool_result" {
					toolResultID := ""
					if id, ok := blockMap["tool_use_id"].(string); ok && id != "" {
						toolResultID = id
					}

					var contentStr string
					switch cr := blockMap["content"].(type) {
					case string:
						contentStr = cr
					case []interface{}:
						var parts []string
						for _, part := range cr {
							if partMap, ok := part.(map[string]interface{}); ok {
								if typ, ok := partMap["type"].(string); ok && typ == "text" {
									if text, ok := partMap["text"].(string); ok {
										parts = append(parts, text)
									}
								}
							}
						}
						contentStr = strings.Join(parts, "\n")
					}

					toolResults = append(toolResults, map[string]interface{}{
						"tool_call_id": toolResultID,
						"content":      contentStr,
					})
				}
			}
		}
	}

	if role == "assistant" && len(toolCalls) > 0 {
		return map[string]interface{}{
			"role":       "assistant",
			"content":    strings.Join(textParts, "\n"),
			"tool_calls": toolCalls,
		}
	}

	if role == "user" && len(toolResults) > 0 {
		return map[string]interface{}{
			"role":    "user",
			"content": strings.Join(textParts, "\n"),
		}
	}

	return map[string]interface{}{
		"role":    role,
		"content": strings.Join(textParts, "\n"),
	}
}

// sendAnthropicErrorResponse sends an Anthropic-formatted error response
func sendAnthropicErrorResponse(w http.ResponseWriter, status int, message string, errorType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"type": errorType,
		"error": map[string]interface{}{
			"type":    errorType,
			"message": message,
		},
	}); err != nil {
		logger.Errorf("Failed to encode error response: %v", err)
	}
}

// openaiToAnthropicResponse converts OpenAI response format to Anthropic format
func openaiToAnthropicResponse(openaiResp map[string]interface{}, model string) map[string]interface{} {
	contentBlocks := []map[string]interface{}{}
	finishReason := "end_turn"

	choices, _ := openaiResp["choices"].([]interface{})
	if len(choices) > 0 {
		choice := choices[0].(map[string]interface{})
		message := choice["message"].(map[string]interface{})

		contentText := ""
		if ct, ok := message["content"].(string); ok {
			contentText = ct
		} else if rc, ok := message["reasoning_content"].(string); ok {
			contentText = rc
		}

		if contentText != "" {
			contentBlocks = append(contentBlocks, map[string]interface{}{
				"type": "text",
				"text": contentText,
			})
		}

		if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
			for _, tc := range toolCalls {
				tcMap := tc.(map[string]interface{})
				funcMap := tcMap["function"].(map[string]interface{})
				var toolInput map[string]interface{}
				if args, ok := funcMap["arguments"].(string); ok {
					json.Unmarshal([]byte(args), &toolInput)
				}

				contentBlocks = append(contentBlocks, map[string]interface{}{
					"type":  "tool_use",
					"id":    tcMap["id"].(string),
					"name":  funcMap["name"].(string),
					"input": toolInput,
				})
			}
		}

		if fr, ok := choice["finish_reason"].(string); ok {
			switch fr {
			case "stop":
				finishReason = "end_turn"
			case "length":
				finishReason = "max_tokens"
			case "tool_calls":
				finishReason = "tool_use"
			}
		}
	}

	usage := openaiResp["usage"].(map[string]interface{})
	inputTokens := 0
	if pt, ok := usage["prompt_tokens"].(float64); ok {
		inputTokens = int(pt)
	}
	outputTokens := 0
	if ct, ok := usage["completion_tokens"].(float64); ok {
		outputTokens = int(ct)
	}

	return map[string]interface{}{
		"id":            fmt.Sprintf("msg_%s", uuid.New().String()[:24]),
		"type":          "message",
		"role":          "assistant",
		"content":       contentBlocks,
		"model":         model,
		"stop_reason":   finishReason,
		"stop_sequence": nil,
		"usage": map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}
}

// sendAnthropicMessagesNonStream handles non-streaming /v1/messages requests
func sendAnthropicMessagesNonStream(w http.ResponseWriter, cfg *config.Config, openaiReq map[string]interface{}, model string) {
	proxyClient := proxy.NewProxy(cfg.APIKey.BaseURL, cfg.APIKey.APIKey)
	req, err := http.NewRequest("POST", cfg.APIKey.BaseURL+"/chat/completions", nil)
	if err != nil {
		sendAnthropicErrorResponse(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}

	req.Header = proxyClient.GenerateHeaders()

	body, err := json.Marshal(openaiReq)
	if err != nil {
		sendAnthropicErrorResponse(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(strings.NewReader(string(body)))

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		sendAnthropicErrorResponse(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		sendAnthropicErrorResponse(w, resp.StatusCode, string(bodyBytes), "api_error")
		return
	}

	var openaiResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		sendAnthropicErrorResponse(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}

	anthropicResp := openaiToAnthropicResponse(openaiResp, model)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(anthropicResp); err != nil {
		logger.Errorf("Failed to encode Anthropic response: %v", err)
	}
}

// streamAnthropicMessages handles streaming /v1/messages requests
func streamAnthropicMessages(w http.ResponseWriter, cfg *config.Config, openaiReq map[string]interface{}, model string) {
	proxyClient := proxy.NewProxy(cfg.APIKey.BaseURL, cfg.APIKey.APIKey)
	req, err := http.NewRequest("POST", cfg.APIKey.BaseURL+"/chat/completions", nil)
	if err != nil {
		sendAnthropicErrorResponse(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}

	req.Header = proxyClient.GenerateHeaders()

	body, err := json.Marshal(openaiReq)
	if err != nil {
		sendAnthropicErrorResponse(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(strings.NewReader(string(body)))

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		sendAnthropicErrorResponse(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		sendAnthropicErrorResponse(w, resp.StatusCode, string(bodyBytes), "api_error")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		sendAnthropicErrorResponse(w, http.StatusInternalServerError, "Streaming not supported", "api_error")
		return
	}

	fmt.Fprint(w, createAnthropicMessageStart(model))
	flusher.Flush()

	fmt.Fprint(w, createAnthropicContentBlockStart(0, "text"))
	flusher.Flush()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			dataStr := strings.TrimSpace(line[5:])
			if dataStr == "[DONE]" {
				break
			}

			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(dataStr), &chunk); err == nil {
				delta := parseOpenAIChunk(chunk)
				if delta != "" {
					fmt.Fprint(w, createAnthropicContentBlockDelta(delta))
					flusher.Flush()
				}
			}
		}
	}

	fmt.Fprint(w, createAnthropicContentBlockStop(0))
	flusher.Flush()

	fmt.Fprint(w, createAnthropicMessageDelta("end_turn", 0))
	flusher.Flush()

	fmt.Fprint(w, createAnthropicMessageStop())
	flusher.Flush()
}

// createAnthropicMessageStart creates a message_start SSE event
func createAnthropicMessageStart(model string) string {
	data := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            fmt.Sprintf("msg_%s", uuid.New().String()[:24]),
			"type":          "message",
			"role":          "assistant",
			"content":       []interface{}{},
			"model":         model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]interface{}{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	}
	return fmt.Sprintf("event: message_start\ndata: %s\n\n", toJSON(data))
}

// createAnthropicContentBlockStart creates a content_block_start SSE event
func createAnthropicContentBlockStart(index int, blockType string) string {
	contentBlock := map[string]interface{}{"type": blockType}
	if blockType == "text" {
		contentBlock["text"] = ""
	}
	data := map[string]interface{}{
		"type":          "content_block_start",
		"index":         index,
		"content_block": contentBlock,
	}
	return fmt.Sprintf("event: content_block_start\ndata: %s\n\n", toJSON(data))
}

// createAnthropicContentBlockDelta creates a content_block_delta SSE event
func createAnthropicContentBlockDelta(text string) string {
	data := map[string]interface{}{
		"type":  "content_block_delta",
		"index": 0,
		"delta": map[string]interface{}{
			"type": "text_delta",
			"text": text,
		},
	}
	return fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", toJSON(data))
}

// createAnthropicContentBlockStop creates a content_block_stop SSE event
func createAnthropicContentBlockStop(index int) string {
	data := map[string]interface{}{
		"type":  "content_block_stop",
		"index": index,
	}
	return fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", toJSON(data))
}

// createAnthropicMessageDelta creates a message_delta SSE event
func createAnthropicMessageDelta(stopReason string, outputTokens int) string {
	data := map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]interface{}{
			"output_tokens": outputTokens,
		},
	}
	return fmt.Sprintf("event: message_delta\ndata: %s\n\n", toJSON(data))
}

// createAnthropicMessageStop creates a message_stop SSE event
func createAnthropicMessageStop() string {
	data := map[string]interface{}{
		"type": "message_stop",
	}
	return fmt.Sprintf("event: message_stop\ndata: %s\n\n", toJSON(data))
}

// parseOpenAIChunk extracts content from OpenAI SSE chunk
func parseOpenAIChunk(chunk map[string]interface{}) string {
	choices, ok := chunk["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return ""
	}

	choice := choices[0].(map[string]interface{})
	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return ""
	}

	if content, ok := delta["content"].(string); ok {
		return content
	}
	if rc, ok := delta["reasoning_content"].(string); ok {
		return rc
	}

	return ""
}
