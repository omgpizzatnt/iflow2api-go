package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"iflow2api-go/internal/config"
	"iflow2api-go/internal/handlers"
	"iflow2api-go/internal/logger"
)

const usageText = `iflow2api - OpenAI-compatible API proxy for iFlow

Usage:
  iflow2api [command] [options]

Commands:
  (no command)    Start the API server
  oauth           Authenticate via OAuth

Global Options:
  -h, -?, --help          Show this help message
  -v, --version           Show version information
      --log-level LEVEL   Set log level: debug, info, warn, error (default: info)

Examples:
  # Start the server (auto-auth if no API key)
  iflow2api

  # Start with debug logging
  iflow2api --log-level debug

  # Manually authenticate via OAuth
  iflow2api oauth

For more information, visit: https://iflow2api-go
`

func showUsage() {
	fmt.Fprint(os.Stdout, usageText)
}

func showVersion() {
	fmt.Println("iflow2api - OpenAI-compatible API proxy for iFlow")
}

func setupLogLevel(level string) error {
	return logger.SetLevelString(level)
}

func parseGlobalArgs(args []string) (logLevel string, err error) {
	logLevel = "info"
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "-?", "--help":
			showUsage()
			os.Exit(0)
		case "-v", "--version":
			showVersion()
			os.Exit(0)
		case "--log-level":
			if i+1 >= len(args) {
				return "", fmt.Errorf("--log-level requires a value")
			}
			logLevel = args[i+1]
			i++
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", fmt.Errorf("unknown flag: %s", arg)
			}
			return "", fmt.Errorf("unknown argument: %s", arg)
		}
	}
	return logLevel, nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "oauth" {
		if err := runOAuthLogin(os.Args[2:]); err != nil {
			logger.Fatalf("OAuth login failed: %v", err)
		}
		return
	}

	logLevel, err := parseGlobalArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		showUsage()
		os.Exit(1)
	}

	if err := setupLogLevel(logLevel); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		logger.Fatalf("Invalid config: %v", err)
	}

	if cfg.APIKey.APIKey == "" {
		logger.Info("No API key configured. Starting OAuth authentication...")
		if err := runOAuthLogin([]string{}); err != nil {
			logger.Fatalf("OAuth login failed: %v", err)
		}

		cfg, err = config.Load()
		if err != nil || cfg.APIKey.APIKey == "" {
			logger.Fatal("Failed to load API key after OAuth. Please run 'iflow2api oauth' manually.")
		}
	}

	checkOAuthTokenStatus()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handlers.Health(cfg))
	mux.HandleFunc("GET /v1/models", handlers.Models(cfg))
	mux.HandleFunc("POST /v1/chat/completions", handlers.ChatCompletions(cfg))
	mux.HandleFunc("POST /v1/messages", handlers.Messages(cfg))
	mux.HandleFunc("GET /", handlers.Root(cfg))

	server := &http.Server{
		Addr:         cfg.GetServerAddr(),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		logger.Printf("Server starting on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Printf("Error during server shutdown: %v", err)
	}
	logger.Println("Server stopped")
}

func checkOAuthTokenStatus() {
	oauthConfig, err := config.LoadAppConfig()
	if err != nil || oauthConfig.OAuthExpiresAt.IsZero() {
		return
	}

	now := time.Now()
	timeUntilExpire := oauthConfig.OAuthExpiresAt.Sub(now)

	if timeUntilExpire < 0 {
		logger.Warn("\nOAuth token has expired!")
		logger.Warn("   API requests will fail. Please run: iflow2api oauth")
		return
	}

	if timeUntilExpire < 24*time.Hour {
		days := int(timeUntilExpire.Hours() / 24)
		hours := int(timeUntilExpire.Hours()) % 24
		logger.Warn("\nOAuth token expires in %d days %d hours", days, hours)
		logger.Warn("   Run 'iflow2api oauth' to refresh before it expires")
	} else {
		logger.Printf("OAuth token expires in %s", timeUntilExpire.Round(time.Hour))
	}
}
