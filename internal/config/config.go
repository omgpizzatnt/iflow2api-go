// Package config provides configuration management for iflow2api-go.
package config

import (
	"strconv"
	"time"

	"github.com/caarlos0/env/v6"
)

// Config represents the application configuration.
type Config struct {
	Server  ServerConfig  `envPrefix:"SERVER_"`
	APIKey  APIKeyConfig  `envPrefix:"API_KEY_"`
	Proxy   ProxyConfig   `envPrefix:"PROXY_"`
	TLS     TLSConfig     `envPrefix:"TLS_"`
	Logging LoggingConfig `envPrefix:"LOG_"`
}

// ServerConfig represents server configuration.
type ServerConfig struct {
	Host         string        `env:"HOST" envDefault:"0.0.0.0"`
	Port         int           `env:"PORT" envDefault:"28000"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT" envDefault:"30s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"120s"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT" envDefault:"120s"`
}

// APIKeyConfig represents API key configuration loaded from environment or config file.
type APIKeyConfig struct {
	// API key for iFlow authentication (from env or ~/.iflow/settings.json)
	APIKey string `env:"API_KEY"`

	// Base URL for iFlow API
	BaseURL string `env:"BASE_URL" envDefault:"https://apis.iflow.cn/v1"`
}

// ProxyConfig represents upstream proxy configuration.
type ProxyConfig struct {
	// Enable upstream proxy
	Enabled bool `env:"ENABLED" envDefault:"false"`
	// Proxy URL (e.g., "http://127.0.0.1:7890" or "socks5://127.0.0.1:1080")
	URL string `env:"URL"`
}

// TLSConfig represents TLS impersonation configuration.
type TLSConfig struct {
	// Enable TLS browser impersonation
	Enabled bool `env:"ENABLED" envDefault:"true"`

	// Browser profile to impersonate (default: chrome124)
	// Supported: chrome100, chrome116, chrome119, chrome120, chrome123, chrome124, chrome131
	//             firefox102, firefox105, firefox120
	//             safari16, safari17, safari18
	BrowserProfile string `env:"BROWSER_PROFILE" envDefault:"chrome124"`

	// Platform for browser fingerprint (windows, mac, linux)
	Platform string `env:"PLATFORM" envDefault:"windows"`
}

// LoggingConfig represents logging configuration.
type LoggingConfig struct {
	// Log level: debug, info, warn, error (default: info)
	Level string `env:"LEVEL" envDefault:"info"`
}

// Load loads configuration from environment variables.
// Environment variables take precedence over defaults.
func Load() (*Config, error) {
	cfg := &Config{}

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	// Try to load API key from app config file if not set in env
	if cfg.APIKey.APIKey == "" {
		if apiKey, err := GetAPIKeyFromAppConfig(); err == nil && apiKey != "" {
			cfg.APIKey.APIKey = apiKey
		}
	}

	return cfg, nil
}

// GetServerAddr returns the server address in host:port format.
func (c *Config) GetServerAddr() string {
	return c.Server.Host + ":" + strconv.Itoa(c.Server.Port)
}

// IsTLSEnabled returns true if TLS impersonation is enabled.
func (c *Config) IsTLSEnabled() bool {
	return c.TLS.Enabled && c.TLS.BrowserProfile != ""
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.APIKey.APIKey == "" {
		// This is not critical - user may be using OAuth or want to set it later
	}

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return &ValidationError{Field: "SERVER_PORT", Message: "port must be between 1 and 65535"}
	}

	if c.Proxy.Enabled && c.Proxy.URL == "" {
		return &ValidationError{Field: "PROXY_URL", Message: "proxy URL is required when proxy is enabled"}
	}

	return nil
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "config validation error: " + e.Field + " - " + e.Message
}
