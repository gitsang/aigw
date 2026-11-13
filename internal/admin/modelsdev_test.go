package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gitsang/aigw/internal/modelsdev"
	"github.com/gitsang/aigw/internal/store"
)

// stubCatalog is a priceCatalog with canned lookup results.
type stubCatalog struct {
	lookups map[string]modelsdev.Lookup
	err     error
	fetched time.Time
	calls   int
}

func (s *stubCatalog) Lookup(_ context.Context, _ string, _ string, modelID string) (modelsdev.Lookup, error) {
	s.calls++
	if s.err != nil {
		return modelsdev.Lookup{}, s.err
	}
	if lookup, ok := s.lookups[modelID]; ok {
		return lookup, nil
	}
	return modelsdev.Lookup{ModelID: modelID, FetchedAt: s.fetched, Matches: []modelsdev.Match{}}, nil
}

func (s *stubCatalog) FetchedAt() time.Time { return s.fetched }

// Matches returns the stub entries for a model, mimicking the uncapped catalog
// enumeration used for explicit source selection.
func (s *stubCatalog) Matches(_ context.Context, modelID string) ([]modelsdev.Match, time.Time, error) {
	if s.err != nil {
		return []modelsdev.Match{}, time.Time{}, s.err
	}
	lookup, ok := s.lookups[modelID]
	if !ok {
		return []modelsdev.Match{}, s.fetched, nil
	}
	return lookup.Matches, s.fetched, nil
}

func matchAt(providerID, modelID string, score int) modelsdev.Match {
	return modelsdev.Match{
		ProviderID: providerID, ProviderName: providerID, ModelID: modelID, ModelName: modelID,
		Cost: store.ModelCost{Currency: "USD", Input: 1, Output: 2}, Score: score,
	}
}

func pricedModel(modelID string, input, output float64) store.ProviderModel {
	return store.ProviderModel{
		ID: modelID + "-id", ModelID: modelID,
		Cost: store.ModelCost{Currency: "USD", Input: input, Output: output},
	}
}

func TestPlanPriceSyncActions(t *testing.T) {
	tests := []struct {
		name      string
		model     store.ProviderModel
		matches   []modelsdev.Match
		overwrite bool
		want      string
	}{
		{name: "no match", model: pricedModel("a", 0, 0), want: priceActionUnmatched},
		{
			name:  "exact provider match updates an unpriced model",
			model: pricedModel("a", 0, 0), matches: []modelsdev.Match{matchAt("p", "a", 0)},
			want: priceActionUpdate,
		},
		{
			name:  "two best-scoring matches are ambiguous",
			model: pricedModel("a", 0, 0),
			matches: []modelsdev.Match{
				matchAt("p1", "a", 0), matchAt("p2", "a", 0), matchAt("p3", "a", 2),
			},
			want: priceActionAmbiguous,
		},
		{
			name:  "a better-scored match resolves the ambiguity",
			model: pricedModel("a", 0, 0),
			matches: []modelsdev.Match{
				matchAt("p1", "a", 0), matchAt("p2", "a", 2), matchAt("p3", "a", 2),
			},
			want: priceActionUpdate,
		},
		{
			name:  "existing price is skipped without overwrite",
			model: pricedModel("a", 3, 4), matches: []modelsdev.Match{matchAt("p", "a", 0)},
			want: priceActionSkipHasPrice,
		},
		{
			name:  "existing price is replaced with overwrite",
			model: pricedModel("a", 3, 4), matches: []modelsdev.Match{matchAt("p", "a", 0)},
			overwrite: true, want: priceActionUpdate,
		},
		{
			// The stub match prices 1/2; the model already has 1/2 and no limits.
			name:  "identical price is unchanged",
			model: pricedModel("a", 1, 2), matches: []modelsdev.Match{matchAt("p", "a", 0)},
			overwrite: true, want: priceActionUnchanged,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{modelsDev: &stubCatalog{
				lookups: map[string]modelsdev.Lookup{tt.model.ModelID: {Matches: tt.matches}},
			}}
			item, err := h.planPriceSync(context.Background(), store.Provider{}, tt.model, tt.overwrite, "")
			if err != nil {
				t.Fatalf("planPriceSync: %v", err)
			}
			if item.Action != tt.want {
				t.Fatalf("action = %q, want %q (item %+v)", item.Action, tt.want, item)
			}
		})
	}
}

func TestPlanPriceSyncNeverWrites(t *testing.T) {
	// The planner reports what would change; the handler decides to persist.
	model := pricedModel("a", 0, 0)
	h := &Handler{modelsDev: &stubCatalog{lookups: map[string]modelsdev.Lookup{
		"a": {Matches: []modelsdev.Match{matchAt("p", "a", 0)}},
	}}}
	item, err := h.planPriceSync(context.Background(), store.Provider{}, model, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if item.Old == nil || item.New == nil {
		t.Fatalf("expected old/new snapshots, got %+v", item)
	}
	if item.Old.Cost.Input != 0 || item.New.Cost.Input != 1 {
		t.Fatalf("unexpected snapshot values: old=%+v new=%+v", item.Old, item.New)
	}
	if model.Cost.Input != 0 {
		t.Fatal("planner must not mutate the input model")
	}
}

func TestPlanPriceSyncFillsOnlyMissingLimits(t *testing.T) {
	match := matchAt("p", "a", 0)
	match.ContextWindow = 200_000
	match.MaxOutputTokens = 64_000
	catalog := &stubCatalog{lookups: map[string]modelsdev.Lookup{"a": {Matches: []modelsdev.Match{match}}}}

	tests := []struct {
		name        string
		contextWin  int
		maxOutput   int
		overwrite   bool
		wantContext int
		wantMaxOut  int
	}{
		{"fills both when empty", 0, 0, false, 200_000, 64_000},
		{"keeps existing limits without overwrite", 128_000, 8_000, false, 128_000, 8_000},
		{"partial: keeps set, fills unset", 128_000, 0, false, 128_000, 64_000},
		{"overwrite replaces limits", 128_000, 8_000, true, 200_000, 64_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := store.ProviderModel{
				ID: "a-id", ModelID: "a",
				ContextWindow: tt.contextWin, MaxOutputTokens: tt.maxOutput,
			}
			h := &Handler{modelsDev: catalog}
			item, err := h.planPriceSync(context.Background(), store.Provider{}, model, tt.overwrite, "")
			if err != nil {
				t.Fatal(err)
			}
			if item.New.ContextWindow != tt.wantContext || item.New.MaxOutputTokens != tt.wantMaxOut {
				t.Fatalf("limits = %d/%d, want %d/%d",
					item.New.ContextWindow, item.New.MaxOutputTokens, tt.wantContext, tt.wantMaxOut)
			}
		})
	}
}

func TestPlanPriceSyncPropagatesCatalogError(t *testing.T) {
	h := &Handler{modelsDev: &stubCatalog{err: errors.New("boom: " + modelsdev.ErrFetch.Error())}}
	if _, err := h.planPriceSync(context.Background(), store.Provider{}, pricedModel("a", 0, 0), false, ""); err == nil {
		t.Fatal("expected an error when the catalog lookup fails")
	}
}

func TestSamePriceSnapshot(t *testing.T) {
	base := priceSnapshot{Cost: store.ModelCost{Currency: "USD", Input: 1, Output: 2}, ContextWindow: 100}
	tests := []struct {
		name string
		next priceSnapshot
		want bool
	}{
		{"equal", base, true},
		{"currency case-insensitive", priceSnapshot{Cost: store.ModelCost{Currency: "usd", Input: 1, Output: 2}, ContextWindow: 100}, true},
		{"price differs", priceSnapshot{Cost: store.ModelCost{Currency: "USD", Input: 1, Output: 3}, ContextWindow: 100}, false},
		{"limit differs", priceSnapshot{Cost: store.ModelCost{Currency: "USD", Input: 1, Output: 2}, ContextWindow: 101}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := samePriceSnapshot(base, tt.next); got != tt.want {
				t.Fatalf("samePriceSnapshot = %v, want %v", got, tt.want)
			}
		})
	}

	withTier := base
	withTier.Cost.Tiers = []store.CostTier{{InputTokensAbove: 200_000, Input: 2, Output: 4}}
	if samePriceSnapshot(base, withTier) {
		t.Fatal("tier difference must count as a change")
	}
	if !samePriceSnapshot(withTier, withTier) {
		t.Fatal("identical tiers must compare equal")
	}
}

func TestPriceSyncCountsCoverEveryItem(t *testing.T) {
	// The response contract requires all six count keys, even when zero.
	counts := make(map[string]int, len(priceActions))
	for _, action := range priceActions {
		counts[action] = 0
	}
	if len(counts) != 6 {
		t.Fatalf("expected 6 actions, got %d", len(counts))
	}
	for _, action := range priceActions {
		if _, ok := counts[action]; !ok {
			t.Fatalf("missing count key %q", action)
		}
	}
}

func TestPlanPriceSyncExplicitSource(t *testing.T) {
	// The live yllm provider is an aggregator: its models.dev id does not exist,
	// and the same model id appears under many catalog providers at different
	// prices. An explicit source must resolve that instead of reporting
	// "ambiguous" for every row.
	priced := func(providerID string, input float64) modelsdev.Match {
		return modelsdev.Match{
			ProviderID: providerID, ProviderName: providerID, ModelID: "qwen3.8-flash",
			Cost: store.ModelCost{Currency: "USD", Input: input, Output: input * 3},
		}
	}
	model := pricedModel("qwen3.8-flash", 0, 0)
	tests := []struct {
		name       string
		source     string
		matches    []modelsdev.Match
		wantAction string
		wantInput  float64
	}{
		{
			name:   "no source and many equally-ranked providers is ambiguous",
			source: "", matches: []modelsdev.Match{priced("alibaba-cn", 0.1), priced("openrouter", 0.3)},
			wantAction: priceActionAmbiguous,
		},
		{
			name:   "explicit source picks its own prices",
			source: "openrouter", matches: []modelsdev.Match{priced("alibaba-cn", 0.1), priced("openrouter", 0.3)},
			wantAction: priceActionUpdate, wantInput: 0.3,
		},
		{
			name:   "source id is matched case-insensitively",
			source: "OpenRouter", matches: []modelsdev.Match{priced("alibaba-cn", 0.1), priced("openrouter", 0.3)},
			wantAction: priceActionUpdate, wantInput: 0.3,
		},
		{
			name:   "source that does not carry the model is unmatched",
			source: "deepseek", matches: []modelsdev.Match{priced("alibaba-cn", 0.1)},
			wantAction: priceActionUnmatched,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{modelsDev: &stubCatalog{lookups: map[string]modelsdev.Lookup{
				model.ModelID: {Matches: tt.matches},
			}}}
			item, err := h.planPriceSync(context.Background(), store.Provider{}, model, false, tt.source)
			if err != nil {
				t.Fatal(err)
			}
			if item.Action != tt.wantAction {
				t.Fatalf("action = %q, want %q", item.Action, tt.wantAction)
			}
			if tt.wantInput > 0 {
				if item.New == nil || item.New.Cost.Input != tt.wantInput {
					t.Fatalf("new cost = %+v, want input %v", item.New, tt.wantInput)
				}
			}
		})
	}
}

func TestPriceSourcesRankByCoverage(t *testing.T) {
	// Two models; both are under source "a", only one under source "b".
	catalog := &stubCatalog{lookups: map[string]modelsdev.Lookup{
		"m1": {Matches: []modelsdev.Match{matchAt("a", "m1", 2), matchAt("b", "m1", 2)}},
		"m2": {Matches: []modelsdev.Match{matchAt("a", "m2", 2)}},
	}}
	h := &Handler{modelsDev: catalog}
	sources, err := h.priceSources(context.Background(), []store.ProviderModel{
		pricedModel("m1", 0, 0), pricedModel("m2", 0, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 {
		t.Fatalf("sources = %+v, want 2", sources)
	}
	if sources[0].ProviderID != "a" || sources[0].Models != 2 {
		t.Fatalf("first source = %+v, want a covering 2 models", sources[0])
	}
	if sources[1].ProviderID != "b" || sources[1].Models != 1 {
		t.Fatalf("second source = %+v, want b covering 1 model", sources[1])
	}
}
