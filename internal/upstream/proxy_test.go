package upstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gitsang/aigw/internal/store"
)

// newTarget builds a minimal runtime target pointing at a test server.
func newTarget(baseURL string) store.RuntimeTarget {
	return store.RuntimeTarget{
		BaseURL: baseURL, AuthType: "bearer", APIKey: "k",
		UpstreamPath: "/v1/chat/completions", TimeoutMS: 5000,
		APIType: store.APIChatCompletions,
	}
}

func TestDoRetriesUntilSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	target := newTarget(srv.URL)
	target.Retry = store.RetryPolicy{Count: 3, Backoff: store.BackoffLinear, LinearDelayMS: 1}

	resp, err := NewClient().Do(context.Background(), target, []byte(`{}`))
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("calls = %d, want 3", got)
	}
}

func TestDoReturnsLastFailureStatus(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	target := newTarget(srv.URL)
	target.Retry = store.RetryPolicy{Count: 2, Backoff: store.BackoffLinear, LinearDelayMS: 1}

	resp, err := NewClient().Do(context.Background(), target, []byte(`{}`))
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	// The real upstream status must survive retries so the gateway can report it.
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("calls = %d, want 3 (1 + 2 retries)", got)
	}
}

func TestDoDoesNotRetryClientErrors(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	target := newTarget(srv.URL)
	target.Retry = store.RetryPolicy{Count: 3, Backoff: store.BackoffLinear, LinearDelayMS: 1}

	resp, err := NewClient().Do(context.Background(), target, []byte(`{}`))
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls = %d, want 1 (4xx is not retryable)", got)
	}
}

func TestDoUsesProxy(t *testing.T) {
	var proxied int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// An HTTP proxy receives the absolute URL of the target.
		atomic.AddInt32(&proxied, 1)
		if r.URL.Host == "" {
			t.Errorf("proxy request has no absolute URL: %q", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer proxy.Close()

	target := newTarget("http://upstream.invalid")
	target.Proxy = store.ProxySettings{Type: store.ProxyHTTP, URL: proxy.URL}

	resp, err := NewClient().Do(context.Background(), target, []byte(`{}`))
	if err != nil {
		t.Fatalf("Do() via proxy error = %v", err)
	}
	defer resp.Body.Close()
	if got := atomic.LoadInt32(&proxied); got != 1 {
		t.Fatalf("proxy hits = %d, want 1", got)
	}
}

func TestDoWithoutProxyBypassesProxyServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	resp, err := NewClient().Do(context.Background(), newTarget(srv.URL), []byte(`{}`))
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestProxyURLValidation(t *testing.T) {
	tests := []struct {
		name    string
		proxy   store.ProxySettings
		wantErr bool
	}{
		{"http ok", store.ProxySettings{Type: store.ProxyHTTP, URL: "http://localhost:7890"}, false},
		{"http without scheme", store.ProxySettings{Type: store.ProxyHTTP, URL: "localhost:7890"}, false},
		{"socks5 ok", store.ProxySettings{Type: store.ProxySOCKS5, URL: "socks5://localhost:1080"}, false},
		{"socks5 with http scheme", store.ProxySettings{Type: store.ProxySOCKS5, URL: "http://localhost:1080"}, true},
		{"http with socks scheme", store.ProxySettings{Type: store.ProxyHTTP, URL: "socks5://localhost:1080"}, true},
		{"empty url", store.ProxySettings{Type: store.ProxyHTTP, URL: ""}, true},
		{"credentials", store.ProxySettings{Type: store.ProxyHTTP, URL: "http://p:1", Username: "u", Password: "p"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := ProxyURL(tt.proxy)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ProxyURL() = %v, want error", u)
				}
				return
			}
			if err != nil {
				t.Fatalf("ProxyURL() error = %v", err)
			}
			if tt.proxy.Username != "" {
				if pw, _ := u.User.Password(); u.User.Username() != tt.proxy.Username || pw != tt.proxy.Password {
					t.Fatalf("credentials not embedded: %v", u.User)
				}
			}
		})
	}
}
