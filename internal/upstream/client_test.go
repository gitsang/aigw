package upstream

import (
	"net/http"
	"testing"

	"github.com/gitsang/aigw/internal/store"
)

func TestBuildURL(t *testing.T) {
	tests := []struct{ base, path, want string }{
		{"https://api.openai.com", "/v1/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"https://openrouter.ai/api/v1", "/chat/completions", "https://openrouter.ai/api/v1/chat/completions"},
		{"https://openrouter.ai/api/v1/", "/chat/completions", "https://openrouter.ai/api/v1/chat/completions"},
		{"http://localhost:9000", "v1/messages", "http://localhost:9000/v1/messages"},
	}
	for _, tt := range tests {
		got, err := BuildURL(tt.base, tt.path)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Fatalf("BuildURL(%q,%q)=%q want %q", tt.base, tt.path, got, tt.want)
		}
	}
}

func TestApplyAuth(t *testing.T) {
	tests := []struct {
		name   string
		target store.RuntimeTarget
		want   map[string]string
		absent []string
	}{
		{
			name:   "bearer is the default auth type",
			target: store.RuntimeTarget{AuthType: "", APIKey: "sk-secret"},
			want:   map[string]string{"Authorization": "Bearer sk-secret"},
			absent: []string{"x-api-key", "x-goog-api-key"},
		},
		{
			name:   "x-api-key auth type",
			target: store.RuntimeTarget{AuthType: "x-api-key", APIKey: "anthropic-secret"},
			want:   map[string]string{"x-api-key": "anthropic-secret"},
			absent: []string{"Authorization", "x-goog-api-key"},
		},
		{
			name:   "x-goog-api-key auth type",
			target: store.RuntimeTarget{AuthType: "x-goog-api-key", APIKey: "google-secret"},
			want:   map[string]string{"x-goog-api-key": "google-secret"},
			absent: []string{"Authorization", "x-api-key"},
		},
		{
			name:   "x-goog-api-key with empty key sets nothing",
			target: store.RuntimeTarget{AuthType: "x-goog-api-key"},
			absent: []string{"Authorization", "x-api-key", "x-goog-api-key"},
		},
		{
			name:   "none auth type sets nothing",
			target: store.RuntimeTarget{AuthType: "none", APIKey: "ignored"},
			absent: []string{"Authorization", "x-api-key", "x-goog-api-key"},
		},
		{
			name:   "messages keeps anthropic-version",
			target: store.RuntimeTarget{AuthType: "x-api-key", APIKey: "k", APIType: store.APIMessages},
			want:   map[string]string{"x-api-key": "k", "anthropic-version": "2023-06-01"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			ApplyAuth(h, tt.target)
			for k, want := range tt.want {
				if got := h.Get(k); got != want {
					t.Fatalf("%s = %q, want %q", k, got, want)
				}
			}
			for _, k := range tt.absent {
				if got := h.Get(k); got != "" {
					t.Fatalf("%s = %q, want it unset", k, got)
				}
			}
		})
	}
}

func TestIsSafeHeaderBlocksAuthHeaders(t *testing.T) {
	// Custom provider headers must never override the credential headers the
	// gateway sets itself, including the Google one.
	for _, k := range []string{"Authorization", "x-api-key", "X-Goog-Api-Key", "Host", "Content-Length"} {
		if isSafeHeader(k) {
			t.Errorf("isSafeHeader(%q) = true, want false", k)
		}
	}
	for _, k := range []string{"x-custom", "Api-Revision", "anthropic-beta"} {
		if !isSafeHeader(k) {
			t.Errorf("isSafeHeader(%q) = false, want true", k)
		}
	}
}
