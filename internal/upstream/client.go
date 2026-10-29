// Package upstream sends OpenAI/Anthropic-compatible requests to providers.
package upstream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gitsang/aigw/internal/store"
)

// Client performs upstream requests. Providers that configure a proxy get their
// own pooled *http.Client so that direct and proxied traffic never share sockets.
type Client struct {
	direct *http.Client

	mu       sync.Mutex
	proxied  map[string]*http.Client
	lastUsed map[string]time.Time
}

func NewClient() *Client {
	return &Client{
		direct:   &http.Client{Transport: newTransport(nil)},
		proxied:  make(map[string]*http.Client),
		lastUsed: make(map[string]time.Time),
	}
}

func newTransport(proxy func(*http.Request) (*url.URL, error)) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 200
	transport.MaxIdleConnsPerHost = 50
	transport.IdleConnTimeout = 90 * time.Second
	transport.ResponseHeaderTimeout = 120 * time.Second
	if proxy != nil {
		transport.Proxy = proxy
	}
	return transport
}

// clientFor returns the HTTP client to use for a target, creating and caching a
// dedicated transport when the provider declares a proxy.
func (c *Client) clientFor(proxy store.ProxySettings) (*http.Client, error) {
	if !proxy.Enabled() {
		return c.direct, nil
	}
	key := proxyKey(proxy)
	c.mu.Lock()
	defer c.mu.Unlock()
	if client, ok := c.proxied[key]; ok {
		c.lastUsed[key] = time.Now()
		return client, nil
	}
	proxyURL, err := buildProxyURL(proxy)
	if err != nil {
		return nil, err
	}
	transport := newTransport(http.ProxyURL(proxyURL))
	client := &http.Client{Transport: transport}
	c.proxied[key] = client
	c.lastUsed[key] = time.Now()
	return client, nil
}

func proxyKey(proxy store.ProxySettings) string {
	return fmt.Sprintf("%s|%s|%s|%s", proxy.Type, proxy.URL, proxy.Username, proxy.Password)
}

// ProxyURL assembles a proxy endpoint from provider settings, embedding
// credentials when present.
func ProxyURL(proxy store.ProxySettings) (*url.URL, error) { return buildProxyURL(proxy) }

// buildProxyURL assembles the proxy endpoint, embedding credentials when present.
func buildProxyURL(proxy store.ProxySettings) (*url.URL, error) {
	raw := strings.TrimSpace(proxy.URL)
	if raw == "" {
		return nil, errors.New("proxy url is empty")
	}
	if !strings.Contains(raw, "://") {
		scheme := "http"
		if proxy.Type == store.ProxySOCKS5 {
			scheme = "socks5"
		}
		raw = scheme + "://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url: %w", err)
	}
	switch proxy.Type {
	case store.ProxySOCKS5:
		if parsed.Scheme != "socks5" && parsed.Scheme != "socks5h" {
			return nil, fmt.Errorf("socks5 proxy must use a socks5:// url, got %q", parsed.Scheme)
		}
	case store.ProxyHTTP:
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, fmt.Errorf("http proxy must use an http(s):// url, got %q", parsed.Scheme)
		}
	default:
		return nil, fmt.Errorf("unsupported proxy type %q", proxy.Type)
	}
	if parsed.Host == "" {
		return nil, errors.New("proxy url is missing a host")
	}
	if proxy.Username != "" {
		parsed.User = url.UserPassword(proxy.Username, proxy.Password)
	}
	return parsed, nil
}

// Do forwards a request body to a routing target. The caller owns the response body.
//
// When the provider configures its own retry policy, retries happen here, so a
// single routing attempt may hit the same upstream more than once. The final
// attempt's response is returned unchanged so the caller can report the real
// status code and error body.
func (c *Client) Do(ctx context.Context, target store.RuntimeTarget, body []byte) (*http.Response, error) {
	policy := target.Retry
	attempts := 1
	if policy.Enabled() {
		attempts = policy.Count + 1
	}
	var lastErr error
	for attempt := 1; ; attempt++ {
		resp, retryable, err := c.doOnce(ctx, target, body)
		last := attempt >= attempts
		if err != nil {
			lastErr = err
			if !retryable || last {
				return nil, err
			}
		} else {
			if !retryable || last {
				return resp, nil
			}
			drainAndClose(resp)
		}
		if last {
			return nil, lastErr
		}
		delay := policy.Delay(attempt)
		if delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func drainAndClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	_ = resp.Body.Close()
}

// doOnce performs a single upstream call and reports whether it may be retried.
func (c *Client) doOnce(ctx context.Context, target store.RuntimeTarget, body []byte) (*http.Response, bool, error) {
	client, err := c.clientFor(target.Proxy)
	if err != nil {
		// A misconfigured proxy will not be fixed by retrying.
		return nil, false, err
	}
	endpoint, err := BuildURL(target.BaseURL, target.UpstreamPath)
	if err != nil {
		return nil, false, err
	}
	timeout := time.Duration(target.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	ApplyAuth(req.Header, target)
	for k, v := range target.Headers {
		if isSafeHeader(k) {
			req.Header.Set(k, v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		if reqCtx.Err() != nil {
			return nil, false, reqCtx.Err()
		}
		return nil, isRetryableNetworkError(err), err
	}
	resp.Body = &cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}
	retryable := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
	return resp, retryable, nil
}

// isRetryableNetworkError treats connection-level failures as transient.
func isRetryableNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return !errors.Is(err, context.Canceled)
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
	u.Path = strings.TrimSuffix(u.Path, "/") + upstreamPath
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
