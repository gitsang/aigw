package app

import (
	"testing"

	"github.com/gitsang/aigw/internal/store"
	"github.com/gitsang/aigw/internal/translate"
)

func targetWithCost(cost store.ModelCost) store.RuntimeTarget {
	return store.RuntimeTarget{Cost: cost}
}

func TestCalculateCostFlat(t *testing.T) {
	target := targetWithCost(store.ModelCost{Currency: "USD", Input: 3, Output: 15})
	usage := translate.Usage{PromptTokens: 1_000_000, CompletionTokens: 1_000_000}
	if got := calculateCost(target, usage); got != 18 {
		t.Fatalf("cost = %v, want 18", got)
	}
}

func TestCalculateCostCacheSplit(t *testing.T) {
	// 1M prompt tokens: 400k cache reads, 100k cache writes, 500k fresh input.
	target := targetWithCost(store.ModelCost{
		Currency: "USD", Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75,
	})
	usage := translate.Usage{
		PromptTokens: 1_000_000, CompletionTokens: 0,
		CacheReadTokens: 400_000, CacheWriteTokens: 100_000,
	}
	want := 0.5*3 + 0.4*0.3 + 0.1*3.75 // 1.5 + 0.12 + 0.375
	if got := calculateCost(target, usage); got != want {
		t.Fatalf("cost = %v, want %v", got, want)
	}
}

func TestCalculateCostTierThreshold(t *testing.T) {
	cost := store.ModelCost{
		Currency: "USD", Input: 3, Output: 15,
		Tiers: []store.CostTier{{InputTokensAbove: 200_000, Input: 6, Output: 22.5}},
	}
	// Below the tier: flat prices.
	below := calculateCost(targetWithCost(cost), translate.Usage{PromptTokens: 200_000})
	if below != 0.6 {
		t.Fatalf("below tier cost = %v, want 0.6", below)
	}
	// Above the tier: tier prices apply to the whole request.
	above := calculateCost(targetWithCost(cost), translate.Usage{PromptTokens: 200_001})
	if want := 200_001.0 * 6 / 1e6; above != want {
		t.Fatalf("above tier cost = %v, want %v", above, want)
	}
}

func TestCalculateCostNoCurrencyIsFree(t *testing.T) {
	if got := calculateCost(store.RuntimeTarget{}, translate.Usage{PromptTokens: 1e6}); got != 0 {
		t.Fatalf("cost = %v, want 0 when no currency is configured", got)
	}
}

func TestCalculateCostCacheNeverExceedsPrompt(t *testing.T) {
	// Misconfigured upstream reporting more cached tokens than prompt tokens
	// must not produce negative fresh input.
	target := targetWithCost(store.ModelCost{Currency: "USD", Input: 1, CacheRead: 1})
	usage := translate.Usage{PromptTokens: 100, CacheReadTokens: 500, CacheWriteTokens: 500}
	if got := calculateCost(target, usage); got < 0 {
		t.Fatalf("cost = %v, must not be negative", got)
	}
}
