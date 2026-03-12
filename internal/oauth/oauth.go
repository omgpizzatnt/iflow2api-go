// Package oauth provides OAuth authentication for iFlow.
package oauth

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// ClientID is the iFlow OAuth client ID.
	ClientID = "10009311001"
	// ClientSecret is the iFlow OAuth client secret.
	ClientSecret = "4Z3YjXycVsQvyGF1etiNlIBB4RsqSDtW"
	// TokenURL is the OAuth token endpoint.
	TokenURL = "https://iflow.cn/oauth/token"
	// AuthURL is the OAuth authorization endpoint.
	AuthURL = "https://iflow.cn/oauth"
	// UserInfoURL is the user info endpoint.
	UserInfoURL = "https://iflow.cn/api/oauth/getUserInfo"
	// DefaultRedirectURI is the default OAuth callback URL.
	DefaultRedirectURI = "http://localhost:19198/oauth2callback"
	// iFlowUserAgent is the user agent for iFlow API.
	iFlowUserAgent = "iFlow-Cli"
)

// OAuthClient handles OAuth authentication.
type OAuthClient struct {
	httpClient  HTTPClient
	server      *CallbackServer
	codeChannel chan OAuthResult
}

// HTTPClient interface for mocking in tests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// OAuthResult represents the result of OAuth flow.
type OAuthResult struct {
	Code  string
	Error string
	State string
}

// TokenData represents OAuth token response.
type TokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	Success      bool      `json:"success"`
	Message      string    `json:"message"`
}

// UserInfo represents user info from iFlow.
type UserInfo struct {
	Success bool  `json:"success"`
	Data    *User `json:"data"`
}

// User represents iFlow user information.
type User struct {
	Username string `json:"username"`
	Phone    string `json:"phone"`
	APIKey   string `json:"apiKey"`
}

// NewOAuthClient creates a new OAuth client.
func NewOAuthClient(httpClient HTTPClient) *OAuthClient {
	return &OAuthClient{
		httpClient:  httpClient,
		codeChannel: make(chan OAuthResult, 1),
	}
}

// GetAuthURL generates OAuth authorization URL.
func (o *OAuthClient) GetAuthURL(redirectURI string, state string) string {
	if state == "" {
		state = generateState()
	}
	if redirectURI == "" {
		redirectURI = DefaultRedirectURI
	}

	return fmt.Sprintf("%s?client_id=%s&loginMethod=phone&type=phone&redirect=%s&state=%s",
		AuthURL, ClientID, redirectURI, state)
}

// GetToken exchanges authorization code for access token.
func (o *OAuthClient) GetToken(code, redirectURI string) (*TokenData, error) {
	credentials := base64.StdEncoding.EncodeToString(
		[]byte(ClientID + ":" + ClientSecret),
	)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", ClientID)
	data.Set("client_secret", ClientSecret)

	req, err := http.NewRequest("POST", TokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+credentials)
	req.Header.Set("User-Agent", iFlowUserAgent)
	req.URL.RawQuery = data.Encode()

	client := o.httpClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth endpoint returned status: %d", resp.StatusCode)
	}

	var tokenData TokenData
	if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if tokenData.AccessToken == "" {
		return nil, fmt.Errorf("OAuth response missing access_token")
	}

	if tokenData.ExpiresIn > 0 {
		tokenData.ExpiresAt = time.Now().Add(time.Duration(tokenData.ExpiresIn) * time.Second)
	}

	return &tokenData, nil
}

// RefreshToken refreshes access token using refresh token.
func (o *OAuthClient) RefreshToken(refreshToken string) (*TokenData, error) {
	credentials := base64.StdEncoding.EncodeToString(
		[]byte(ClientID + ":" + ClientSecret),
	)

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", ClientID)
	data.Set("client_secret", ClientSecret)
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequest("POST", TokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+credentials)
	req.Header.Set("User-Agent", iFlowUserAgent)
	req.URL.RawQuery = data.Encode()

	client := o.httpClient
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false,
				},
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		var errorResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errorResp)
		if errorResp.Error == "invalid_grant" {
			return nil, fmt.Errorf("refresh_token无效或已过期")
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth endpoint returned status: %d", resp.StatusCode)
	}

	var tokenData TokenData
	if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !tokenData.Success && tokenData.Message != "" {
		if contains(tokenData.Message, "太多") || contains(tokenData.Message, "过载") {
			return nil, fmt.Errorf("服务器过载: %s", tokenData.Message)
		}
		return nil, fmt.Errorf("OAuth刷新失败: %s", tokenData.Message)
	}

	if tokenData.AccessToken == "" {
		return nil, fmt.Errorf("OAuth response missing access_token")
	}

	if tokenData.ExpiresIn > 0 {
		tokenData.ExpiresAt = time.Now().Add(time.Duration(tokenData.ExpiresIn) * time.Second)
	}

	return &tokenData, nil
}

// GetUserInfo retrieves user information using access token.
func (o *OAuthClient) GetUserInfo(accessToken string) (*User, error) {
	req, err := http.NewRequest("GET", UserInfoURL+"?accessToken="+accessToken, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", iFlowUserAgent)

	client := o.httpClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("access_token无效或已过期")
	}

	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !userInfo.Success || userInfo.Data == nil {
		return nil, fmt.Errorf("获取用户信息失败")
	}

	return userInfo.Data, nil
}

// StartCallbackServer starts the OAuth callback server.
func (o *OAuthClient) StartCallbackServer(port int) error {
	o.server = NewCallbackServer(port)
	return o.server.Start()
}

// StopCallbackServer stops the OAuth callback server.
func (o *OAuthClient) StopCallbackServer() {
	if o.server != nil {
		o.server.Stop()
	}
}

// WaitForCallback waits for OAuth callback or timeout.
func (o *OAuthClient) WaitForCallback(timeout time.Duration) (code, error, state string) {
	timeoutChan := time.After(timeout)

	select {
	case result := <-o.codeChannel:
		return result.Code, result.Error, result.State
	case <-timeoutChan:
		return "", "timeout", ""
	case <-o.server.Done():
		return "", "server_stopped", ""
	}
}

// handleCallback is called by the callback server when OAuth completes.
func (o *OAuthClient) handleCallback(code, error, state string) {
	select {
	case o.codeChannel <- OAuthResult{Code: code, Error: error, State: state}:
	default:
	}
}

// generateState generates a random CSRF state token.
func generateState() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr)
}

func GenerateState() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func (o *OAuthClient) ExtractCodeFromManualInput(input string) (string, error) {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		parsed, err := url.Parse(input)
		if err != nil {
			return "", fmt.Errorf("parse URL: %w", err)
		}
		code := parsed.Query().Get("code")
		if code == "" {
			return "", fmt.Errorf("no code found in URL")
		}
		return code, nil
	}

	if len(input) > 10 {
		return input, nil
	}

	return "", fmt.Errorf("input does not appear to be a valid code or URL")
}
