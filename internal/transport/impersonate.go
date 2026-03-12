// Package transport provides HTTP transport clients with TLS and browser impersonation support.
package transport

import (
	"fmt"
	"net"
	"time"

	"github.com/aarock1234/mimic"
	http "github.com/saucesteals/fhttp"
)

type Config struct {
	BrowserProfile string
	Platform       string
}

func ParseProfile(profile string, platform string) (*mimic.ClientSpec, error) {
	var spec *mimic.ClientSpec
	var err error

	switch profile {
	case "chrome", "chrome124":
		spec, err = mimic.Chromium(mimic.BrandChrome, "124.0.0.0")
	case "chrome123":
		spec, err = mimic.Chromium(mimic.BrandChrome, "123.0.0.0")
	case "firefox", "firefox120":
		spec, err = mimic.Firefox("120.0")
	case "firefox121":
		spec, err = mimic.Firefox("121.0")
	case "safari", "safari18":
		spec, err = mimic.Safari("18.0")
	case "edge", "edge124":
		spec, err = mimic.Chromium(mimic.BrandEdge, "124.0.0.0")
	case "brave", "brave124":
		spec, err = mimic.Chromium(mimic.BrandBrave, "124.0.0.0")
	default:
		spec, err = mimic.Chromium(mimic.BrandChrome, "124.0.0.0")
	}

	return spec, err
}

func ParsePlatform(platform string) mimic.Platform {
	switch platform {
	case "windows":
		return mimic.PlatformWindows
	case "mac", "macos":
		return mimic.PlatformMac
	case "linux":
		return mimic.PlatformLinux
	case "ios":
		return mimic.PlatformIOS
	case "ipados":
		return mimic.PlatformIPadOS
	default:
		return mimic.PlatformWindows
	}
}

func NewImpersonatedTransport(cfg *Config) (*http.Transport, error) {
	if cfg == nil || cfg.BrowserProfile == "" {
		return defaultTransport(), nil
	}

	spec, err := ParseProfile(cfg.BrowserProfile, cfg.Platform)
	if err != nil {
		return nil, fmt.Errorf("parse profile: %w", err)
	}

	platform := ParsePlatform(cfg.Platform)

	baseTransport := defaultTransport()
	if err := spec.ConfigureTransport(baseTransport, platform); err != nil {
		return nil, fmt.Errorf("configure transport: %w", err)
	}

	return baseTransport, nil
}

func defaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
