// Package oauth provides local HTTP server for OAuth callback.
package oauth

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"iflow2api-go/internal/logger"
)

// CallbackServer handles OAuth callback requests.
type CallbackServer struct {
	host     string
	port     int
	server   *http.Server
	code     string
	error    string
	state    string
	callback OAuthCallbackHandler
	mu       sync.Mutex
	stopped  chan struct{}
}

// OAuthCallbackHandler is called when OAuth completes.
type OAuthCallbackHandler func(code, error, state string)

// NewCallbackServer creates a new OAuth callback server.
func NewCallbackServer(port int) *CallbackServer {
	return &CallbackServer{
		host:    "localhost",
		port:    port,
		stopped: make(chan struct{}),
	}
}

// Start starts the OAuth callback server.
func (s *CallbackServer) Start() error {
	if !s.isPortAvailable() {
		return fmt.Errorf("port %d is not available", s.port)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2callback", s.handleCallback)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.host, s.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("OAuth callback server error: %v", err)
		}
		close(s.stopped)
	}()

	return nil
}

// Stop stops the OAuth callback server.
func (s *CallbackServer) Stop() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			logger.Errorf("Error during server shutdown: %v", err)
		}
	}
}

// Done returns a channel that closes when server stops.
func (s *CallbackServer) Done() <-chan struct{} {
	return s.stopped
}

// SetCallbackHandler sets the callback handler.
func (s *CallbackServer) SetCallbackHandler(handler OAuthCallbackHandler) {
	s.callback = handler
}

// GetCallbackURL returns the callback URL.
func (s *CallbackServer) GetCallbackURL() string {
	return fmt.Sprintf("http://%s:%d/oauth2callback", s.host, s.port)
}

// isPortAvailable checks if the port is available.
func (s *CallbackServer) isPortAvailable() bool {
	conn, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.host, s.port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// handleCallback handles OAuth callback requests.
func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("code")
	errorMsg := query.Get("error")
	state := query.Get("state")

	s.mu.Lock()
	s.code = code
	s.error = errorMsg
	s.state = state
	s.mu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if code != "" {
		io.WriteString(w, successHTML)
	} else {
		io.WriteString(w, errorHTML(errorMsg))
	}

	if s.callback != nil {
		s.callback(code, errorMsg, state)
	}

	go s.Stop()
}

const successHTML = `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>登录成功</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;background:#f5f5f5}
.container{text-align:center;padding:40px;background:white;border-radius:8px;box-shadow:0 2px 10px rgba(0,0,0,0.1)}
.icon{font-size:64px;color:#4CAF50;margin-bottom:20px}
h1{color:#333;margin-bottom:10px}
p{color:#666;margin-bottom:20px}
.hint{font-size:14px;color:#999}
</style>
</head>
<body>
<div class="container">
<div class="icon">✓</div>
<h1>登录成功！</h1>
<p>您可以关闭此页面并返回应用程序。</p>
<p class="hint">此页面将在 5 秒后自动关闭...</p>
<script>setTimeout(function(){window.close()},5000)</script>
</div>
</body>
</html>`

func errorHTML(message string) string {
	msg := "授权失败"
	if message != "" {
		msg = message
	}

	html := `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>登录失败</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;background:#f5f5f5}
.container{text-align:center;padding:40px;background:white;border-radius:8px;box-shadow:0 2px 10px rgba(0,0,0,0.1)}
.icon{font-size:64px;color:#f44336;margin-bottom:20px}
h1{color:#333;margin-bottom:10px}
p{color:#666;margin-bottom:20px}
.error{color:#f44336;font-weight:bold;margin-bottom:20px}
.hint{font-size:14px;color:#999}
</style>
</head>
<body>
<div class="container">
<div class="icon">✕</div>
<h1>登录失败</h1>
<p class="error">` + msg + `</p>
<p>请重试或联系技术支持。</p>
<p class="hint">此页面将在 5 秒后自动关闭...</p>
<script>setTimeout(function(){window.close()},5000)</script>
</div>
</body>
</html>`

	return html
}

// GetResult returns the OAuth result (code, error, state).
func (s *CallbackServer) GetResult() (string, string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.code, s.error, s.state
}
