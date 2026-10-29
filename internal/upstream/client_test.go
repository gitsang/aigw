package upstream

import "testing"

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
