package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Model catalog entry kinds.
const (
	ModelKindRoute    = "route"
	ModelKindProvider = "provider"
)

// ModelEntry is one model exposed by the gateway: either a virtual model backed
// by a routing rule, or an upstream model reachable as "{providerId}/{modelId}".
type ModelEntry struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Enabled   bool      `json:"enabled"`
	APITypes  []string  `json:"api_types"`
	CreatedAt time.Time `json:"created_at"`

	// Routing rule backed model (kind = route).
	RouteID     string `json:"route_id,omitempty"`
	RouteName   string `json:"route_name,omitempty"`
	TargetCount int    `json:"target_count,omitempty"`

	// Provider backed model (kind = provider).
	ProviderID      string     `json:"provider_id,omitempty"`
	ProviderSlug    string     `json:"provider_slug,omitempty"`
	ProviderName    string     `json:"provider_name,omitempty"`
	ModelID         string     `json:"model_id,omitempty"`
	DisplayName     string     `json:"display_name,omitempty"`
	Source          string     `json:"source,omitempty"`
	ProviderEnabled bool       `json:"provider_enabled,omitempty"`
	ModelEnabled    bool       `json:"model_enabled,omitempty"`
	ContextWindow   int        `json:"context_window,omitempty"`
	MaxOutputTokens int        `json:"max_output_tokens,omitempty"`
	Cost            *ModelCost `json:"cost,omitempty"`
}

// providerModelID builds the public "{providerId}/{modelId}" identifier. The
// provider slug is the stable id shown in the console; the name is a fallback
// for rows created before slugs were introduced.
func providerModelID(providerSlug, providerName, modelID string) string {
	prefix := strings.TrimSpace(providerSlug)
	if prefix == "" {
		prefix = strings.TrimSpace(providerName)
	}
	if prefix == "" {
		return modelID
	}
	return prefix + "/" + modelID
}

// providerModelName builds the public "{providerName}/{modelName}" display name.
func providerModelName(providerName, displayName, modelID string) string {
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = modelID
	}
	providerName = strings.TrimSpace(providerName)
	if providerName == "" {
		return name
	}
	return providerName + "/" + name
}

// effectiveModelAPITypes mirrors the routing resolution order: model protocols,
// then provider protocols, then the built-in defaults.
func effectiveModelAPITypes(modelTypes, providerTypes []string) []string {
	if len(modelTypes) > 0 {
		return modelTypes
	}
	if len(providerTypes) > 0 {
		return providerTypes
	}
	return DefaultAPITypes()
}

// ListModelCatalog returns every model the gateway can serve: routing-rule
// virtual models first, then each provider's models ordered by provider and
// model id.
func (s *Store) ListModelCatalog(ctx context.Context) ([]ModelEntry, error) {
	out, err := s.listRouteModelEntries(ctx)
	if err != nil {
		return nil, err
	}
	providerEntries, err := s.listProviderModelEntries(ctx)
	if err != nil {
		return nil, err
	}
	return append(out, providerEntries...), nil
}

func (s *Store) listRouteModelEntries(ctx context.Context) ([]ModelEntry, error) {
	rows, err := s.pool.Query(ctx, `
SELECT r.id, r.name, r.virtual_model, r.api_types, r.enabled, r.created_at,
       (SELECT COUNT(*) FROM routing_targets t WHERE t.rule_id = r.id)
FROM routing_rules r ORDER BY r.created_at ASC, r.virtual_model ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ModelEntry, 0)
	for rows.Next() {
		e := ModelEntry{Kind: ModelKindRoute}
		if err := rows.Scan(&e.RouteID, &e.RouteName, &e.ID, &e.APITypes, &e.Enabled,
			&e.CreatedAt, &e.TargetCount); err != nil {
			return nil, err
		}
		if e.APITypes == nil {
			e.APITypes = []string{}
		}
		e.Name = e.ID
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) listProviderModelEntries(ctx context.Context) ([]ModelEntry, error) {
	rows, err := s.pool.Query(ctx, `
SELECT pm.id, p.id, p.slug, p.name, pm.model_id, pm.display_name, pm.source,
       pm.api_types, p.api_types, pm.enabled, p.enabled,
       pm.context_window, pm.max_output_tokens,
       pm.cost_currency, pm.cost_input::float8, pm.cost_output::float8,
       pm.cost_cache_read::float8, pm.cost_cache_write::float8, pm.cost_tiers,
       pm.created_at
FROM provider_models pm
JOIN providers p ON p.id = pm.provider_id
ORDER BY p.created_at ASC, pm.model_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ModelEntry, 0)
	for rows.Next() {
		var (
			modelPK, providerID, slug, providerName, modelID, displayName, source string
			modelTypes, providerTypes                                             []string
			modelEnabled, providerEnabled                                         bool
			contextWindow, maxOutputTokens                                        int
			cost                                                                  ModelCost
			tiersRaw                                                              []byte
			createdAt                                                             time.Time
		)
		if err := rows.Scan(&modelPK, &providerID, &slug, &providerName, &modelID,
			&displayName, &source, &modelTypes, &providerTypes, &modelEnabled, &providerEnabled,
			&contextWindow, &maxOutputTokens,
			&cost.Currency, &cost.Input, &cost.Output, &cost.CacheRead, &cost.CacheWrite,
			&tiersRaw, &createdAt); err != nil {
			return nil, err
		}
		cost.Tiers = []CostTier{}
		if len(tiersRaw) > 0 {
			_ = json.Unmarshal(tiersRaw, &cost.Tiers)
		}
		if cost.Currency == "" {
			cost.Currency = "USD"
		}
		entry := ModelEntry{
			ID:              providerModelID(slug, providerName, modelID),
			Name:            providerModelName(providerName, displayName, modelID),
			Kind:            ModelKindProvider,
			Enabled:         modelEnabled && providerEnabled,
			APITypes:        effectiveModelAPITypes(modelTypes, providerTypes),
			CreatedAt:       createdAt,
			ProviderID:      providerID,
			ProviderSlug:    slug,
			ProviderName:    providerName,
			ModelID:         modelID,
			DisplayName:     displayName,
			Source:          source,
			ProviderEnabled: providerEnabled,
			ModelEnabled:    modelEnabled,
			ContextWindow:   contextWindow,
			MaxOutputTokens: maxOutputTokens,
			Cost:            &cost,
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// ModelMatch is an enabled provider model resolved from a client-supplied model
// name by the auto model matching fallback.
type ModelMatch struct {
	ProviderID      string `json:"provider_id"`
	ProviderSlug    string `json:"provider_slug"`
	ProviderName    string `json:"provider_name"`
	ProviderModelID string `json:"provider_model_id"`
	ModelID         string `json:"model_id"`
}

// ID is the public model identifier: "{providerId}/{modelId}".
func (m ModelMatch) ID() string {
	return providerModelID(m.ProviderSlug, m.ProviderName, m.ModelID)
}

// MatchProviderModel picks the first enabled provider model whose public
// "{providerId}/{modelId}" identifier contains the requested model name.
func (s *Store) MatchProviderModel(ctx context.Context, query string) (ModelMatch, error) {
	candidates, err := s.listModelMatchCandidates(ctx)
	if err != nil {
		return ModelMatch{}, err
	}
	match, ok := matchProviderModel(candidates, query)
	if !ok {
		return ModelMatch{}, ErrNotFound
	}
	return match, nil
}

func (s *Store) listModelMatchCandidates(ctx context.Context) ([]ModelMatch, error) {
	rows, err := s.pool.Query(ctx, `
SELECT pm.id, p.id, p.slug, p.name, pm.model_id
FROM provider_models pm
JOIN providers p ON p.id = pm.provider_id
WHERE pm.enabled = TRUE AND p.enabled = TRUE
ORDER BY p.created_at ASC, pm.model_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ModelMatch, 0)
	for rows.Next() {
		var m ModelMatch
		if err := rows.Scan(&m.ProviderModelID, &m.ProviderID, &m.ProviderSlug,
			&m.ProviderName, &m.ModelID); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// matchProviderModel implements the "first model containing the requested name"
// rule. Matching is case-insensitive and considers the public identifier only.
func matchProviderModel(candidates []ModelMatch, query string) (ModelMatch, bool) {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return ModelMatch{}, false
	}
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate.ID()), needle) {
			return candidate, true
		}
	}
	return ModelMatch{}, false
}

// ResolveModelTarget loads one provider model as a standalone runtime target.
// It lets the gateway dispatch a request to a matched model without a routing
// rule; provider-level protocol, path, retry and proxy settings still apply.
func (s *Store) ResolveModelTarget(ctx context.Context, providerModelID, inboundAPIType string) (RuntimeTarget, error) {
	var t RuntimeTarget
	var apiKeyCT, proxyCT string
	var headersRaw, providerOverridesRaw, modelOverridesRaw, tiersRaw []byte
	var modelID string
	var modelTypes, providerTypes []string
	err := s.pool.QueryRow(ctx, `
SELECT pm.id, p.id, p.name, p.base_url, p.auth_type, p.api_key_ciphertext, p.headers,
       p.timeout_ms, pm.model_id,
       pm.api_types, p.api_types, p.path_overrides, pm.path_overrides,
       pm.cost_currency, pm.cost_input::float8, pm.cost_output::float8,
       pm.cost_cache_read::float8, pm.cost_cache_write::float8, pm.cost_tiers,
       p.proxy_type, p.proxy_url, p.proxy_username, p.proxy_password_ciphertext,
       p.retry_count, p.retry_backoff, p.retry_initial_ms, p.retry_max_ms,
       p.retry_linear_interval_ms
FROM provider_models pm
JOIN providers p ON p.id = pm.provider_id
WHERE pm.id = $1 AND pm.enabled = TRUE AND p.enabled = TRUE`, providerModelID).Scan(
		&t.TargetID, &t.ProviderID, &t.ProviderName, &t.BaseURL, &t.AuthType, &apiKeyCT,
		&headersRaw, &t.TimeoutMS, &modelID,
		&modelTypes, &providerTypes, &providerOverridesRaw, &modelOverridesRaw,
		&t.Cost.Currency, &t.Cost.Input, &t.Cost.Output,
		&t.Cost.CacheRead, &t.Cost.CacheWrite, &tiersRaw,
		&t.Proxy.Type, &t.Proxy.URL, &t.Proxy.Username, &proxyCT,
		&t.Retry.Count, &t.Retry.Backoff, &t.Retry.InitialMS, &t.Retry.MaxMS,
		&t.Retry.LinearDelayMS)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return t, ErrNotFound
		}
		return t, err
	}
	if apiKeyCT != "" {
		t.APIKey, _ = s.cipher.Decrypt(apiKeyCT)
	}
	if proxyCT != "" {
		t.Proxy.Password, _ = s.cipher.Decrypt(proxyCT)
	}
	t.Headers = decodeHeaders(headersRaw)
	t.Cost.Tiers = []CostTier{}
	if len(tiersRaw) > 0 {
		_ = json.Unmarshal(tiersRaw, &t.Cost.Tiers)
	}
	providerOverrides := map[string]string{}
	if len(providerOverridesRaw) > 0 {
		_ = json.Unmarshal(providerOverridesRaw, &providerOverrides)
	}
	modelOverrides := map[string]string{}
	if len(modelOverridesRaw) > 0 {
		_ = json.Unmarshal(modelOverridesRaw, &modelOverrides)
	}
	t.Role = RoleLB
	t.Weight = 100
	t.UpstreamModel = modelID
	t.APIType = ChooseAPIType(nil, modelTypes, providerTypes, inboundAPIType)
	t.UpstreamPath = ResolvePath(modelOverrides, providerOverrides, t.APIType)
	return t, nil
}
