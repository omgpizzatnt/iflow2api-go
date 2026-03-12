package oauth

import (
	"fmt"
	"strings"
	"time"

	"iflow2api-go/internal/logger"
)

const (
	retryCount    = 5
	retryDelay    = 30 * time.Second
	maxRetryDelay = 5 * time.Minute
	refreshBuffer = 24 * time.Hour
	checkInterval = 6 * time.Hour
)

type Refresher struct {
	client        *OAuthClient
	checkInterval time.Duration
	refreshBuffer time.Duration
	stopChan      chan struct{}
	running       bool
	onRefreshFunc func(*TokenData) error
	retryCount    int
	retryDelay    time.Duration
}

func NewRefresher() *Refresher {
	return &Refresher{
		client:        NewOAuthClient(nil),
		checkInterval: checkInterval,
		refreshBuffer: refreshBuffer,
		stopChan:      make(chan struct{}),
		retryCount:    retryCount,
		retryDelay:    retryDelay,
	}
}

func (r *Refresher) SetOnRefresh(fn func(*TokenData) error) {
	r.onRefreshFunc = fn
}

func (r *Refresher) SetRetryPolicy(count int, delay time.Duration) {
	r.retryCount = count
	r.retryDelay = delay
}

func (r *Refresher) Start(refreshToken string, expiresAt time.Time) {
	if r.running {
		return
	}

	r.running = true
	r.stopChan = make(chan struct{})

	go r.run(refreshToken, expiresAt)
}

func (r *Refresher) Stop() {
	if !r.running {
		return
	}

	r.running = false
	close(r.stopChan)
}

func (r *Refresher) run(refreshToken string, expiresAt time.Time) {
	for r.running {
		select {
		case <-time.After(r.checkInterval):
			if r.shouldRefresh(expiresAt) {
				tokenResp, err := r.refreshWithRetry(refreshToken)
				if err != nil {
					continue
				}

				if r.onRefreshFunc != nil {
					if err := r.onRefreshFunc(tokenResp); err == nil {
						refreshToken = tokenResp.RefreshToken
						expiresAt = tokenResp.ExpiresAt
					}
				}
			}
		case <-r.stopChan:
			return
		}
	}
}

func (r *Refresher) shouldRefresh(expiresAt time.Time) bool {
	now := time.Now()
	timeUntilExpiry := expiresAt.Sub(now)

	if timeUntilExpiry <= 0 {
		return true
	}

	if timeUntilExpiry < r.refreshBuffer {
		return true
	}

	return false
}

func (r *Refresher) refreshWithRetry(refreshToken string) (*TokenData, error) {
	var lastError error

	for attempt := 1; attempt <= r.retryCount; attempt++ {
		tokenResp, err := r.client.RefreshToken(refreshToken)
		if err == nil {
			return tokenResp, nil
		}

		lastError = err

		if attempt < r.retryCount {
			if isTransientError(err) {
				delay := calculateExponentialBackoff(r.retryDelay, attempt)
				delay = minDuration(delay, maxRetryDelay)

				logger.Warn("Token refresh failed (attempt %d/%d): %v. Retrying in %v...",
					attempt, r.retryCount, err, delay)

				time.Sleep(delay)
				continue
			} else if isInvalidGrantError(err) {
				logger.Error("Token refresh failed: invalid grant, need to re-login: %v", err)
				return nil, err
			}
		}
	}

	return nil, fmt.Errorf("token refresh failed after %d attempts: %w", r.retryCount, lastError)
}

func isTransientError(err error) bool {
	errMsg := strings.ToLower(err.Error())

	transientKeywords := []string{
		"太多", "服务器过载", "overload", "timeout", "timed out",
		"connect", "网络", "503", "502", "429",
	}

	for _, keyword := range transientKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

func isInvalidGrantError(err error) bool {
	errMsg := strings.ToLower(err.Error())

	invalidGrantKeywords := []string{
		"invalid_grant", "invalid_token", "refresh_token 无效", "已过期",
	}

	for _, keyword := range invalidGrantKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

func calculateExponentialBackoff(baseDelay time.Duration, attempt int) time.Duration {
	return baseDelay * time.Duration(1<<(attempt-1))
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
