package modelsdev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gitsang/aigw/internal/store"
)

// fixtureCatalog mimics https://models.dev/api.json for the tests.
const fixtureCatalog = `{
  "alibaba-cn": {
    "id": "alibaba-cn",
    "name": "Alibaba Cloud (CN)",
    "models": {
      "qwen3.8-flash": {
        "id": "qwen3.8-flash",
        "name": "Qwen3.8 Flash",
        "cost": {
          "input": 0.1,
          "output": 0.4,
          "cache_read": 0.01,
          "cache_write": 0,
          "tiers": [
            {"input": 0.3, "output": 1.2, "cache_read": 0.03, "cache_write": 0, "tier": {"type": "context", "size": 262144}},
            {"input": 9, "output": 9, "cache_read": 9, "cache_write": 9, "tier": {"type": "context", "size": 0}},
            {"input": 9, "output": 9, "cache_read": 9, "cache_write": 9, "tier": {"type": "batch", "size": 65536}},
            {"input": 0.2, "output": 0.8, "cache_read": 0.02, "cache_write": 0, "tier": {"type": "context", "size": 131072}},
            {"input": 0.25, "output": 0.9, "cache_read": 0.025, "cache_write": 0, "tier": {"type": "context", "size": 131072}},
            {"input": 5, "output": 5, "cache_read": 5, "cache_write": 5}
          ]
        },
        "limit": {"context": 262144, "input": 131072, "output": 8192}
      }
    }
  },
  "openrouter": {
    "id": "openrouter",
    "name": "OpenRouter",
    "models": {
      "qwen3.8-flash": {
        "id": "qwen3.8-flash",
        "name": "Qwen3.8 Flash (OpenRouter)",
        "cost": {"input": 0.12, "output": 0.5},
        "limit": {"context": 131072, "output": 4096}
      },
      "openai/gpt-4o": {
        "id": "openai/gpt-4o",
        "name": "GPT-4o",
        "cost": {"input": 2.5, "output": 10, "cache_read": 1.25, "cache_write": 0},
        "limit": {"context": 128000, "output": 16384}
      }
    }
  },
  "acme": {
    "id": "acme",
    "name": "Acme",
    "models": {
      "bar": {"id": "bar", "name": "Bar", "cost": {"input": 1, "output": 2}},
      "foo/bar": {"id": "foo/bar", "name": "Bar via Foo", "cost": {"input": 2, "output": 4}}
    }
  },
  "moonshot": {
    "id": "moonshot",
    "name": "Moonshot",
    "models": {
      "kimi/kimi-k2.7-code": {
        "id": "kimi/kimi-k2.7-code",
        "name": "Kimi K2.7 Code",
        "cost": {"input": 0.6, "output": 2.4},
        "limit": {"context": 262144, "output": 32768}
      }
    }
  },
  "limits-r-us": {
    "id": "limits-r-us",
    "name": "Limits R Us",
    "models": {
      "zmodel": {"id": "zmodel", "name": "", "cost": {}, "limit": {"context": 100, "output": 50}}
    }
  }
}`

// fixtureServer serves body for every request and counts the requests.
func fixtureServer(t *testing.T, body string) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func newFixtureClient(t *testing.T) (*Client, *atomic.Int64) {
	t.Helper()
	srv, calls := fixtureServer(t, fixtureCatalog)
	return NewClient(srv.URL, time.Hour), calls
}

func mustLookup(t *testing.T, client *Client, providerSlug, providerName, modelID string) Lookup {
	t.Helper()
	got, err := client.Lookup(context.Background(), providerSlug, providerName, modelID)
	if err != nil {
		t.Fatalf("Lookup(%q, %q, %q): %v", providerSlug, providerName, modelID, err)
	}
	return got
}

func providerIDs(matches []Match) []string {
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match.ProviderID)
	}
	return ids
}

func TestLookupProviderSlugRanking(t *testing.T) {
	client, _ := newFixtureClient(t)

	for _, tc := range []struct {
		name string
		slug string
	}{
		{name: "exact id", slug: "alibaba-cn"},
		{name: "case insensitive", slug: "Alibaba-CN"},
		{name: "trimmed", slug: "  alibaba-cn  "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mustLookup(t, client, tc.slug, "", "qwen3.8-flash")
			want := []string{"alibaba-cn", "openrouter"}
			if !reflect.DeepEqual(providerIDs(got.Matches), want) {
				t.Fatalf("providers = %v, want %v", providerIDs(got.Matches), want)
			}
			if got.Matches[0].Score != ScoreProviderID {
				t.Fatalf("best score = %d, want %d", got.Matches[0].Score, ScoreProviderID)
			}
			if got.Matches[0].ProviderName != "Alibaba Cloud (CN)" {
				t.Fatalf("best provider name = %q", got.Matches[0].ProviderName)
			}
			if got.Matches[0].ModelID != "qwen3.8-flash" {
				t.Fatalf("model id = %q", got.Matches[0].ModelID)
			}
			if got.Matches[1].Score != ScoreModelID {
				t.Fatalf("runner-up score = %d, want %d", got.Matches[1].Score, ScoreModelID)
			}
			if got.Matches[0].Cost.Input != 0.1 {
				t.Fatalf("cost input = %v, want 0.1", got.Matches[0].Cost.Input)
			}
		})
	}
}

func TestLookupProviderNameRanking(t *testing.T) {
	client, _ := newFixtureClient(t)

	t.Run("name match outranks model-only match", func(t *testing.T) {
		got := mustLookup(t, client, "", "OpenRouter", "qwen3.8-flash")
		want := []string{"openrouter", "alibaba-cn"}
		if !reflect.DeepEqual(providerIDs(got.Matches), want) {
			t.Fatalf("providers = %v, want %v", providerIDs(got.Matches), want)
		}
		if got.Matches[0].Score != ScoreProviderName {
			t.Fatalf("best score = %d, want %d", got.Matches[0].Score, ScoreProviderName)
		}
		if got.Matches[1].Score != ScoreModelID {
			t.Fatalf("runner-up score = %d, want %d", got.Matches[1].Score, ScoreModelID)
		}
	})

	t.Run("name match is case insensitive and trimmed", func(t *testing.T) {
		got := mustLookup(t, client, "", "  openROUTER  ", "qwen3.8-flash")
		if len(got.Matches) == 0 || got.Matches[0].ProviderID != "openrouter" || got.Matches[0].Score != ScoreProviderName {
			t.Fatalf("matches = %+v", got.Matches)
		}
	})

	t.Run("slug match outranks name match", func(t *testing.T) {
		got := mustLookup(t, client, "alibaba-cn", "OpenRouter", "qwen3.8-flash")
		want := []string{"alibaba-cn", "openrouter"}
		if !reflect.DeepEqual(providerIDs(got.Matches), want) {
			t.Fatalf("providers = %v, want %v", providerIDs(got.Matches), want)
		}
		if got.Matches[0].Score != ScoreProviderID {
			t.Fatalf("slug score = %d, want %d", got.Matches[0].Score, ScoreProviderID)
		}
		if got.Matches[1].Score != ScoreProviderName {
			t.Fatalf("name score = %d, want %d", got.Matches[1].Score, ScoreProviderName)
		}
	})

	t.Run("unknown provider cannot score", func(t *testing.T) {
		got := mustLookup(t, client, "unknown-slug", "Unknown Provider", "qwen3.8-flash")
		want := []string{"alibaba-cn", "openrouter"}
		if !reflect.DeepEqual(providerIDs(got.Matches), want) {
			t.Fatalf("providers = %v, want %v", providerIDs(got.Matches), want)
		}
		for _, match := range got.Matches {
			if match.Score != ScoreModelID {
				t.Fatalf("provider %s score = %d, want %d", match.ProviderID, match.Score, ScoreModelID)
			}
		}
	})
}

func TestLookupModelIDOnly(t *testing.T) {
	client, _ := newFixtureClient(t)

	got := mustLookup(t, client, "", "", "qwen3.8-flash")
	want := []string{"alibaba-cn", "openrouter"}
	if !reflect.DeepEqual(providerIDs(got.Matches), want) {
		t.Fatalf("providers = %v, want %v", providerIDs(got.Matches), want)
	}
	for _, match := range got.Matches {
		if match.Score != ScoreModelID {
			t.Fatalf("provider %s score = %d, want %d", match.ProviderID, match.Score, ScoreModelID)
		}
		if match.ModelName == "" {
			t.Fatalf("provider %s has empty model name", match.ProviderID)
		}
	}
	if got.ModelID != "qwen3.8-flash" {
		t.Fatalf("lookup model id = %q", got.ModelID)
	}
	if got.FetchedAt.IsZero() {
		t.Fatal("fetched_at must be set after a lookup")
	}
}

func TestLookupNoMatch(t *testing.T) {
	client, calls := newFixtureClient(t)

	got := mustLookup(t, client, "alibaba-cn", "Alibaba Cloud (CN)", "does-not-exist")
	if got.Matches == nil {
		t.Fatal("matches must be non-nil")
	}
	if len(got.Matches) != 0 {
		t.Fatalf("matches = %+v, want none", got.Matches)
	}
	if got.ModelID != "does-not-exist" {
		t.Fatalf("lookup model id = %q", got.ModelID)
	}
	if got.FetchedAt.IsZero() {
		t.Fatal("fetched_at must be set after a lookup")
	}
	if calls.Load() != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls.Load())
	}

	// A blank model id normalizes to "" and matches nothing, but is not an error.
	blank := mustLookup(t, client, "", "", "   ")
	if blank.ModelID != "" || len(blank.Matches) != 0 || blank.Matches == nil {
		t.Fatalf("blank lookup = %+v", blank)
	}
}

func TestLookupNormalization(t *testing.T) {
	client, _ := newFixtureClient(t)

	for _, tc := range []struct {
		name         string
		modelID      string
		wantModelID  string
		wantProvider string
	}{
		{name: "leading models prefix", modelID: "models/openai/gpt-4o", wantModelID: "gpt-4o", wantProvider: "openrouter"},
		{name: "vendor prefix and case", modelID: "OpenAI/GPT-4o", wantModelID: "gpt-4o", wantProvider: "openrouter"},
		{name: "last path segment", modelID: "openai/gpt-4o", wantModelID: "gpt-4o", wantProvider: "openrouter"},
		{name: "surrounding spaces", modelID: "  gpt-4o\t", wantModelID: "gpt-4o", wantProvider: "openrouter"},
		{name: "catalog id keeps vendor prefix", modelID: "kimi-k2.7-code", wantModelID: "kimi-k2.7-code", wantProvider: "moonshot"},
		{name: "lookup with vendor prefix", modelID: "kimi/kimi-k2.7-code", wantModelID: "kimi-k2.7-code", wantProvider: "moonshot"},
		{name: "models prefix on vendor path", modelID: "models/kimi/kimi-k2.7-code", wantModelID: "kimi-k2.7-code", wantProvider: "moonshot"},
		{name: "duplicate normalized id", modelID: "foo/bar", wantModelID: "bar", wantProvider: "acme"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mustLookup(t, client, "", "", tc.modelID)
			if len(got.Matches) != 1 {
				t.Fatalf("matches = %+v, want exactly 1", got.Matches)
			}
			if got.Matches[0].ProviderID != tc.wantProvider {
				t.Fatalf("provider = %q, want %q", got.Matches[0].ProviderID, tc.wantProvider)
			}
			if got.Matches[0].ModelID != tc.wantModelID {
				t.Fatalf("model id = %q, want %q", got.Matches[0].ModelID, tc.wantModelID)
			}
			if got.ModelID != tc.wantModelID {
				t.Fatalf("lookup model id = %q, want %q", got.ModelID, tc.wantModelID)
			}
		})
	}
}

func TestLookupDedupesProviderModel(t *testing.T) {
	client, _ := newFixtureClient(t)

	// "bar" and "foo/bar" normalize to the same id for provider acme; the
	// sorted-first raw id ("bar") wins deterministically.
	for _, modelID := range []string{"bar", "foo/bar", "Bar"} {
		got := mustLookup(t, client, "acme", "Acme", modelID)
		if len(got.Matches) != 1 {
			t.Fatalf("Lookup(%q) matches = %+v, want exactly 1", modelID, got.Matches)
		}
		if got.Matches[0].Cost.Input != 1 {
			t.Fatalf("Lookup(%q) matched cost input = %v, want 1 (the sorted-first id)", modelID, got.Matches[0].Cost.Input)
		}
		if got.Matches[0].ModelName != "Bar" {
			t.Fatalf("Lookup(%q) model name = %q, want %q", modelID, got.Matches[0].ModelName, "Bar")
		}
	}
}

func TestLookupTierMapping(t *testing.T) {
	client, _ := newFixtureClient(t)

	got := mustLookup(t, client, "alibaba-cn", "", "qwen3.8-flash")
	match := got.Matches[0]

	if match.Cost.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", match.Cost.Currency)
	}
	if match.Cost.Input != 0.1 || match.Cost.Output != 0.4 || match.Cost.CacheRead != 0.01 || match.Cost.CacheWrite != 0 {
		t.Fatalf("flat cost = %+v", match.Cost)
	}

	want := []store.CostTier{
		{InputTokensAbove: 131072, Input: 0.2, Output: 0.8, CacheRead: 0.02, CacheWrite: 0},
		{InputTokensAbove: 262144, Input: 0.3, Output: 1.2, CacheRead: 0.03, CacheWrite: 0},
	}
	if !reflect.DeepEqual(match.Cost.Tiers, want) {
		t.Fatalf("tiers = %s, want %s", jsonText(t, match.Cost.Tiers), jsonText(t, want))
	}
}

func TestLookupLimitsMapping(t *testing.T) {
	client, _ := newFixtureClient(t)

	got := mustLookup(t, client, "limits-r-us", "Limits R Us", "zmodel")
	match := got.Matches[0]

	if match.ContextWindow != 100 {
		t.Fatalf("context window = %d, want 100", match.ContextWindow)
	}
	if match.MaxOutputTokens != 50 {
		t.Fatalf("max output tokens = %d, want 50", match.MaxOutputTokens)
	}
	// An empty name falls back to the normalized model id.
	if match.ModelName != "zmodel" {
		t.Fatalf("model name = %q, want zmodel", match.ModelName)
	}
	// Zero prices are valid and still carry the USD currency and a non-nil tier list.
	if match.Cost.Currency != "USD" || match.Cost.Tiers == nil || len(match.Cost.Tiers) != 0 {
		t.Fatalf("cost = %+v", match.Cost)
	}

	qwen := mustLookup(t, client, "alibaba-cn", "", "qwen3.8-flash").Matches[0]
	if qwen.ContextWindow != 262144 || qwen.MaxOutputTokens != 8192 {
		t.Fatalf("qwen limits = (%d, %d)", qwen.ContextWindow, qwen.MaxOutputTokens)
	}
}

func TestLookupCapsMatchesAtTen(t *testing.T) {
	payload := make(map[string]any, 12)
	for i := 1; i <= 12; i++ {
		id := fmt.Sprintf("p%02d", i)
		payload[id] = map[string]any{
			"id":   id,
			"name": "Provider " + id,
			"models": map[string]any{
				"shared-model": map[string]any{
					"id":   "shared-model",
					"name": "Shared Model",
					"cost": map[string]any{"input": i, "output": i * 2},
				},
			},
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	srv, calls := fixtureServer(t, string(body))
	client := NewClient(srv.URL, time.Hour)

	got := mustLookup(t, client, "", "", "shared-model")
	if len(got.Matches) != maxMatches {
		t.Fatalf("matches = %d, want %d", len(got.Matches), maxMatches)
	}
	for i, match := range got.Matches {
		want := fmt.Sprintf("p%02d", i+1)
		if match.ProviderID != want {
			t.Fatalf("match %d provider = %q, want %q (score asc, provider id asc)", i, match.ProviderID, want)
		}
		if match.Score != ScoreModelID {
			t.Fatalf("match %d score = %d, want %d", i, match.Score, ScoreModelID)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls.Load())
	}
}

func TestFetchRequest(t *testing.T) {
	var method, accept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, accept = r.Method, r.Header.Get("Accept")
		fmt.Fprint(w, fixtureCatalog)
	}))
	t.Cleanup(srv.Close)

	mustLookup(t, NewClient(srv.URL, time.Hour), "", "", "gpt-4o")
	if method != http.MethodGet {
		t.Fatalf("method = %q, want GET", method)
	}
	if accept != "application/json" {
		t.Fatalf("Accept = %q, want application/json", accept)
	}
}

func TestLookupFetchError(t *testing.T) {
	t.Run("non-2xx status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "upstream exploded", http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)
		client := NewClient(srv.URL, time.Hour)

		_, err := client.Lookup(context.Background(), "alibaba-cn", "", "qwen3.8-flash")
		if !errors.Is(err, ErrFetch) {
			t.Fatalf("error = %v, want ErrFetch", err)
		}
		if !strings.Contains(err.Error(), "500") {
			t.Fatalf("error = %v, want status code in message", err)
		}
		if !client.FetchedAt().IsZero() {
			t.Fatalf("failed fetch must not update fetched_at, got %v", client.FetchedAt())
		}
	})

	t.Run("decode error", func(t *testing.T) {
		srv, _ := fixtureServer(t, `{not-json`)
		client := NewClient(srv.URL, time.Hour)

		_, err := client.Lookup(context.Background(), "", "", "gpt-4o")
		if !errors.Is(err, ErrFetch) {
			t.Fatalf("error = %v, want ErrFetch", err)
		}
		if !client.FetchedAt().IsZero() {
			t.Fatalf("failed fetch must not update fetched_at, got %v", client.FetchedAt())
		}
	})

	t.Run("transport error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, fixtureCatalog)
		}))
		url := srv.URL
		srv.Close()

		_, err := NewClient(url, time.Hour).Lookup(context.Background(), "", "", "gpt-4o")
		if !errors.Is(err, ErrFetch) {
			t.Fatalf("error = %v, want ErrFetch", err)
		}
	})
}

func TestLookupFailedFetchDoesNotPoisonCache(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, fixtureCatalog)
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, time.Hour)

	if _, err := client.Lookup(context.Background(), "", "", "qwen3.8-flash"); !errors.Is(err, ErrFetch) {
		t.Fatalf("error = %v, want ErrFetch", err)
	}
	if !client.FetchedAt().IsZero() {
		t.Fatalf("failed fetch must not be cached, got fetched_at %v", client.FetchedAt())
	}

	fail.Store(false)
	got := mustLookup(t, client, "", "", "qwen3.8-flash")
	if len(got.Matches) != 2 {
		t.Fatalf("matches = %+v, want 2", got.Matches)
	}
	if calls.Load() != 2 {
		t.Fatalf("fetch calls = %d, want 2", calls.Load())
	}
}

func TestLookupCacheHit(t *testing.T) {
	client, calls := newFixtureClient(t)

	if got := client.FetchedAt(); !got.IsZero() {
		t.Fatalf("fetched_at before first fetch = %v, want zero", got)
	}

	first := mustLookup(t, client, "alibaba-cn", "", "qwen3.8-flash")
	if calls.Load() != 1 {
		t.Fatalf("fetch calls after first lookup = %d, want 1", calls.Load())
	}
	firstAt := client.FetchedAt()
	if firstAt.IsZero() {
		t.Fatal("fetched_at must be set after a successful fetch")
	}
	if !first.FetchedAt.Equal(firstAt) {
		t.Fatalf("lookup fetched_at = %v, client fetched_at = %v", first.FetchedAt, firstAt)
	}

	second := mustLookup(t, client, "openrouter", "OpenRouter", "qwen3.8-flash")
	if calls.Load() != 1 {
		t.Fatalf("fetch calls after cached lookup = %d, want 1", calls.Load())
	}
	if !second.FetchedAt.Equal(firstAt) {
		t.Fatalf("cached lookup fetched_at = %v, want %v", second.FetchedAt, firstAt)
	}
	if !reflect.DeepEqual(second.Matches, mustLookup(t, client, "openrouter", "OpenRouter", "qwen3.8-flash").Matches) {
		t.Fatal("repeated lookups must return identical matches")
	}
}

func TestLookupTTLExpiry(t *testing.T) {
	client, calls := newFixtureClient(t)

	mustLookup(t, client, "", "", "qwen3.8-flash")
	if calls.Load() != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls.Load())
	}
	firstAt := client.FetchedAt()

	// Force the cached catalog to look older than the TTL.
	client.mu.Lock()
	client.fetchedAt = time.Now().Add(-2 * time.Hour)
	client.mu.Unlock()

	got := mustLookup(t, client, "", "", "qwen3.8-flash")
	if calls.Load() != 2 {
		t.Fatalf("fetch calls after TTL expiry = %d, want 2", calls.Load())
	}
	if len(got.Matches) != 2 {
		t.Fatalf("matches = %+v, want 2", got.Matches)
	}
	if refreshedAt := client.FetchedAt(); !refreshedAt.After(firstAt) {
		t.Fatalf("fetched_at after refresh = %v, want after %v", refreshedAt, firstAt)
	}
}

func TestLookupConcurrent(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond) // widen the race window
		fmt.Fprint(w, fixtureCatalog)
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, time.Hour)

	runLookupWave(t, client, 16)
	if calls.Load() != 1 {
		t.Fatalf("concurrent cold lookups fetched %d times, want 1", calls.Load())
	}

	// Expire the cache and race a second wave: still only one refetch.
	client.mu.Lock()
	client.fetchedAt = time.Now().Add(-2 * time.Hour)
	client.mu.Unlock()

	runLookupWave(t, client, 16)
	if calls.Load() != 2 {
		t.Fatalf("concurrent refresh fetched %d times in total, want 2", calls.Load())
	}
}

func runLookupWave(t *testing.T, client *Client, n int) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := client.Lookup(context.Background(), "alibaba-cn", "", "qwen3.8-flash")
			if err != nil {
				errs <- err
				return
			}
			if len(got.Matches) != 2 {
				errs <- fmt.Errorf("matches = %d, want 2", len(got.Matches))
				return
			}
			if got.Matches[0].ProviderID != "alibaba-cn" || got.Matches[0].Score != ScoreProviderID {
				errs <- fmt.Errorf("best match = %+v", got.Matches[0])
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestJSONShape(t *testing.T) {
	client, _ := newFixtureClient(t)

	// Empty matches must encode as [] (not null) for the admin API.
	empty := mustLookup(t, client, "", "", "does-not-exist")
	raw, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal lookup: %v", err)
	}
	if !strings.Contains(string(raw), `"matches":[]`) {
		t.Fatalf("empty matches JSON = %s, want \"matches\":[]", raw)
	}

	// Match field names are part of the frozen contract.
	raw, err = json.Marshal(Match{})
	if err != nil {
		t.Fatalf("marshal match: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal match: %v", err)
	}
	for _, key := range []string{
		"provider_id", "provider_name", "model_id", "model_name",
		"cost", "context_window", "max_output_tokens", "score",
	} {
		if _, ok := fields[key]; !ok {
			t.Errorf("match JSON is missing %q: %s", key, raw)
		}
	}
}

// jsonText renders a value for readable test failure messages.
func jsonText(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return string(raw)
}
