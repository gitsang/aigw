package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gitsang/aigw/internal/modelsdev"
	"github.com/gitsang/aigw/internal/store"
	"github.com/go-chi/chi/v5"
)

// priceCatalog is the models.dev client surface used by the admin API. It is an
// interface so the sync planner can be unit tested without network access.
type priceCatalog interface {
	LookupProvider(ctx context.Context, identity modelsdev.Identity, modelID string) (modelsdev.Lookup, error)
	Matches(ctx context.Context, modelID string) ([]modelsdev.Match, time.Time, error)
	FetchedAt() time.Time
}

// Price sync actions reported per provider model. Every item is counted under
// exactly one of these.
const (
	priceActionUpdate       = "update"
	priceActionSkipHasPrice = "skip_has_price"
	priceActionUnmatched    = "unmatched"
	priceActionAmbiguous    = "ambiguous"
	priceActionUnchanged    = "unchanged"
	priceActionError        = "error"
)

var priceActions = []string{
	priceActionUpdate, priceActionSkipHasPrice, priceActionUnmatched,
	priceActionAmbiguous, priceActionUnchanged, priceActionError,
}

const maxPriceSources = 30

// priceSnapshot is the part of a provider model that the price sync may write.
type priceSnapshot struct {
	Cost            store.ModelCost `json:"cost"`
	ContextWindow   int             `json:"context_window"`
	MaxOutputTokens int             `json:"max_output_tokens"`
}

// priceSource is one candidate models.dev provider for a provider's model set.
// Models is how many of the provider's models are present under that source.
type priceSource struct {
	ProviderID   string `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	Models       int    `json:"models"`
}

// priceSyncItem is one row of the preview/apply response.
type priceSyncItem struct {
	ID      string           `json:"id"`
	ModelID string           `json:"model_id"`
	Action  string           `json:"action"`
	Match   *modelsdev.Match `json:"match"`
	Old     *priceSnapshot   `json:"old"`
	New     *priceSnapshot   `json:"new"`
	Error   string           `json:"error,omitempty"`
}

type priceSyncResponse struct {
	DryRun bool `json:"dry_run"`
	// SourceProvider echoes the explicit models.dev provider the caller selected
	// ("" means the lookup ranked providers by slug/name).
	SourceProvider string    `json:"source_provider"`
	FetchedAt      time.Time `json:"fetched_at"`
	// Sources lists the models.dev providers that cover the most of this
	// provider's models, so the console can offer an explicit choice.
	Sources []priceSource   `json:"sources"`
	Items   []priceSyncItem `json:"items"`
	Counts  map[string]int  `json:"counts"`
}

type priceSyncRequest struct {
	DryRun    *bool `json:"dry_run"`
	Overwrite bool  `json:"overwrite"`
	// SourceProvider picks a specific models.dev provider, by its catalog id.
	// Needed when the provider is an aggregator: models.dev then lists the same
	// model id under many providers at different prices.
	SourceProvider string `json:"source_provider"`
}

// lookupMatches serves the per-model price lookup used by the model editor.
func (h *Handler) lookupModelPrices(w http.ResponseWriter, r *http.Request) {
	modelID := strings.TrimSpace(r.URL.Query().Get("model_id"))
	if modelID == "" {
		writeError(w, http.StatusBadRequest, "model_id is required")
		return
	}
	if h.modelsDev == nil {
		writeError(w, http.StatusServiceUnavailable, "models.dev 同步未启用")
		return
	}
	providerID := chi.URLParam(r, "id")
	provider, err := h.store.GetProvider(r.Context(), providerID)
	if err != nil {
		h.fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	lookup, err := h.modelsDev.LookupProvider(ctx, providerIdentity(provider), modelID)
	if err != nil {
		h.failModelsDev(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lookup)
}

// syncModelPrices previews (dry run, the default) or applies the models.dev
// price catalog to every model of a provider.
func (h *Handler) syncModelPrices(w http.ResponseWriter, r *http.Request) {
	var req priceSyncRequest
	if ok := decodeOptional(w, r, &req); !ok {
		return
	}
	dryRun := req.DryRun == nil || *req.DryRun
	source := strings.TrimSpace(req.SourceProvider)
	if h.modelsDev == nil {
		writeError(w, http.StatusServiceUnavailable, "models.dev 同步未启用")
		return
	}
	providerID := chi.URLParam(r, "id")
	provider, err := h.store.GetProvider(r.Context(), providerID)
	if err != nil {
		h.fail(w, err)
		return
	}
	models, err := h.store.ListProviderModels(r.Context(), providerID)
	if err != nil {
		h.fail(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	resp := priceSyncResponse{
		DryRun:         dryRun,
		SourceProvider: source,
		Items:          make([]priceSyncItem, 0, len(models)),
		Counts:         make(map[string]int, len(priceActions)),
	}
	for _, action := range priceActions {
		resp.Counts[action] = 0
	}
	for _, model := range models {
		item, err := h.planPriceSync(ctx, provider, model, req.Overwrite, source)
		if err != nil {
			h.failModelsDev(w, err)
			return
		}
		if !dryRun && item.Action == priceActionUpdate {
			if err := h.store.UpdateProviderModelPricing(ctx, providerID, model.ID,
				item.New.Cost, item.New.ContextWindow, item.New.MaxOutputTokens); err != nil {
				item.Action = priceActionError
				item.Error = err.Error()
			}
		}
		resp.Counts[item.Action]++
		resp.Items = append(resp.Items, item)
	}
	if source == "" {
		resp.Sources, err = h.priceSources(ctx, models)
		if err != nil {
			h.failModelsDev(w, err)
			return
		}
	}
	resp.FetchedAt = h.modelsDev.FetchedAt()
	writeJSON(w, http.StatusOK, resp)
}

// priceSources ranks the models.dev providers that carry this provider's models.
// It is advisory: the console shows it as a picker when automatic matching is
// ambiguous, which is the normal case for aggregator providers.
func (h *Handler) priceSources(ctx context.Context, models []store.ProviderModel) ([]priceSource, error) {
	counts := map[string]*priceSource{}
	for _, model := range models {
		matches, _, err := h.modelsDev.Matches(ctx, model.ModelID)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			entry, ok := counts[match.ProviderID]
			if !ok {
				entry = &priceSource{ProviderID: match.ProviderID, ProviderName: match.ProviderName}
				counts[match.ProviderID] = entry
			}
			entry.Models++
		}
	}
	sources := make([]priceSource, 0, len(counts))
	for _, entry := range counts {
		sources = append(sources, *entry)
	}
	sort.SliceStable(sources, func(i, j int) bool {
		if sources[i].Models != sources[j].Models {
			return sources[i].Models > sources[j].Models
		}
		return sources[i].ProviderID < sources[j].ProviderID
	})
	if len(sources) > maxPriceSources {
		sources = sources[:maxPriceSources]
	}
	return sources, nil
}

// planPriceSync classifies one provider model without writing anything. An empty
// source ranks models.dev providers by slug/name; a non-empty source restricts
// the lookup to that provider id.
func (h *Handler) planPriceSync(ctx context.Context, provider store.Provider, model store.ProviderModel, overwrite bool, source string) (priceSyncItem, error) {
	item := priceSyncItem{
		ID:      model.ID,
		ModelID: model.ModelID,
		Old: &priceSnapshot{
			Cost: model.Cost, ContextWindow: model.ContextWindow, MaxOutputTokens: model.MaxOutputTokens,
		},
	}
	match, action, err := h.pickPriceMatch(ctx, provider, model.ModelID, source)
	if err != nil {
		return item, err
	}
	if action != "" {
		item.Action = action
		return item, nil
	}
	item.Match = &match

	next := priceSnapshot{
		Cost:            match.Cost,
		ContextWindow:   model.ContextWindow,
		MaxOutputTokens: model.MaxOutputTokens,
	}
	if overwrite {
		if match.ContextWindow > 0 {
			next.ContextWindow = match.ContextWindow
		}
		if match.MaxOutputTokens > 0 {
			next.MaxOutputTokens = match.MaxOutputTokens
		}
	} else {
		if next.ContextWindow == 0 {
			next.ContextWindow = match.ContextWindow
		}
		if next.MaxOutputTokens == 0 {
			next.MaxOutputTokens = match.MaxOutputTokens
		}
	}
	item.New = &next

	switch {
	case !overwrite && (model.Cost.Input > 0 || model.Cost.Output > 0):
		item.Action = priceActionSkipHasPrice
	case samePriceSnapshot(*item.Old, next):
		item.Action = priceActionUnchanged
	default:
		item.Action = priceActionUpdate
	}
	return item, nil
}

// pickPriceMatch resolves the single models.dev entry to use for a model. A
// non-empty action means "no match to apply" (unmatched or ambiguous).
func (h *Handler) pickPriceMatch(ctx context.Context, provider store.Provider, modelID, source string) (modelsdev.Match, string, error) {
	if source != "" {
		matches, _, err := h.modelsDev.Matches(ctx, modelID)
		if err != nil {
			return modelsdev.Match{}, "", err
		}
		chosen := make([]modelsdev.Match, 0, 1)
		for _, candidate := range matches {
			if strings.EqualFold(strings.TrimSpace(candidate.ProviderID), source) {
				chosen = append(chosen, candidate)
			}
		}
		switch len(chosen) {
		case 0:
			return modelsdev.Match{}, priceActionUnmatched, nil
		case 1:
			return chosen[0], "", nil
		default:
			return modelsdev.Match{}, priceActionAmbiguous, nil
		}
	}

	lookup, err := h.modelsDev.LookupProvider(ctx, providerIdentity(provider), modelID)
	if err != nil {
		return modelsdev.Match{}, "", err
	}
	if len(lookup.Matches) == 0 {
		return modelsdev.Match{}, priceActionUnmatched, nil
	}
	best := lookup.Matches[0].Score
	matches := make([]modelsdev.Match, 0, len(lookup.Matches))
	for _, candidate := range lookup.Matches {
		if candidate.Score == best {
			matches = append(matches, candidate)
		}
	}
	if len(matches) > 1 {
		return modelsdev.Match{}, priceActionAmbiguous, nil
	}
	return matches[0], "", nil
}

// providerIdentity builds the matching identity for the models.dev catalog.
// The base URL lets the catalog recognize direct providers by API host even when
// their slug or display name differs from the catalog's naming.
func providerIdentity(provider store.Provider) modelsdev.Identity {
	return modelsdev.Identity{
		Slug:    provider.Slug,
		Name:    provider.Name,
		BaseURL: provider.BaseURL,
	}
}

func samePriceSnapshot(a, b priceSnapshot) bool {
	if a.ContextWindow != b.ContextWindow || a.MaxOutputTokens != b.MaxOutputTokens {
		return false
	}
	if !strings.EqualFold(a.Cost.Currency, b.Cost.Currency) {
		return false
	}
	if a.Cost.Input != b.Cost.Input || a.Cost.Output != b.Cost.Output ||
		a.Cost.CacheRead != b.Cost.CacheRead || a.Cost.CacheWrite != b.Cost.CacheWrite {
		return false
	}
	if len(a.Cost.Tiers) != len(b.Cost.Tiers) {
		return false
	}
	for i := range a.Cost.Tiers {
		if a.Cost.Tiers[i] != b.Cost.Tiers[i] {
			return false
		}
	}
	return true
}

// failModelsDev maps catalog failures to a gateway-style error without leaking
// the upstream URL/HTML into the console.
func (h *Handler) failModelsDev(w http.ResponseWriter, err error) {
	if errors.Is(err, modelsdev.ErrFetch) {
		h.log.Warn("models.dev lookup failed", "error", err)
		writeError(w, http.StatusBadGateway, "models.dev 同步失败: "+err.Error())
		return
	}
	h.fail(w, err)
}

// decodeOptional decodes an optional JSON body: an empty body means "defaults".
func decodeOptional(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}
