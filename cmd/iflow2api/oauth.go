package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"iflow2api-go/internal/config"
	"iflow2api-go/internal/logger"
	"iflow2api-go/internal/oauth"
)

type OAuthLoginHandler struct {
	oauthClient *oauth.OAuthClient
}

func NewOAuthLoginHandler() *OAuthLoginHandler {
	return &OAuthLoginHandler{
		oauthClient: oauth.NewOAuthClient(nil),
	}
}

func (h *OAuthLoginHandler) Login() error {
	port, err := findAvailablePort(19198)
	if err != nil {
		return fmt.Errorf("find available port: %w", err)
	}

	fmt.Printf("Starting OAuth callback server on port %d...\n", port)

	callbackURL := fmt.Sprintf("http://localhost:%d/callback", port)
	state := oauth.GenerateState()

	authURL := h.oauthClient.GetAuthURL(callbackURL, state)

	fmt.Printf("\nVisit this URL in your browser:\n%s\n\n", authURL)
	fmt.Println("Wait for the redirect or paste the callback URL here.")
	fmt.Println("Press Ctrl+C to cancel.")

	server := &CallbackServer{
		Port:    port,
		State:   state,
		oauthCl: h.oauthClient,
	}

	if err := server.Start(); err != nil {
		return fmt.Errorf("start callback server: %w", err)
	}
	defer server.Stop()

	select {
	case result := <-server.ResultChan:
		if result.Code == "" {
			return fmt.Errorf("no authorization code received: %s", result.Error)
		}

		return h.completeLogin(result.Code, callbackURL)
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timeout waiting for authorization code")
	}
}

func (h *OAuthLoginHandler) completeLogin(code, redirectURI string) error {
	fmt.Println("\nReceived authorization code. Getting token...")

	tokenResp, err := h.oauthClient.GetToken(code, redirectURI)
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}

	fmt.Println("Got token. Getting user info...")

	userInfo, err := h.oauthClient.GetUserInfo(tokenResp.AccessToken)
	if err != nil {
		return fmt.Errorf("get user info: %w", err)
	}

	username := userInfo.Username
	if username == "" {
		username = userInfo.Phone
	}

	if username == "" {
		username = "Unknown"
	}

	fmt.Printf("\nLogin successful! User: %s\n", username)
	fmt.Printf("API Key: %s\n", userInfo.APIKey)

	configData := &config.TokenData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	if err := config.UpdateAppConfigWithToken(userInfo.APIKey, configData); err != nil {
		logger.Warn("failed to save config: %v", err)
	}

	fmt.Println("\nConfiguration saved to config.json")
	fmt.Printf("Token will expire at: %s\n", configData.ExpiresAt.Format("2006-01-02 15:04:05"))
	fmt.Println("\nNOTE: OAuth tokens will expire. When expired:")
	fmt.Println("      Run: iflow2api oauth")
	fmt.Println("      Then start server: iflow2api")

	return nil
}

type OAuthResult struct {
	Code  string
	Error string
	State string
}

type CallbackServer struct {
	Port       int
	State      string
	oauthCl    *oauth.OAuthClient
	ResultChan chan OAuthResult
	httpServer *http.Server
}

func (s *CallbackServer) Start() error {
	s.ResultChan = make(chan OAuthResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleCallback)

	s.httpServer = &http.Server{
		Addr:        fmt.Sprintf(":%d", s.Port),
		Handler:     mux,
		ReadTimeout: 10 * time.Second,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.ResultChan <- OAuthResult{Error: fmt.Sprintf("server error: %v", err)}
		}
	}()

	return nil
}

func (s *CallbackServer) Stop() {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			logger.Warn("Error during HTTP server shutdown: %v", err)
		}
	}
}

func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	error := query.Get("error")
	if error != "" {
		s.ResultChan <- OAuthResult{Error: fmt.Sprintf("oauth error: %s", error)}
		http.Error(w, error, http.StatusBadRequest)
		return
	}

	state := query.Get("state")
	if state != "" && state != s.State {
		s.ResultChan <- OAuthResult{Error: "invalid state"}
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	code := query.Get("code")
	if code == "" {
		s.ResultChan <- OAuthResult{Error: "no code provided"}
		http.Error(w, "No code provided", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Authentication successful! You can close this window.")

	s.ResultChan <- OAuthResult{Code: code, State: state}
}

func findAvailablePort(startPort int) (int, error) {
	for port := startPort; port < startPort+50; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue
		}
		ln.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no available port found")
}

const oauthUsageText = `OAuth Login - Authenticate with iFlow

Usage:
  iflow2api oauth

Options:
  -h, --help   Show this help message

Example:
  iflow2api oauth

The process:
1. visit the displayed authorization URL in your browser
2. authenticate with iFlow
3. wait for the redirect or paste the callback URL here
`

func showOAuthUsage() {
	fmt.Fprint(os.Stdout, oauthUsageText)
}

func validateOAuthArgs(args []string) error {
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			showOAuthUsage()
			os.Exit(0)
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return fmt.Errorf("unknown flag: %s", arg)
			}
			return fmt.Errorf("unknown argument: %s (no arguments accepted)", arg)
		}
	}
	return nil
}

func runOAuthLogin(args []string) error {
	if err := validateOAuthArgs(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		showOAuthUsage()
		os.Exit(1)
	}

	handler := NewOAuthLoginHandler()
	if err := handler.Login(); err != nil {
		return fmt.Errorf("OAuth login failed: %w", err)
	}

	return nil
}
