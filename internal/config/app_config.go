package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const defaultConfigFile = "config.json"

type AppConfig struct {
	APIKey            string    `json:"api_key"`
	BaseURL           string    `json:"base_url,omitempty"`
	OAuthAccessToken  string    `json:"oauth_access_token,omitempty"`
	OAuthRefreshToken string    `json:"oauth_refresh_token,omitempty"`
	OAuthExpiresAt    time.Time `json:"oauth_expires_at,omitempty"`
	OAuthExpiresAtISO string    `json:"oauth_expires_at_iso,omitempty"`
}

func LoadAppConfig() (*AppConfig, error) {
	configPath := getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AppConfig{
				BaseURL: "https://apis.iflow.cn/v1",
			}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if cfg.OAuthExpiresAtISO != "" && cfg.OAuthExpiresAt.IsZero() {
		cfg.OAuthExpiresAt, _ = time.Parse(time.RFC3339, cfg.OAuthExpiresAtISO)
	} else if !cfg.OAuthExpiresAt.IsZero() {
		cfg.OAuthExpiresAtISO = cfg.OAuthExpiresAt.Format(time.RFC3339)
	}

	return &cfg, nil
}

func SaveAppConfig(cfg *AppConfig) error {
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	configPath := getConfigPath()

	if !cfg.OAuthExpiresAt.IsZero() {
		cfg.OAuthExpiresAtISO = cfg.OAuthExpiresAt.Format(time.RFC3339)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func UpdateAppConfigWithToken(apiKey string, tokenData *TokenData) error {
	cfg, err := LoadAppConfig()
	if err != nil {
		cfg = &AppConfig{
			BaseURL: "https://apis.iflow.cn/v1",
		}
	}

	cfg.APIKey = apiKey
	cfg.OAuthAccessToken = tokenData.AccessToken
	cfg.OAuthRefreshToken = tokenData.RefreshToken
	cfg.OAuthExpiresAt = tokenData.ExpiresAt

	return SaveAppConfig(cfg)
}

func GetAPIKeyFromAppConfig() (string, error) {
	cfg, err := LoadAppConfig()
	if err != nil {
		return "", err
	}
	return cfg.APIKey, nil
}

func getExecDir() string {
	if execPath, err := os.Executable(); err == nil {
		return filepath.Dir(execPath)
	}
	return "."
}

func getConfigDir() string {
	return getExecDir()
}

func getConfigPath() string {
	return filepath.Join(getConfigDir(), defaultConfigFile)
}
