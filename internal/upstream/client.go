// Package upstream sends OpenAI/Anthropic-compatible requests to providers.
package upstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gitsang/aigw/internal/store"
)

type Client struct {
	HTTP *http.Client
}

func NewClient() *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 200
	transport.MaxIdleConnsPerHost = 50
	transport.IdleConnTimeout = 90 * time.Second
	transport.ResponseHeaderTimeout = 120 * time.Second
	return &Client{HTTP: &http.Client{Transport: transport}}
}

// Do forwards a request body to a routing target. The caller owns the response body.
func (c *Client) Do(ctx context.Context, target store.RuntimeTarget, body []byte) (*http.Response, error) {
	endpoint, err := BuildURL(target.BaseURL, target.UpstreamPath)
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(target.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	ApplyAuth(req.Header, target)
	for k, v := range target.Headers {
		if isSafeHeader(k) {
			req.Header.Set(k, v)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// BuildURL joins a provider base URL and an upstream path intelligently.
func BuildURL(baseURL, upstreamPath string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", fmt.Errorf("provider base_url is empty")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid provider base_url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("provider base_url must use http or https")
	}
	if upstreamPath == "" {
		upstreamPath = "/v1/chat/completions"
	}
	if !strings.HasPrefix(upstreamPath, "/") {
		upstreamPath = "/" + upstreamPath
	}
	basePath := strings.TrimSuffix(u.Path, "/")
	path := upstreamPath
	if strings.HasSuffix(basePath, "/v1") && strings.HasPrefix(path, "/v1/") {
		path = strings.TrimPrefix(path, "/v1")
	}
	u.Path = basePath + path
	u.RawQuery = ""
	return u.String(), nil
}

// ApplyAuth sets provider credentials on an outbound request.
func ApplyAuth(h http.Header, target store.RuntimeTarget) {
	switch target.AuthType {
	case "x-api-key":
		if target.APIKey != "" {
			h.Set("x-api-key", target.APIKey)
		}
	case "none":
	default:
		if target.APIKey != "" {
			h.Set("Authorization", "Bearer "+target.APIKey)
		}
	}
	if target.APIType == store.APIMessages && h.Get("anthropic-version") == "" {
		h.Set("anthropic-version", "2023-06-01")
	}
}

func isSafeHeader(k string) bool {
	switch strings.ToLower(k) {
	case "host", "content-length", "authorization", "x-api-key", "connection", "transfer-encoding":
		return false
	}
	return true
}
