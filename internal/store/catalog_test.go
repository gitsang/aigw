package store

import "testing"

func TestChooseAPIType(t *testing.T) {
	all := []string{APIChatCompletions, APIResponses, APIMessages}
	tests := []struct {
		name      string
		supported []string
		enabled   []string
		inbound   string
		want      string
	}{
		{"same protocol wins", []string{APIMessages, APIChatCompletions}, all, APIMessages, APIMessages},
		{"fallback to first supported", []string{APIMessages}, all, APIChatCompletions, APIMessages},
		{"empty model uses global", nil, []string{APIResponses, APIChatCompletions}, APIChatCompletions, APIChatCompletions},
		{"empty model first global", nil, []string{APIResponses}, APIChatCompletions, APIResponses},
		{"no config defaults chat", nil, nil, APIResponses, APIChatCompletions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ChooseAPIType(tt.supported, tt.enabled, tt.inbound); got != tt.want {
				t.Fatalf("ChooseAPIType=%q want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	defaults := map[string]APITypeDef{
		APIChatCompletions: {Name: APIChatCompletions, Path: "/v1/chat/completions"},
		APIMessages:        {Name: APIMessages, Path: "/v1/messages"},
	}
	if got := ResolvePath(nil, defaults, APIMessages); got != "/v1/messages" {
		t.Fatalf("default path = %q", got)
	}
	if got := ResolvePath(map[string]string{APIMessages: "/anthropic/v1/messages"}, defaults, APIMessages); got != "/anthropic/v1/messages" {
		t.Fatalf("override path = %q", got)
	}
	if got := ResolvePath(nil, defaults, APIResponses); got != "/v1/responses" {
		t.Fatalf("builtin fallback = %q", got)
	}
}

func TestNormalizeModelInput(t *testing.T) {
	in, err := normalizeModelInput(ProviderModelInput{
		ModelID: " deepseek/deepseek-flash ", APITypes: []string{"messages", "messages", "bogus"},
		PathOverrides: map[string]string{"messages": "custom/messages"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if in.ModelID != "deepseek/deepseek-flash" || in.DisplayName != in.ModelID {
		t.Fatalf("bad normalize: %+v", in)
	}
	if len(in.APITypes) != 2 || in.APITypes[0] != APIMessages || in.APITypes[1] != APIChatCompletions {
		t.Fatalf("bad api types: %+v", in.APITypes)
	}
	if in.PathOverrides[APIMessages] != "/custom/messages" {
		t.Fatalf("bad path override: %+v", in.PathOverrides)
	}
	if _, err := normalizeModelInput(ProviderModelInput{ModelID: ""}); err == nil {
		t.Fatal("expected empty model_id error")
	}
}
