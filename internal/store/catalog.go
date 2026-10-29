package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func IsSupportedAPIType(name string) bool {
	switch name {
	case APIChatCompletions, APIResponses, APIMessages:
		return true
	}
	return false
}

// DefaultAPITypes is the built-in protocol list used when a provider has not
// configured its own list.
func DefaultAPITypes() []string {
	return []string{APIChatCompletions, APIResponses, APIMessages}
}

// ChooseAPIType picks the upstream protocol for an inbound request.
//
// Precedence of the supported-protocol list:
//
//	target protocols > model protocols > provider protocols > built-in defaults.
//
// The target list may restrict the available protocols; it is intersected with
// the model/provider supported list while preserving the target's configured
// order. If the inbound protocol is in the resulting list it is used directly
// (no translation); otherwise the first configured protocol wins.
func ChooseAPIType(targetTypes, modelTypes, providerTypes []string, inbound string) string {
	available := modelTypes
	if len(available) == 0 {
		available = providerTypes
	}
	if len(available) == 0 {
		available = DefaultAPITypes()
	}
	pool := available
	if len(targetTypes) > 0 {
		allowed := map[string]bool{}
		for _, t := range available {
			allowed[t] = true
		}
		filtered := make([]string, 0, len(targetTypes))
		for _, t := range targetTypes {
			if allowed[t] {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) > 0 {
			pool = filtered
		}
	}
	if inbound != "" {
		for _, name := range pool {
			if name == inbound {
				return inbound
			}
		}
	}
	return pool[0]
}

// NormalizeOrderedAPITypes dedupes protocols while preserving caller order.
func NormalizeOrderedAPITypes(v []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(v))
	for _, item := range v {
		item = normalizeAPIType(item)
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

// ResolvePath returns the effective upstream path for a protocol.
// Model override > provider override > protocol built-in default.
func ResolvePath(modelOverrides, providerOverrides map[string]string, apiType string) string {
	if p := strings.TrimSpace(modelOverrides[apiType]); p != "" {
		return p
	}
	if p := strings.TrimSpace(providerOverrides[apiType]); p != "" {
		return p
	}
	return DefaultPath(apiType)
}

// --------------------------------------------------------- provider models

const providerModelSelect = `
SELECT id, provider_id, model_id, display_name, source, api_types, path_overrides,
       context_window, max_output_tokens,
       cost_currency, cost_input::float8, cost_output::float8,
       cost_cache_read::float8, cost_cache_write::float8, cost_tiers,
       enabled, created_at, updated_at
FROM provider_models`

func scanProviderModel(row rowScanner) (ProviderModel, error) {
	var m ProviderModel
	var overrides, tiers []byte
	err := row.Scan(&m.ID, &m.ProviderID, &m.ModelID, &m.DisplayName, &m.Source,
		&m.APITypes, &overrides, &m.ContextWindow, &m.MaxOutputTokens,
		&m.Cost.Currency, &m.Cost.Input, &m.Cost.Output,
		&m.Cost.CacheRead, &m.Cost.CacheWrite, &tiers,
		&m.Enabled, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return m, err
	}
	m.PathOverrides = map[string]string{}
	if len(overrides) > 0 {
		_ = json.Unmarshal(overrides, &m.PathOverrides)
	}
	m.Cost.Tiers = []CostTier{}
	if len(tiers) > 0 {
		_ = json.Unmarshal(tiers, &m.Cost.Tiers)
	}
	if m.Cost.Currency == "" {
		m.Cost.Currency = "USD"
	}
	if m.APITypes == nil {
		m.APITypes = []string{}
	}
	return m, nil
}

func (s *Store) ListProviderModels(ctx context.Context, providerID string) ([]ProviderModel, error) {
	rows, err := s.pool.Query(ctx, providerModelSelect+`
WHERE provider_id=$1 ORDER BY model_id ASC`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProviderModel, 0)
	for rows.Next() {
		m, err := scanProviderModel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetProviderModel(ctx context.Context, id string) (ProviderModel, error) {
	m, err := scanProviderModel(s.pool.QueryRow(ctx, providerModelSelect+` WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrNotFound
	}
	return m, err
}

func normalizeModelInput(in ProviderModelInput) (ProviderModelInput, error) {
	in.ModelID = strings.TrimSpace(in.ModelID)
	if in.ModelID == "" {
		return in, fmt.Errorf("%w: model_id is required", ErrInvalid)
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if in.DisplayName == "" {
		in.DisplayName = in.ModelID
	}
	types := make([]string, 0, len(in.APITypes))
	seen := map[string]bool{}
	for _, t := range in.APITypes {
		t = normalizeAPIType(t)
		if !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}
	in.APITypes = types
	if in.ContextWindow < 0 || in.MaxOutputTokens < 0 {
		return in, fmt.Errorf("%w: context_window and max_output_tokens must not be negative", ErrInvalid)
	}
	cost, err := normalizeModelCost(in.Cost)
	if err != nil {
		return in, err
	}
	in.Cost = cost
	if in.PathOverrides == nil {
		in.PathOverrides = map[string]string{}
	}
	for k, v := range in.PathOverrides {
		if !IsSupportedAPIType(k) {
			return in, fmt.Errorf("%w: unsupported api type %q in path_overrides", ErrInvalid, k)
		}
		v = strings.TrimSpace(v)
		if v != "" && !strings.HasPrefix(v, "/") {
			in.PathOverrides[k] = "/" + v
		}
	}
	return in, nil
}

func (s *Store) CreateProviderModel(ctx context.Context, providerID string, in ProviderModelInput) (ProviderModel, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM providers WHERE id=$1)`, providerID).Scan(&exists); err != nil {
		return ProviderModel{}, err
	}
	if !exists {
		return ProviderModel{}, ErrNotFound
	}
	in, err := normalizeModelInput(in)
	if err != nil {
		return ProviderModel{}, err
	}
	var id string
	err = s.pool.QueryRow(ctx, `
INSERT INTO provider_models (provider_id, model_id, display_name, source, api_types,
path_overrides, context_window, max_output_tokens,
cost_currency, cost_input, cost_output, cost_cache_read, cost_cache_write, cost_tiers, enabled)
VALUES ($1,$2,$3,'manual',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id`,
		providerID, in.ModelID, in.DisplayName, in.APITypes, marshalHeaders(in.PathOverrides),
		in.ContextWindow, in.MaxOutputTokens,
		in.Cost.Currency, in.Cost.Input, in.Cost.Output, in.Cost.CacheRead, in.Cost.CacheWrite,
		marshalCostTiers(in.Cost.Tiers), boolOr(in.Enabled, true)).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return ProviderModel{}, fmt.Errorf("%w: model %q already exists for this provider", ErrConflict, in.ModelID)
		}
		return ProviderModel{}, err
	}
	return s.GetProviderModel(ctx, id)
}

func (s *Store) UpdateProviderModel(ctx context.Context, providerID, id string, in ProviderModelInput) (ProviderModel, error) {
	in, err := normalizeModelInput(in)
	if err != nil {
		return ProviderModel{}, err
	}
	cmd, err := s.pool.Exec(ctx, `
UPDATE provider_models SET model_id=$3, display_name=$4, api_types=$5,
path_overrides=$6, context_window=$7, max_output_tokens=$8,
cost_currency=$9, cost_input=$10, cost_output=$11, cost_cache_read=$12,
cost_cache_write=$13, cost_tiers=$14, enabled=$15
WHERE id=$1 AND provider_id=$2`,
		id, providerID, in.ModelID, in.DisplayName, in.APITypes, marshalHeaders(in.PathOverrides),
		in.ContextWindow, in.MaxOutputTokens,
		in.Cost.Currency, in.Cost.Input, in.Cost.Output, in.Cost.CacheRead, in.Cost.CacheWrite,
		marshalCostTiers(in.Cost.Tiers), boolOr(in.Enabled, true))
	if err != nil {
		return ProviderModel{}, err
	}
	if cmd.RowsAffected() == 0 {
		return ProviderModel{}, ErrNotFound
	}
	return s.GetProviderModel(ctx, id)
}

func (s *Store) DeleteProviderModel(ctx context.Context, providerID, id string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM provider_models WHERE id=$1 AND provider_id=$2`, id, providerID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DiscoveredModel is one model returned by a provider's /models endpoint.
type DiscoveredModel struct {
	ID          string
	DisplayName string
}

// UpsertDiscoveredModels records models discovered from /models without
// clobbering manual per-model overrides.
func (s *Store) UpsertDiscoveredModels(ctx context.Context, providerID string, models []DiscoveredModel) ([]ProviderModel, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	for _, m := range models {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(m.DisplayName)
		if name == "" {
			name = id
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO provider_models (provider_id, model_id, display_name, source, enabled)
VALUES ($1,$2,$3,'discovered',TRUE)
ON CONFLICT (provider_id, model_id) DO UPDATE SET
display_name=CASE WHEN provider_models.display_name='' THEN EXCLUDED.display_name
ELSE provider_models.display_name END,
source=CASE WHEN provider_models.source='manual' THEN 'manual' ELSE 'discovered' END`,
			providerID, id, name); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ListProviderModels(ctx, providerID)
}

// ProviderCredentials returns the decrypted connection settings for a provider,
// used by model discovery and health checks.
// ProviderCredentials loads a provider together with its decrypted API key and
// proxy password.
func (s *Store) ProviderCredentials(ctx context.Context, providerID string) (Provider, Credentials, error) {
	p, err := s.GetProvider(ctx, providerID)
	if err != nil {
		return p, Credentials{}, err
	}
	var ct, proxyCT string
	if err := s.pool.QueryRow(ctx,
		`SELECT api_key_ciphertext, proxy_password_ciphertext FROM providers WHERE id=$1`,
		providerID).Scan(&ct, &proxyCT); err != nil {
		return p, Credentials{}, err
	}
	var creds Credentials
	if ct != "" {
		creds.APIKey, err = s.cipher.Decrypt(ct)
		if err != nil {
			return p, creds, err
		}
	}
	if proxyCT != "" {
		creds.ProxyPassword, err = s.cipher.Decrypt(proxyCT)
		if err != nil {
			return p, creds, err
		}
	}
	return p, creds, nil
}

// ResolveTarget loads a runtime target and resolves its upstream protocol and
// path from the model definition, provider-level config and built-in defaults.
func (s *Store) ResolveTarget(ctx context.Context, targetID, inboundAPIType string) (RuntimeTarget, error) {
	var t RuntimeTarget
	var ct, proxyCT string
	var headers []byte
	var targetTypes, providerTypes []string
	var providerOverridesRaw []byte
	var modelID string
	var modelTypes []string
	var modelOverridesRaw []byte
	var tiersRaw []byte
	err := s.pool.QueryRow(ctx, `
SELECT t.id, t.rule_id, t.role, p.id, p.name, p.base_url, p.auth_type,
       p.api_key_ciphertext, p.headers, p.timeout_ms, t.upstream_model, t.api_types,
       t.weight, t.priority,
       COALESCE(pm.cost_currency,'USD'), COALESCE(pm.cost_input,0)::float8,
       COALESCE(pm.cost_output,0)::float8, COALESCE(pm.cost_cache_read,0)::float8,
       COALESCE(pm.cost_cache_write,0)::float8, COALESCE(pm.cost_tiers,'[]'::jsonb),
       p.api_types, p.path_overrides,
       COALESCE(pm.model_id,''), COALESCE(pm.api_types,'{}'::text[]),
       COALESCE(pm.path_overrides,'{}'::jsonb),
       p.proxy_type, p.proxy_url, p.proxy_username, p.proxy_password_ciphertext,
       p.retry_count, p.retry_backoff, p.retry_initial_ms, p.retry_max_ms,
       p.retry_linear_interval_ms
FROM routing_targets t
JOIN providers p ON p.id=t.provider_id
LEFT JOIN provider_models pm ON pm.id=t.provider_model_id
WHERE t.id=$1 AND t.enabled=TRUE AND p.enabled=TRUE
  AND (t.provider_model_id IS NULL OR pm.enabled=TRUE)`, targetID).Scan(
		&t.TargetID, &t.RuleID, &t.Role, &t.ProviderID, &t.ProviderName, &t.BaseURL,
		&t.AuthType, &ct, &headers, &t.TimeoutMS, &t.UpstreamModel, &targetTypes,
		&t.Weight, &t.Priority,
		&t.Cost.Currency, &t.Cost.Input, &t.Cost.Output,
		&t.Cost.CacheRead, &t.Cost.CacheWrite, &tiersRaw,
		&providerTypes, &providerOverridesRaw, &modelID, &modelTypes, &modelOverridesRaw,
		&t.Proxy.Type, &t.Proxy.URL, &t.Proxy.Username, &proxyCT,
		&t.Retry.Count, &t.Retry.Backoff, &t.Retry.InitialMS, &t.Retry.MaxMS,
		&t.Retry.LinearDelayMS)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	if ct != "" {
		t.APIKey, _ = s.cipher.Decrypt(ct)
	}
	if proxyCT != "" {
		t.Proxy.Password, _ = s.cipher.Decrypt(proxyCT)
	}
	t.Headers = decodeHeaders(headers)
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
	if modelID != "" {
		t.UpstreamModel = modelID
	}
	t.APIType = ChooseAPIType(targetTypes, modelTypes, providerTypes, inboundAPIType)
	t.UpstreamPath = ResolvePath(modelOverrides, providerOverrides, t.APIType)
	return t, nil
}

// normalizeModelCost validates a price list and defaults the currency.
func normalizeModelCost(cost ModelCost) (ModelCost, error) {
	cost.Currency = strings.ToUpper(strings.TrimSpace(cost.Currency))
	if cost.Currency == "" {
		cost.Currency = "USD"
	}
	if len(cost.Currency) != 3 {
		return cost, fmt.Errorf("%w: cost currency must be a 3-letter ISO 4217 code, got %q", ErrInvalid, cost.Currency)
	}
	for _, v := range []struct {
		name string
		val  float64
	}{
		{"input", cost.Input}, {"output", cost.Output},
		{"cache_read", cost.CacheRead}, {"cache_write", cost.CacheWrite},
	} {
		if v.val < 0 {
			return cost, fmt.Errorf("%w: cost %s must not be negative", ErrInvalid, v.name)
		}
	}
	if cost.Tiers == nil {
		cost.Tiers = []CostTier{}
	}
	prev := -1
	for i, tier := range cost.Tiers {
		if tier.InputTokensAbove <= 0 {
			return cost, fmt.Errorf("%w: cost tier %d: input_tokens_above must be positive", ErrInvalid, i)
		}
		if tier.InputTokensAbove <= prev {
			return cost, fmt.Errorf("%w: cost tiers must be sorted by input_tokens_above", ErrInvalid)
		}
		prev = tier.InputTokensAbove
		for _, v := range []struct {
			name string
			val  float64
		}{
			{"input", tier.Input}, {"output", tier.Output},
			{"cache_read", tier.CacheRead}, {"cache_write", tier.CacheWrite},
		} {
			if v.val < 0 {
				return cost, fmt.Errorf("%w: cost tier %d: %s must not be negative", ErrInvalid, i, v.name)
			}
		}
	}
	return cost, nil
}

func marshalCostTiers(tiers []CostTier) []byte {
	if len(tiers) == 0 {
		return []byte("[]")
	}
	raw, err := json.Marshal(tiers)
	if err != nil {
		return []byte("[]")
	}
	return raw
}

// EffectiveCost returns the price row that applies to a request with the given
// input token count: the highest tier whose threshold is exceeded, else the
// flat prices.
func (m ProviderModel) EffectiveCost(inputTokens int) CostTier {
	chosen := CostTier{
		Input: m.Cost.Input, Output: m.Cost.Output,
		CacheRead: m.Cost.CacheRead, CacheWrite: m.Cost.CacheWrite,
	}
	for _, tier := range m.Cost.Tiers {
		if inputTokens > tier.InputTokensAbove {
			chosen = tier
		}
	}
	return chosen
}
