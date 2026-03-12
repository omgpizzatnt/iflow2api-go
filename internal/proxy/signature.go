// Package proxy provides API proxy functionality with iFlow-specific headers and HMAC signatures.
package proxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// UserAgent is the iFlow CLI user agent that unlocks premium models.
	UserAgent = "iFlow-Cli"

	// Version is the iFlow CLI version to mimic.
	Version = "0.5.13"
)

// SignatureData contains data needed for HMAC signature generation.
type SignatureData struct {
	UserAgent string
	SessionID string
	Timestamp int64
	APIKey    string
}

// GenerateSignature generates HMAC-SHA256 signature for iFlow API authentication.
// Signature format: HMAC-SHA256(apiKey, user_agent:session_id:timestamp)
func GenerateSignature(data SignatureData) (string, error) {
	if data.APIKey == "" {
		return "", fmt.Errorf("API key is required for signature generation")
	}

	message := fmt.Sprintf("%s:%s:%d", data.UserAgent, data.SessionID, data.Timestamp)

	mac := hmac.New(sha256.New, []byte(data.APIKey))
	mac.Write([]byte(message))

	return hex.EncodeToString(mac.Sum(nil)), nil
}

// Headers contains iFlow API headers.
type Headers struct {
	ContentType    string
	Authorization  string
	UserAgent      string
	SessionID      string
	ConversationID string
	Signature      string
	Timestamp      string
	TraceParent    string
	Accept         string
	AcceptLanguage string
	SecFetchMode   string
	AcceptEncoding string
}

// GenerateHeaders generates all required headers for iFlow API requests.
func GenerateHeaders(apiKey string) (*Headers, error) {
	sessionID := fmt.Sprintf("session-%s", uuid.New().String())
	conversationID := uuid.New().String()
	timestamp := time.Now().UnixMilli()

	sigData := SignatureData{
		UserAgent: UserAgent,
		SessionID: sessionID,
		Timestamp: timestamp,
		APIKey:    apiKey,
	}

	signature, err := GenerateSignature(sigData)
	if err != nil {
		return nil, err
	}

	return &Headers{
		ContentType:    "application/json",
		Authorization:  fmt.Sprintf("Bearer %s", apiKey),
		UserAgent:      UserAgent,
		SessionID:      sessionID,
		ConversationID: conversationID,
		Signature:      signature,
		Timestamp:      fmt.Sprintf("%d", timestamp),
		TraceParent:    generateTraceParent(),
		Accept:         "*/*",
		AcceptLanguage: "*",
		SecFetchMode:   "cors",
		AcceptEncoding: "br, gzip, deflate",
	}, nil
}

// generateTraceParent generates a W3C trace context traceparent.
// Format: 00-<32hex trace_id>-<16hex parent_id>-01
func generateTraceParent() string {
	traceID := hex.EncodeToString([]byte(uuid.New().String()))
	trim(traceID, 32)

	parentID := hex.EncodeToString([]byte(uuid.New().String()))[:16]
	trim(parentID, 16)

	return fmt.Sprintf("00-%s-%s-01", traceID, parentID)
}

// ToMap converts Headers to a map for use with HTTP requests.
func (h *Headers) ToMap() map[string]string {
	return map[string]string{
		"Content-Type":      h.ContentType,
		"Authorization":     h.Authorization,
		"user-agent":        h.UserAgent,
		"session-id":        h.SessionID,
		"conversation-id":   h.ConversationID,
		"accept":            h.Accept,
		"accept-language":   h.AcceptLanguage,
		"sec-fetch-mode":    h.SecFetchMode,
		"accept-encoding":   h.AcceptEncoding,
		"x-iflow-signature": h.Signature,
		"x-iflow-timestamp": h.Timestamp,
		"traceparent":       h.TraceParent,
	}
}

// Helper function to trim string to length
func trim(s string, length int) string {
	if len(s) > length {
		return s[:length]
	}
	return s
}
