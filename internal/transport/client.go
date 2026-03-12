package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	fhttp "github.com/saucesteals/fhttp"
)

type TransportConfig struct {
	EnableImpersonation bool
	BrowserProfile      string
	Platform            string
	ProxyURL            string
	ConnectTimeout      time.Duration
	RequestTimeout      time.Duration
	IdleTimeout         time.Duration
	MaxIdleConns        int
	MaxConnsPerHost     int
}

func DefaultConfig() *TransportConfig {
	return &TransportConfig{
		EnableImpersonation: true,
		BrowserProfile:      "chrome124",
		Platform:            "windows",
		ConnectTimeout:      30 * time.Second,
		RequestTimeout:      300 * time.Second,
		IdleTimeout:         90 * time.Second,
		MaxIdleConns:        100,
		MaxConnsPerHost:     20,
	}
}

type Client struct {
	fhttpClient *fhttp.Client
	nhttpClient *http.Client
	config      *TransportConfig
}

func NewClient(cfg *TransportConfig) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	client := &Client{config: cfg}

	if cfg.EnableImpersonation && cfg.BrowserProfile != "" {
		transport, err := NewImpersonatedTransport(&Config{
			BrowserProfile: cfg.BrowserProfile,
			Platform:       cfg.Platform,
		})
		if err != nil {
			return nil, fmt.Errorf("create impersonated transport: %w", err)
		}

		if cfg.ProxyURL != "" {
			proxyURL, err := url.Parse(cfg.ProxyURL)
			if err != nil {
				return nil, fmt.Errorf("parse proxy URL: %w", err)
			}
			transport.Proxy = fhttp.ProxyURL(proxyURL)
		}

		client.fhttpClient = &fhttp.Client{
			Transport: transport,
			Timeout:   cfg.RequestTimeout,
		}
	} else {
		transport := createStandardTransport(cfg)

		client.nhttpClient = &http.Client{
			Transport: transport,
			Timeout:   cfg.RequestTimeout,
		}
	}

	return client, nil
}

func createStandardTransport(cfg *TransportConfig) *http.Transport {
	baseTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.ConnectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxConnsPerHost,
		IdleConnTimeout:       cfg.IdleTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
	}

	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err == nil {
			baseTransport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return baseTransport
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.fhttpClient != nil {
		return c.FHTTPDo(req)
	}
	return c.nhttpClient.Do(req)
}

func (c *Client) Get(url string) (*http.Response, error) {
	if c.fhttpClient != nil {
		return c.FHTTPGet(url)
	}
	return c.nhttpClient.Get(url)
}

func (c *Client) Post(requestURL string, body interface{}) (*http.Response, error) {
	if c.fhttpClient != nil {
		return c.FHTTPPost(requestURL, body)
	}
	return c.nhttpClientPost(requestURL, body)
}

func (c *Client) PostForm(requestURL string, data map[string]string) (*http.Response, error) {
	if c.fhttpClient != nil {
		return c.FHTTPPostForm(requestURL, data)
	}
	return c.nhttpClientPostForm(requestURL, data)
}

func (c *Client) FHTTPDo(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("FHTTPDo not implemented yet")
}

func (c *Client) FHTTPGet(requestURL string) (*http.Response, error) {
	return nil, fmt.Errorf("FHTTPGet not implemented yet")
}

func (c *Client) FHTTPPost(requestURL string, body interface{}) (*http.Response, error) {
	return nil, fmt.Errorf("FHTTPPost not implemented yet")
}

func (c *Client) FHTTPPostForm(requestURL string, data map[string]string) (*http.Response, error) {
	return nil, fmt.Errorf("FHTTPPostForm not implemented yet")
}

func (c *Client) nhttpClientPost(requestURL string, body interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}

	req, err := http.NewRequest("POST", requestURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return c.nhttpClient.Do(req)
}

func (c *Client) nhttpClientPostForm(requestURL string, data map[string]string) (*http.Response, error) {
	formData := &strings.Builder{}
	first := true
	for k, v := range data {
		if !first {
			formData.WriteString("&")
		}
		formData.WriteString(url.QueryEscape(k))
		formData.WriteString("=")
		formData.WriteString(url.QueryEscape(v))
		first = false
	}

	req, err := http.NewRequest("POST", requestURL, strings.NewReader(formData.String()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.nhttpClient.Do(req)
}

func (c *Client) Close() {
	if c.fhttpClient != nil && c.fhttpClient.Transport != nil {
		if transport, ok := c.fhttpClient.Transport.(*fhttp.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
	if c.nhttpClient != nil && c.nhttpClient.Transport != nil {
		if transport, ok := c.nhttpClient.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
}
