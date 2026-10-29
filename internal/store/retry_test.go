package store

import (
	"testing"
	"time"
)

func TestRetryPolicyDelay(t *testing.T) {
	exp := RetryPolicy{Count: 3, Backoff: BackoffExponential, InitialMS: 500, MaxMS: 10000}
	tests := []struct {
		name   string
		policy RetryPolicy
		retry  int
		want   time.Duration
	}{
		{"exponential first", exp, 1, 500 * time.Millisecond},
		{"exponential second", exp, 2, 1000 * time.Millisecond},
		{"exponential third", exp, 3, 2000 * time.Millisecond},
		{"exponential capped", exp, 8, 10000 * time.Millisecond},
		{"linear constant", RetryPolicy{Backoff: BackoffLinear, LinearDelayMS: 5000}, 1, 5 * time.Second},
		{"linear third", RetryPolicy{Backoff: BackoffLinear, LinearDelayMS: 5000}, 3, 5 * time.Second},
		{"zero initial", RetryPolicy{Backoff: BackoffExponential, InitialMS: 0}, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.policy.Delay(tt.retry); got != tt.want {
				t.Fatalf("Delay(%d) = %v, want %v", tt.retry, got, tt.want)
			}
		})
	}
}

func TestRetryPolicyEnabled(t *testing.T) {
	if (RetryPolicy{Count: 0}).Enabled() {
		t.Fatal("retry count 0 must mean disabled so the routing rule stays in charge")
	}
	if !(RetryPolicy{Count: 1}).Enabled() {
		t.Fatal("retry count 1 must be enabled")
	}
}

func TestProxySettingsEnabled(t *testing.T) {
	cases := []struct {
		proxy ProxySettings
		want  bool
	}{
		{ProxySettings{}, false},
		{ProxySettings{Type: ProxyHTTP}, false},
		{ProxySettings{URL: "http://localhost:7890"}, false},
		{ProxySettings{Type: ProxyHTTP, URL: "http://localhost:7890"}, true},
	}
	for _, c := range cases {
		if got := c.proxy.Enabled(); got != c.want {
			t.Fatalf("%+v Enabled() = %v, want %v", c.proxy, got, c.want)
		}
	}
}
