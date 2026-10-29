package store

import "testing"

func TestNormalizeSlug(t *testing.T) {
	tests := []struct{ slug, name, want string }{
		{"", "DeepSeek Official", "deepseek-official"},
		{"", "anthropic proxy", "anthropic-proxy"},
		{"", "A  B___C", "a-b-c"},
		{"custom-id", "ignored", "custom-id"},
		{"", "   ", ""},
		{"", "OpenAI/GPT", "openaigpt"},
	}
	for _, tt := range tests {
		if got := normalizeSlug(tt.slug, tt.name); got != tt.want {
			t.Fatalf("normalizeSlug(%q,%q) = %q, want %q", tt.slug, tt.name, got, tt.want)
		}
	}
}

func TestNormalizeModelCost(t *testing.T) {
	cost, err := normalizeModelCost(ModelCost{Currency: ""})
	if err != nil || cost.Currency != "USD" {
		t.Fatalf("empty currency should default to USD, got %q err=%v", cost.Currency, err)
	}
	if _, err := normalizeModelCost(ModelCost{Currency: "US"}); err == nil {
		t.Fatal("2-letter currency must be rejected")
	}
	if _, err := normalizeModelCost(ModelCost{Currency: "USD", Input: -1}); err == nil {
		t.Fatal("negative price must be rejected")
	}
	// Tiers must be ascending.
	_, err = normalizeModelCost(ModelCost{Currency: "USD", Tiers: []CostTier{
		{InputTokensAbove: 500}, {InputTokensAbove: 200},
	}})
	if err == nil {
		t.Fatal("unordered tiers must be rejected")
	}
	if _, err := normalizeModelCost(ModelCost{Currency: "cny", Tiers: []CostTier{
		{InputTokensAbove: 200_000, Input: 1},
	}}); err != nil {
		t.Fatalf("lowercase currency should be upper-cased: %v", err)
	}
}
