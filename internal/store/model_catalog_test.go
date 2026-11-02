package store

import "testing"

func TestProviderModelPublicIdentity(t *testing.T) {
	tests := []struct {
		name         string
		providerSlug string
		providerName string
		modelID      string
		displayName  string
		wantID       string
		wantName     string
	}{
		{
			name:         "slug prefixes the model id verbatim",
			providerSlug: "yllm", providerName: "yllm",
			modelID: "deepseek/deepseek-flash", displayName: "deepseek/deepseek-flash",
			wantID: "yllm/deepseek/deepseek-flash", wantName: "yllm/deepseek/deepseek-flash",
		},
		{
			name:         "empty slug falls back to provider name",
			providerName: "yllm",
			modelID:      "gpt-4o", displayName: "",
			wantID: "yllm/gpt-4o", wantName: "yllm/gpt-4o",
		},
		{
			name:         "display name is used in the public name",
			providerSlug: "azure", providerName: "Azure OpenAI",
			modelID: "gpt-4o", displayName: "GPT-4o",
			wantID: "azure/gpt-4o", wantName: "Azure OpenAI/GPT-4o",
		},
		{
			name:    "model only",
			modelID: "gpt-4o", wantID: "gpt-4o", wantName: "gpt-4o",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := providerModelID(tt.providerSlug, tt.providerName, tt.modelID); got != tt.wantID {
				t.Fatalf("providerModelID()=%q want %q", got, tt.wantID)
			}
			if got := providerModelName(tt.providerName, tt.displayName, tt.modelID); got != tt.wantName {
				t.Fatalf("providerModelName()=%q want %q", got, tt.wantName)
			}
		})
	}
}

func TestEffectiveModelAPITypes(t *testing.T) {
	if got := effectiveModelAPITypes([]string{APIResponses}, []string{APIMessages}); len(got) != 1 || got[0] != APIResponses {
		t.Fatalf("model types should win: %v", got)
	}
	if got := effectiveModelAPITypes(nil, []string{APIMessages}); len(got) != 1 || got[0] != APIMessages {
		t.Fatalf("provider types should be inherited: %v", got)
	}
	if got := effectiveModelAPITypes(nil, nil); len(got) != 3 {
		t.Fatalf("built-in defaults expected: %v", got)
	}
}

func TestMatchProviderModel(t *testing.T) {
	candidates := []ModelMatch{
		{ProviderModelID: "a", ProviderSlug: "yllm", ProviderName: "yllm", ModelID: "deepseek/deepseek-flash"},
		{ProviderModelID: "b", ProviderSlug: "yllm", ProviderName: "yllm", ModelID: "deepseek-flash"},
		{ProviderModelID: "c", ProviderSlug: "openai", ProviderName: "OpenAI", ModelID: "gpt-4o"},
	}
	tests := []struct {
		name   string
		query  string
		wantID string
		ok     bool
	}{
		{name: "contains the requested name", query: "deepseek/deepseek-flash", wantID: "yllm/deepseek/deepseek-flash", ok: true},
		{name: "simple model name matches first containing model", query: "deepseek-flash", wantID: "yllm/deepseek/deepseek-flash", ok: true},
		{name: "case insensitive", query: "GPT-4O", wantID: "openai/gpt-4o", ok: true},
		{name: "trims whitespace", query: "  gpt-4o  ", wantID: "openai/gpt-4o", ok: true},
		{name: "no match", query: "claude", ok: false},
		{name: "empty query", query: "   ", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := matchProviderModel(candidates, tt.query)
			if ok != tt.ok {
				t.Fatalf("matchProviderModel() ok=%v want %v", ok, tt.ok)
			}
			if ok && got.ID() != tt.wantID {
				t.Fatalf("matchProviderModel() id=%q want %q", got.ID(), tt.wantID)
			}
		})
	}
}

func TestModelMatchID(t *testing.T) {
	m := ModelMatch{ProviderSlug: "yllm", ProviderName: "yllm", ModelID: "deepseek/deepseek-flash"}
	if got := m.ID(); got != "yllm/deepseek/deepseek-flash" {
		t.Fatalf("ID()=%q", got)
	}
}
