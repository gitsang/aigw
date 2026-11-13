// Package modelsdev fetches the public models.dev catalog and matches its
// per-provider model pricing against the gateway's own providers and models.
//
// The catalog is a map of provider id to {id, name, models}; every model carries
// a cost (USD per 1M tokens) and token limits. The catalog is cached in memory
// and refreshed when older than the configured TTL.
package modelsdev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gitsang/aigw/internal/store"
)

// ErrFetch marks failures to fetch, read or decode the models.dev catalog.
var ErrFetch = errors.New("models.dev")

// Match confidence scores. Lower is better and therefore sorts first.
const (
	// ScoreProviderID means the provider slug equals a models.dev provider id.
	ScoreProviderID = 0
	// ScoreAPIHost means the provider base URL host equals a models.dev api host.
	ScoreAPIHost = 1
	// ScoreProviderName means the provider display name matched.
	ScoreProviderName = 2
	// ScoreModelID means only the model id matched.
	ScoreModelID = 3
)

const (
	defaultURL       = "https://models.dev/api.json"
	fetchTimeout     = 30 * time.Second
	maxResponseBytes = 32 << 20
	maxMatches       = 10
)

// Match is one models.dev entry that matched a lookup.
type Match struct {
	ProviderID      string          `json:"provider_id"`
	ProviderName    string          `json:"provider_name"`
	ModelID         string          `json:"model_id"`
	ModelName       string          `json:"model_name"`
	Cost            store.ModelCost `json:"cost"`
	ContextWindow   int             `json:"context_window"`
	MaxOutputTokens int             `json:"max_output_tokens"`
	Score           int             `json:"score"`
}

// Lookup is the result of a single model lookup. Matches is always non-nil and
// contains at most 10 entries, sorted by score and then provider id.
type Lookup struct {
	ModelID   string    `json:"model_id"`
	FetchedAt time.Time `json:"fetched_at"`
	Matches   []Match   `json:"matches"`
}

// Client fetches and caches the models.dev catalog. It is safe for concurrent
// use.
type Client struct {
	url        string
	ttl        time.Duration
	httpClient *http.Client

	mu        sync.RWMutex
	catalog   *catalog
	fetchedAt time.Time

	fetchMu sync.Mutex
}

// NewClient returns a client for the models.dev catalog at url. An empty url
// uses https://models.dev/api.json. The catalog is cached until it is older than
// ttl; a non-positive ttl disables caching.
func NewClient(url string, ttl time.Duration) *Client {
	url = strings.TrimSpace(url)
	if url == "" {
		url = defaultURL
	}
	return &Client{
		url:        url,
		ttl:        ttl,
		httpClient: &http.Client{Timeout: fetchTimeout},
	}
}

// Lookup returns the models.dev entries whose normalized model id equals the
// normalized modelID, ranked by how well the provider matches. providerSlug and
// providerName may be empty; they are only used for scoring.
func (c *Client) Lookup(ctx context.Context, providerSlug, providerName, modelID string) (Lookup, error) {
	return c.LookupProvider(ctx, Identity{Slug: providerSlug, Name: providerName}, modelID)
}

// LookupProvider is Lookup with the full provider identity, so a provider's
// base URL host can be used as an additional matching signal. It ranks catalog
// entries by provider slug, then API host, then display name, then model id
// alone.
func (c *Client) LookupProvider(ctx context.Context, identity Identity, modelID string) (Lookup, error) {
	normalized := normalizeModelID(modelID)
	cat, fetchedAt, err := c.snapshot(ctx)
	if err != nil {
		return Lookup{ModelID: normalized, Matches: []Match{}}, err
	}
	return Lookup{
		ModelID:   normalized,
		FetchedAt: fetchedAt,
		Matches:   cat.matches(identity, normalized),
	}, nil
}

// FetchedAt reports when the cached catalog was last fetched. It is zero before
// the first successful fetch.
func (c *Client) FetchedAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fetchedAt
}

// Matches returns every models.dev entry whose normalized model id equals the
// normalized modelID, sorted by provider id and without the 10-entry cap that
// Lookup applies. It lets callers enumerate all source providers for a model and
// pick one explicitly, which matters for aggregator-style providers whose name
// is not present in the catalog and whose model ids are shared by many
// providers at different prices.
func (c *Client) Matches(ctx context.Context, modelID string) ([]Match, time.Time, error) {
	normalized := normalizeModelID(modelID)
	cat, fetchedAt, err := c.snapshot(ctx)
	if err != nil {
		return []Match{}, time.Time{}, err
	}
	return cat.allMatches(normalized), fetchedAt, nil
}

// snapshot returns a fresh cached catalog, fetching one if necessary. At most
// one fetch runs at a time; a failed fetch never replaces the cached catalog.
func (c *Client) snapshot(ctx context.Context) (*catalog, time.Time, error) {
	c.mu.RLock()
	cat, fetchedAt := c.catalog, c.fetchedAt
	c.mu.RUnlock()
	if c.fresh(cat, fetchedAt) {
		return cat, fetchedAt, nil
	}

	c.fetchMu.Lock()
	defer c.fetchMu.Unlock()

	// Another goroutine may have refreshed the catalog while we waited.
	c.mu.RLock()
	cat, fetchedAt = c.catalog, c.fetchedAt
	c.mu.RUnlock()
	if c.fresh(cat, fetchedAt) {
		return cat, fetchedAt, nil
	}

	cat, err := c.fetch(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	fetchedAt = time.Now()
	c.mu.Lock()
	c.catalog = cat
	c.fetchedAt = fetchedAt
	c.mu.Unlock()
	return cat, fetchedAt, nil
}

// fresh reports whether cat was fetched recently enough to serve from cache.
func (c *Client) fresh(cat *catalog, fetchedAt time.Time) bool {
	return cat != nil && time.Since(fetchedAt) < c.ttl
}

// fetch downloads and parses the full catalog. The returned catalog is only
// published once it is complete and valid.
func (c *Client) fetch(ctx context.Context) (*catalog, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %w", ErrFetch, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed: %w", ErrFetch, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %w", ErrFetch, err)
	}
	if len(raw) > maxResponseBytes {
		return nil, fmt.Errorf("%w: response exceeds %d bytes", ErrFetch, maxResponseBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: unexpected status %d: %s", ErrFetch, resp.StatusCode, snippet(raw))
	}

	var payload map[string]apiProvider
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%w: decode catalog: %w", ErrFetch, err)
	}
	return buildCatalog(payload), nil
}

// snippet shortens a response body for inclusion in an error message.
func snippet(raw []byte) string {
	const max = 200
	s := strings.TrimSpace(string(raw))
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = s[:max] + "..."
	}
	return s
}

// normalizeModelID trims, lowercases, strips a leading "models/" prefix and
// keeps the last path segment: "models/openai/GPT-4o" -> "gpt-4o".
func normalizeModelID(id string) string {
	s := strings.ToLower(strings.TrimSpace(id))
	s = strings.TrimPrefix(s, "models/")
	s = strings.TrimRight(s, "/")
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	return strings.TrimSpace(s)
}

// Identity identifies the gateway provider being priced. Every field is
// optional; more fields make matching more precise.
type Identity struct {
	Slug    string
	Name    string
	BaseURL string
}

// normalizeHost reduces a provider base URL to a bare lowercase host name so it
// can be compared with the api fields in the models.dev catalog.
func normalizeHost(baseURL string) string {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	return host
}

// catalog is the parsed models.dev payload, indexed by normalized model id.
type catalog struct {
	byModel map[string][]catalogModel
}

type catalogModel struct {
	providerID      string
	providerName    string
	apiHost         string
	modelID         string
	modelName       string
	cost            store.ModelCost
	contextWindow   int
	maxOutputTokens int
}

// matches ranks catalog entries sharing modelID by provider match quality.
func (cat *catalog) matches(identity Identity, modelID string) []Match {
	slug := strings.TrimSpace(identity.Slug)
	name := strings.TrimSpace(identity.Name)
	host := normalizeHost(identity.BaseURL)

	matches := make([]Match, 0, len(cat.byModel[modelID]))
	for _, entry := range cat.byModel[modelID] {
		score := ScoreModelID
		switch {
		case slug != "" && strings.EqualFold(slug, entry.providerID):
			score = ScoreProviderID
		case host != "" && host == entry.apiHost:
			// The provider points at the same API endpoint models.dev
			// documents, e.g. https://api.deepseek.com. Strong evidence for
			// direct providers, but weaker than an explicit slug match.
			score = ScoreAPIHost
		case name != "" && strings.EqualFold(name, entry.providerName):
			score = ScoreProviderName
		}
		matches = append(matches, Match{
			ProviderID:      entry.providerID,
			ProviderName:    entry.providerName,
			ModelID:         entry.modelID,
			ModelName:       entry.modelName,
			Cost:            entry.cost,
			ContextWindow:   entry.contextWindow,
			MaxOutputTokens: entry.maxOutputTokens,
			Score:           score,
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score < matches[j].Score
		}
		return matches[i].ProviderID < matches[j].ProviderID
	})
	if len(matches) > maxMatches {
		matches = matches[:maxMatches]
	}
	return matches
}

// all matches returns every catalog entry sharing modelID, sorted by provider id
// and without the ranking cap. It backs explicit source selection, where the
// caller names the models.dev provider instead of relying on score ranking.
func (cat *catalog) allMatches(modelID string) []Match {
	entries := cat.byModel[modelID]
	matches := make([]Match, 0, len(entries))
	for _, entry := range entries {
		matches = append(matches, Match{
			ProviderID:      entry.providerID,
			ProviderName:    entry.providerName,
			ModelID:         entry.modelID,
			ModelName:       entry.modelName,
			Cost:            entry.cost,
			ContextWindow:   entry.contextWindow,
			MaxOutputTokens: entry.maxOutputTokens,
			Score:           ScoreModelID,
		})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].ProviderID < matches[j].ProviderID
	})
	return matches
}

// buildCatalog converts the decoded models.dev payload into a lookup index.
// Provider and model traversal is sorted so that duplicate normalized ids are
// resolved deterministically.
func buildCatalog(payload map[string]apiProvider) *catalog {
	providerKeys := make([]string, 0, len(payload))
	for key := range payload {
		providerKeys = append(providerKeys, key)
	}
	sort.Strings(providerKeys)

	byModel := make(map[string][]catalogModel)
	seen := make(map[string]struct{})
	for _, key := range providerKeys {
		provider := payload[key]
		providerID := strings.TrimSpace(provider.ID)
		if providerID == "" {
			providerID = strings.TrimSpace(key)
		}
		if providerID == "" {
			continue
		}
		providerName := strings.TrimSpace(provider.Name)
		if providerName == "" {
			providerName = providerID
		}

		modelKeys := make([]string, 0, len(provider.Models))
		for id := range provider.Models {
			modelKeys = append(modelKeys, id)
		}
		sort.Strings(modelKeys)

		for _, modelKey := range modelKeys {
			model := provider.Models[modelKey]
			rawID := strings.TrimSpace(model.ID)
			if rawID == "" {
				rawID = strings.TrimSpace(modelKey)
			}
			modelID := normalizeModelID(rawID)
			if modelID == "" {
				continue
			}
			dedupeKey := providerID + "\x00" + modelID
			if _, ok := seen[dedupeKey]; ok {
				continue
			}
			seen[dedupeKey] = struct{}{}

			modelName := strings.TrimSpace(model.Name)
			if modelName == "" {
				modelName = modelID
			}
			contextWindow, maxOutputTokens := model.limits()
			byModel[modelID] = append(byModel[modelID], catalogModel{
				providerID:      providerID,
				providerName:    providerName,
				apiHost:         normalizeHost(provider.API),
				modelID:         modelID,
				modelName:       modelName,
				cost:            model.Cost.modelCost(),
				contextWindow:   contextWindow,
				maxOutputTokens: maxOutputTokens,
			})
		}
	}
	return &catalog{byModel: byModel}
}

// modelCost maps a models.dev cost block to the gateway's price list.
func (c *apiCost) modelCost() store.ModelCost {
	cost := store.ModelCost{Currency: "USD", Tiers: []store.CostTier{}}
	if c == nil {
		return cost
	}
	cost.Input = c.Input
	cost.Output = c.Output
	cost.CacheRead = c.CacheRead
	cost.CacheWrite = c.CacheWrite
	cost.Tiers = mapTiers(c.Tiers)
	return cost
}

// mapTiers keeps context tiers with a positive threshold, sorted by threshold
// with duplicate thresholds dropped (first wins).
func mapTiers(tiers []apiTier) []store.CostTier {
	out := make([]store.CostTier, 0, len(tiers))
	for _, tier := range tiers {
		if tier.Tier == nil || tier.Tier.Type != "context" || tier.Tier.Size <= 0 {
			continue
		}
		out = append(out, store.CostTier{
			InputTokensAbove: tier.Tier.Size,
			Input:            tier.Input,
			Output:           tier.Output,
			CacheRead:        tier.CacheRead,
			CacheWrite:       tier.CacheWrite,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].InputTokensAbove < out[j].InputTokensAbove
	})
	deduped := out[:0]
	for i, tier := range out {
		if i > 0 && tier.InputTokensAbove == deduped[len(deduped)-1].InputTokensAbove {
			continue
		}
		deduped = append(deduped, tier)
	}
	return deduped
}

// limits returns the context window and max output tokens, treating a missing
// limit block as unknown (zero).
func (m *apiModel) limits() (contextWindow, maxOutputTokens int) {
	if m.Limit == nil {
		return 0, 0
	}
	return m.Limit.Context, m.Limit.Output
}

// apiProvider models one entry of the models.dev provider map.
type apiProvider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// API is the provider's OpenAI-compatible base URL, when documented.
	API    string              `json:"api"`
	Models map[string]apiModel `json:"models"`
}

// apiModel models one entry of a models.dev provider's model map.
type apiModel struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	Cost  *apiCost  `json:"cost"`
	Limit *apiLimit `json:"limit"`
}

// apiCost is a models.dev price list: USD per 1M tokens plus context tiers.
type apiCost struct {
	Input      float64   `json:"input"`
	Output     float64   `json:"output"`
	CacheRead  float64   `json:"cache_read"`
	CacheWrite float64   `json:"cache_write"`
	Tiers      []apiTier `json:"tiers"`
}

// apiTier is one price bracket; Tier describes when it applies.
type apiTier struct {
	Input      float64      `json:"input"`
	Output     float64      `json:"output"`
	CacheRead  float64      `json:"cache_read"`
	CacheWrite float64      `json:"cache_write"`
	Tier       *apiTierSpec `json:"tier"`
}

// apiTierSpec is the bracket descriptor attached to an apiTier.
type apiTierSpec struct {
	Type string `json:"type"`
	Size int    `json:"size"`
}

// apiLimit is the token limit block of a models.dev model.
type apiLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}
