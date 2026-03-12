package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type IFlowConfig struct {
	APIKey            string    `json:"apiKey"`
	BaseURL           string    `json:"base_url,omitempty"`
	AuthType          string    `json:"auth_type,omitempty"`
	OAuthAccessToken  string    `json:"oauth_access_token,omitempty"`
	OAuthRefreshToken string    `json:"oauth_refresh_token,omitempty"`
	OAuthExpiresAt    time.Time `json:"oauth_expires_at,omitempty"`
	APIKeyExpiresAt   time.Time `json:"api_key_expires_at,omitempty"`
}

type TokenData struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

func LoadIFlowConfig() (*IFlowConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".iflow", "settings.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg IFlowConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}

func SaveIFlowConfig(cfg *IFlowConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".iflow")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "settings.json")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

func UpdateConfigWithToken(apiKey string, tokenData *TokenData) error {
	cfg, err := LoadIFlowConfig()
	if err != nil {
		cfg = &IFlowConfig{
			BaseURL:  "https://apis.iflow.cn/v1",
			AuthType: "oauth-iflow",
		}
	}

	cfg.APIKey = apiKey
	cfg.AuthType = "oauth-iflow"
	cfg.OAuthAccessToken = tokenData.AccessToken
	cfg.OAuthRefreshToken = tokenData.RefreshToken
	cfg.OAuthExpiresAt = tokenData.ExpiresAt
	cfg.APIKeyExpiresAt = tokenData.ExpiresAt

	return SaveIFlowConfig(cfg)
}

func CheckIFlowLogin() bool {
	cfg, err := LoadIFlowConfig()
	if err != nil {
		return false
	}
	return cfg.APIKey != ""
}

func GetAPIKey() (string, error) {
	cfg, err := LoadIFlowConfig()
	if err != nil {
		return "", err
	}
	return cfg.APIKey, nil
}

func GetOAuthInfo() (*OAuthInfo, error) {
	cfg, err := LoadIFlowConfig()
	if err != nil {
		return nil, err
	}

	return &OAuthInfo{
		AccessToken:  cfg.OAuthAccessToken,
		RefreshToken: cfg.OAuthRefreshToken,
		ExpiresAt:    cfg.OAuthExpiresAt,
		AuthType:     cfg.AuthType,
	}, nil
}

type OAuthInfo struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	AuthType     string
}
