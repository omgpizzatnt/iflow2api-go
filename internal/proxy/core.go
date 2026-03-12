package proxy

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"iflow2api-go/internal/logger"
)

const (
	IFLOW_CLI_USER_AGENT  = "iFlow-Cli"
	IFLOW_CLI_VERSION     = "0.5.13"
	NODE_VERSION_EMULATED = "v22.22.0"

	MMSTAT_GM_BASE    = "https://gm.mmstat.com"
	MMSTAT_VGIF_URL   = "https://log.mmstat.com/v.gif"
	TELEMETRY_TIMEOUT = 10 * time.Second
)

type Proxy struct {
	baseURL         string
	apiKey          string
	sessionID       string
	conversationID  string
	telemetryUserID string
	EnableTelemetry bool
}

func NewProxy(baseURL, apiKey string) *Proxy {
	sessionID := fmt.Sprintf("session-%s", uuid.New().String())
	conversationID := uuid.New().String()

	telemetryUserID := uuid.NewMD5(
		uuid.NameSpaceDNS,
		[]byte(apiKey),
	).String()

	return &Proxy{
		baseURL:         baseURL,
		apiKey:          apiKey,
		sessionID:       sessionID,
		conversationID:  conversationID,
		telemetryUserID: telemetryUserID,
		EnableTelemetry: true,
	}
}

func (p *Proxy) GenerateHeaders() http.Header {
	timestamp := time.Now().UnixMilli()

	signature := p.generateSignature(timestamp)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))
	headers.Set("user-agent", IFLOW_CLI_USER_AGENT)
	headers.Set("session-id", p.sessionID)
	headers.Set("conversation-id", p.conversationID)
	headers.Set("accept", "*/*")
	headers.Set("accept-language", "*")
	headers.Set("sec-fetch-mode", "cors")
	headers.Set("accept-encoding", "br, gzip, deflate")

	if signature != "" {
		headers.Set("x-iflow-signature", signature)
		headers.Set("x-iflow-timestamp", strconv.FormatInt(timestamp, 10))
	}

	headers.Set("traceparent", p.generateTraceParent())

	return headers
}

func (p *Proxy) generateSignature(timestamp int64) string {
	if p.apiKey == "" {
		return ""
	}

	message := fmt.Sprintf("%s:%s:%d", IFLOW_CLI_USER_AGENT, p.sessionID, timestamp)

	h := hmac.New(sha256.New, []byte(p.apiKey))
	h.Write([]byte(message))

	return hex.EncodeToString(h.Sum(nil))
}

func (p *Proxy) generateTraceParent() string {
	traceID := uuid.New().String()
	parentID := uuid.New().String()[0:16]
	return fmt.Sprintf("00-%s-%s-01", traceID, parentID)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *Proxy) MakeRequest(client *http.Client, request interface{}) (map[string]interface{}, error) {
	if client == nil {
		client = &http.Client{
			Timeout: 120 * time.Second,
		}
	}

	fullURL := p.baseURL + "/chat/completions"
	logger.Printf("Requesting: %s", fullURL)

	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header = p.GenerateHeaders()
	req.Body = io.NopCloser(bytes.NewReader(reqBody))

	authPreview := req.Header.Get("Authorization")
	if len(authPreview) > 20 {
		authPreview = authPreview[:20] + "..."
	}

	sigPreview := req.Header.Get("x-iflow-signature")
	if len(sigPreview) > 20 {
		sigPreview = sigPreview[:20] + "..."
	}

	logger.Printf("Headers: auth=%s, signature=%s", authPreview, sigPreview)

	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("HTTP request error: %v", err)
		return nil, fmt.Errorf("make request: %w", err)
	}
	defer resp.Body.Close()

	logger.Printf("Response: status=%d, encoding=%s", resp.StatusCode, resp.Header.Get("Content-Encoding"))

	var reader io.Reader = resp.Body

	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		greader, err := gzip.NewReader(resp.Body)
		if err != nil {
			bodyBytes, _ := io.ReadAll(resp.Body)
			logger.Errorf("Gzip decompress error: %v", err)
			return nil, fmt.Errorf("decompress gzip: %w, body: %s", err, string(bodyBytes))
		}
		defer greader.Close()
		reader = greader
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(reader)
		logger.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("API error: status=%d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(reader).Decode(&result); err != nil {
		logger.Errorf("JSON decode error: %v", err)
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	logger.Printf("Request successful")

	if _, ok := result["usage"]; !ok {
		result["usage"] = map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		}
	}

	return NormalizeResponse(result, false), nil
}

func (p *Proxy) MakeRequestWithNormalization(client *http.Client, request interface{}, preserveReasoning bool) (map[string]interface{}, error) {
	result, err := p.MakeRequest(client, request)
	if err != nil {
		return nil, err
	}

	return NormalizeResponse(result, preserveReasoning), nil
}

func (p *Proxy) generateObservationID() string {
	h := md5.New()
	h.Write([]byte(uuid.New().String()))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (p *Proxy) ExtractTraceID(traceparent string) string {
	parts := strings.Split(traceparent, "-")
	if len(parts) == 4 && len(parts[1]) == 32 {
		return parts[1]
	}
	h := md5.New()
	h.Write([]byte(uuid.New().String()))
	return hex.EncodeToString(h.Sum(nil))
}

func (p *Proxy) telemetryPostGM(path, gokey string) {
	if !p.EnableTelemetry {
		return
	}

	client := &http.Client{Timeout: TELEMETRY_TIMEOUT}
	payload := map[string]string{
		"gmkey": "AI",
		"gokey": gokey,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", MMSTAT_GM_BASE+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("user-agent", "node")
	req.Header.Set("accept-encoding", "br, gzip, deflate")

	resp, err := client.Do(req)
	if err != nil {
		logger.Debugf("telemetry GM failed (%s): %v", path, err)
		return
	}
	defer resp.Body.Close()
}

func (p *Proxy) telemetryPostVGif() {
	if !p.EnableTelemetry {
		return
	}

	client := &http.Client{Timeout: TELEMETRY_TIMEOUT}

	os := runtime.GOOS
	if strings.HasPrefix(os, "win") {
		os = "win"
	}

	data := url.Values{}
	data.Set("logtype", "1")
	data.Set("title", "iFlow-CLI")
	data.Set("pre", "-")
	data.Set("platformType", "pc")
	data.Set("device_model", runtime.GOOS)
	data.Set("os", runtime.GOOS)
	data.Set("o", os)
	data.Set("node_version", NODE_VERSION_EMULATED)
	data.Set("language", "zh_CN.UTF-8")
	data.Set("interactive", "0")
	data.Set("iFlowEnv", "")
	data.Set("_g_encode", "utf-8")
	data.Set("pid", "iflow")
	data.Set("_user_id", p.telemetryUserID)

	req, _ := http.NewRequest("POST", MMSTAT_VGIF_URL, strings.NewReader(data.Encode()))
	req.Header.Set("content-type", "text/plain;charset=UTF-8")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("user-agent", "node")
	req.Header.Set("accept-encoding", "br, gzip, deflate")

	resp, err := client.Do(req)
	if err != nil {
		logger.Debugf("telemetry v.gif failed: %v", err)
		return
	}
	defer resp.Body.Close()
}

func (p *Proxy) EmitRunStarted(model, traceID string) string {
	observationID := p.generateObservationID()

	gokey := fmt.Sprintf(
		"pid=iflow"+
			"&sam=iflow.cli.%s.%s"+
			"&trace_id=%s"+
			"&session_id=%s"+
			"&conversation_id=%s"+
			"&observation_id=%s"+
			"&model=%s"+
			"&tool=%s"+
			"&user_id=%s",
		p.conversationID, traceID,
		traceID,
		p.sessionID,
		p.conversationID,
		observationID,
		url.QueryEscape(model),
		"",
		p.telemetryUserID,
	)

	p.telemetryPostGM("//aitrack.lifecycle.run_started", gokey)
	p.telemetryPostVGif()

	return observationID
}

func (p *Proxy) EmitRunError(model, traceID, parentObservationID, errorMsg string) {
	observationID := p.generateObservationID()

	gokey := fmt.Sprintf(
		"pid=iflow"+
			"&sam=iflow.cli.%s.%s"+
			"&trace_id=%s"+
			"&observation_id=%s"+
			"&parent_observation_id=%s"+
			"&session_id=%s"+
			"&conversation_id=%s"+
			"&user_id=%s"+
			"&error_msg=%s"+
			"&model=%s"+
			"&tool=%s"+
			"&toolName=%s"+
			"&toolArgs=%s"+
			"&cliVer=%s"+
			"&platform=%s"+
			"&arch=%s"+
			"&nodeVersion=%s"+
			"&osVersion=%s",
		p.conversationID, traceID,
		traceID,
		observationID,
		parentObservationID,
		p.sessionID,
		p.conversationID,
		p.telemetryUserID,
		url.QueryEscape(errorMsg),
		url.QueryEscape(model),
		"",
		"",
		"",
		IFLOW_CLI_VERSION,
		strings.ToLower(runtime.GOOS),
		strings.ToLower(runtime.GOARCH),
		url.QueryEscape(NODE_VERSION_EMULATED),
		runtime.GOOS+" "+runtime.GOARCH,
	)

	p.telemetryPostGM("//aitrack.lifecycle.run_error", gokey)
}

func (p *Proxy) ConfigureModelRequest(requestBody map[string]interface{}, model string) map[string]interface{} {
	body := make(map[string]interface{})
	for k, v := range requestBody {
		body[k] = v
	}

	modelLower := strings.ToLower(model)

	switch {
	case strings.HasPrefix(modelLower, "deepseek"):
		if _, ok := body["thinking_mode"]; !ok {
			body["thinking_mode"] = true
		}
		if _, ok := body["reasoning"]; !ok {
			body["reasoning"] = true
		}
		logger.Debugf("Model %s: added thinking_mode, reasoning", model)

	case model == "glm-5":
		if _, ok := body["chat_template_kwargs"]; !ok {
			body["chat_template_kwargs"] = map[string]interface{}{"enable_thinking": true}
		}
		if _, ok := body["enable_thinking"]; !ok {
			body["enable_thinking"] = true
		}
		if _, ok := body["thinking"]; !ok {
			body["thinking"] = map[string]interface{}{"type": "enabled"}
		}
		logger.Debugf("Model %s: added chat_template_kwargs, enable_thinking, thinking", model)

	case model == "glm-4.7":
		if _, ok := body["chat_template_kwargs"]; !ok {
			body["chat_template_kwargs"] = map[string]interface{}{"enable_thinking": true}
		}
		logger.Debugf("Model %s: added chat_template_kwargs", model)

	case strings.HasPrefix(modelLower, "glm-"):
		if _, ok := body["chat_template_kwargs"]; !ok {
			body["chat_template_kwargs"] = map[string]interface{}{"enable_thinking": true}
		}
		logger.Debugf("Model %s: added chat_template_kwargs", model)

	case strings.HasPrefix(modelLower, "kimi-k2.5"):
		if _, ok := body["thinking"]; !ok {
			body["thinking"] = map[string]interface{}{"type": "enabled"}
		}
		logger.Debugf("Model %s: added thinking", model)

	case strings.Contains(modelLower, "thinking"):
		if _, ok := body["thinking_mode"]; !ok {
			body["thinking_mode"] = true
		}
		logger.Debugf("Model %s: added thinking_mode", model)

	case strings.HasPrefix(modelLower, "mimo-"):
		if _, ok := body["thinking"]; !ok {
			body["thinking"] = map[string]interface{}{"type": "enabled"}
		}
		logger.Debugf("Model %s: added thinking", model)

	case strings.Contains(modelLower, "claude"):
		if _, ok := body["chat_template_kwargs"]; !ok {
			body["chat_template_kwargs"] = map[string]interface{}{"enable_thinking": true}
		}
		logger.Debugf("Model %s: added chat_template_kwargs", model)

	case strings.Contains(modelLower, "sonnet-"):
		if _, ok := body["chat_template_kwargs"]; !ok {
			body["chat_template_kwargs"] = map[string]interface{}{"enable_thinking": true}
		}
		logger.Debugf("Model %s: added chat_template_kwargs", model)

	case strings.Contains(modelLower, "reasoning"):
		if _, ok := body["reasoning"]; !ok {
			body["reasoning"] = true
		}
		logger.Debugf("Model %s: added reasoning", model)
	}

	match, _ := regexp.MatchString(`(?i)^qwen.*4b$`, model)
	if match {
		for _, key := range []string{"thinking_mode", "reasoning", "chat_template_kwargs"} {
			delete(body, key)
		}
		logger.Debugf("Model %s: removed thinking parameters (not supported)", model)
	}

	return body
}

func (p *Proxy) AlignOfficialBodyDefaults(requestBody map[string]interface{}, stream bool) map[string]interface{} {
	body := make(map[string]interface{})
	for k, v := range requestBody {
		body[k] = v
	}

	delete(body, "stream")
	if stream {
		body["stream"] = true
	}

	if _, ok := body["temperature"]; !ok {
		body["temperature"] = 0.7
	}
	if _, ok := body["top_p"]; !ok {
		body["top_p"] = 0.95
	}
	if _, ok := body["max_new_tokens"]; !ok {
		body["max_new_tokens"] = 8192
	}
	if _, ok := body["tools"]; !ok {
		body["tools"] = []interface{}{}
	}

	return body
}
