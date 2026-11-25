package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// IsSupportedAPIType reports whether name is a known upstream protocol.
func IsSupportedAPIType(name string) bool {
	switch name {
	case APIChatCompletions, APIResponses, APIMessages, APIInteractions:
		return true
	}
	return false
}

// DefaultProtocols is the built-in protocol list in canonical fallback order.
func DefaultProtocols() []string {
	return []string{APIChatCompletions, APIResponses, APIMessages, APIInteractions}
}

// DefaultEndpoint is the built-in upstream path of a protocol.
func DefaultEndpoint(apiType string) string {
	switch apiType {
	case APIResponses:
		return "/v1/responses"
	case APIMessages:
		return "/v1/messages"
	case APIInteractions:
		return "/v1beta/interactions"
	default:
		return "/v1/chat/completions"
	}
}

// DefaultAPISchema is the schema used when nothing is configured: every
// protocol enabled through its built-in endpoint.
func DefaultAPISchema() APISchema {
	out := make(APISchema, len(DefaultProtocols()))
	for _, name := range DefaultProtocols() {
		out[name] = APISchemaEntry{Enabled: true, Endpoint: DefaultEndpoint(name)}
	}
	return out
}

// Protocols returns the enabled protocols of the schema in canonical fallback
// order chat.completions → responses → messages → interactions.
func (s APISchema) Protocols() []string {
	out := make([]string, 0, len(s))
	for _, name := range DefaultProtocols() {
		if entry, ok := s[name]; ok && entry.Enabled {
			out = append(out, name)
		}
	}
	return out
}

// normalizeAPISchema validates a caller-supplied schema and normalizes its
// endpoints. Unknown protocols are rejected so a typo cannot silently disable a
// protocol, and at least one protocol must stay enabled.
func normalizeAPISchema(in APISchema) (APISchema, error) {
	if len(in) == 0 {
		return APISchema{}, nil
	}
	out := make(APISchema, len(in))
	enabled := 0
	for name, entry := range in {
		if !IsSupportedAPIType(name) {
			return nil, fmt.Errorf("%w: unknown protocol %q in api_schema", ErrInvalid, name)
		}
		endpoint := strings.TrimSpace(entry.Endpoint)
		if endpoint != "" && !strings.HasPrefix(endpoint, "/") {
			endpoint = "/" + endpoint
		}
		if entry.Enabled {
			enabled++
		}
		out[name] = APISchemaEntry{Enabled: entry.Enabled, Endpoint: endpoint}
	}
	if enabled == 0 {
		return nil, fmt.Errorf("%w: api_schema must enable at least one protocol", ErrInvalid)
	}
	return out, nil
}

// ResolveAPISchema merges the model schema over the provider schema over the
// built-in defaults. The result always carries every protocol: Enabled reports
// whether the upstream speaks it, and Endpoint is the path to use for the
// enabled ones.
//
// Protocol support replaces wholesale: a non-empty model schema defines the set
// of protocols the model speaks, otherwise the provider schema does, otherwise
// all built-in protocols are enabled. Endpoints fall back per protocol: model →
// provider → built-in default.
func ResolveAPISchema(model, provider APISchema) APISchema {
	support := model
	if len(support) == 0 {
		support = provider
	}
	out := make(APISchema, len(DefaultProtocols()))
	for _, name := range DefaultProtocols() {
		enabled := true
		if len(support) > 0 {
			declared, ok := support[name]
			enabled = ok && declared.Enabled
		}
		if !enabled {
			out[name] = APISchemaEntry{}
			continue
		}
		endpoint := ""
		if entry, ok := model[name]; ok {
			endpoint = strings.TrimSpace(entry.Endpoint)
		}
		if endpoint == "" {
			if entry, ok := provider[name]; ok {
				endpoint = strings.TrimSpace(entry.Endpoint)
			}
		}
		if endpoint == "" {
			endpoint = DefaultEndpoint(name)
		}
		if !strings.HasPrefix(endpoint, "/") {
			endpoint = "/" + endpoint
		}
		out[name] = APISchemaEntry{Enabled: true, Endpoint: endpoint}
	}
	return out
}

// ChooseUpstreamAPIType picks the upstream protocol for an inbound request from
// an effective schema.
//
// The upstream protocol is not configurable per target: an inbound request is
// forwarded with the protocol it arrived on whenever the upstream speaks it.
// Otherwise the gateway falls back in the fixed order chat.completions →
// responses → messages → interactions and takes the first protocol the
// upstream supports,
// translating the payload as needed. An empty inbound protocol (used by the
// health prober) takes that same first protocol.
func ChooseUpstreamAPIType(schema APISchema, inbound string) string {
	supported := schema.Protocols()
	if len(supported) == 0 {
		// Nothing is enabled: let the request fail upstream rather than
		// silently picking a protocol that is marked unsupported.
		return APIChatCompletions
	}
	if inbound != "" {
		for _, name := range supported {
			if name == inbound {
				return inbound
			}
		}
	}
	return supported[0]
}

// UnionAPISchemas merges effective schemas: a protocol is enabled when any
// input enables it, and takes the endpoint of the first input that enables it.
// It answers "can this virtual model serve protocol X?" across all its targets.
func UnionAPISchemas(schemas []APISchema) APISchema {
	out := make(APISchema, len(DefaultProtocols()))
	for _, name := range DefaultProtocols() {
		out[name] = APISchemaEntry{}
	}
	for _, schema := range schemas {
		for name, entry := range schema {
			if !entry.Enabled {
				continue
			}
			current := out[name]
			if !current.Enabled {
				out[name] = APISchemaEntry{Enabled: true, Endpoint: entry.Endpoint}
			}
		}
	}
	return out
}

// marshalAPISchema encodes a schema for a JSONB column.
func marshalAPISchema(schema APISchema) []byte {
	if len(schema) == 0 {
		return []byte("{}")
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

// decodeAPISchema decodes a JSONB column, never returning nil.
func decodeAPISchema(raw []byte) APISchema {
	out := APISchema{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

// --------------------------------------------------------- provider models

const providerModelSelect = `
SELECT id, provider_id, model_id, display_name, source, api_schema,
       context_window, max_output_tokens,
       cost_currency, cost_input::float8, cost_output::float8,
       cost_cache_read::float8, cost_cache_write::float8, cost_tiers,
       enabled, created_at, updated_at
FROM provider_models`

func scanProviderModel(row rowScanner) (ProviderModel, error) {
	var m ProviderModel
	var schema, tiers []byte
	err := row.Scan(&m.ID, &m.ProviderID, &m.ModelID, &m.DisplayName, &m.Source,
		&schema, &m.ContextWindow, &m.MaxOutputTokens,
		&m.Cost.Currency, &m.Cost.Input, &m.Cost.Output,
		&m.Cost.CacheRead, &m.Cost.CacheWrite, &tiers,
		&m.Enabled, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return m, err
	}
	m.APISchema = decodeAPISchema(schema)
	m.Cost.Tiers = []CostTier{}
	if len(tiers) > 0 {
		_ = json.Unmarshal(tiers, &m.Cost.Tiers)
	}
	if m.Cost.Currency == "" {
		m.Cost.Currency = "USD"
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
	schema, err := normalizeAPISchema(in.APISchema)
	if err != nil {
		return in, err
	}
	in.APISchema = schema
	if in.ContextWindow < 0 || in.MaxOutputTokens < 0 {
		return in, fmt.Errorf("%w: context_window and max_output_tokens must not be negative", ErrInvalid)
	}
	cost, err := normalizeModelCost(in.Cost)
	if err != nil {
		return in, err
	}
	in.Cost = cost
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
INSERT INTO provider_models (provider_id, model_id, display_name, source, api_schema,
context_window, max_output_tokens,
cost_currency, cost_input, cost_output, cost_cache_read, cost_cache_write, cost_tiers, enabled)
VALUES ($1,$2,$3,'manual',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`,
		providerID, in.ModelID, in.DisplayName, marshalAPISchema(in.APISchema),
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
UPDATE provider_models SET model_id=$3, display_name=$4, api_schema=$5,
context_window=$6, max_output_tokens=$7,
cost_currency=$8, cost_input=$9, cost_output=$10, cost_cache_read=$11,
cost_cache_write=$12, cost_tiers=$13, enabled=$14
WHERE id=$1 AND provider_id=$2`,
		id, providerID, in.ModelID, in.DisplayName, marshalAPISchema(in.APISchema),
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

// UpdateProviderModelPricing replaces the price list and the token limits of a
// provider model. It is used by the models.dev price sync, which must not touch
// the model's identity, API types or enabled state.
func (s *Store) UpdateProviderModelPricing(ctx context.Context, providerID, id string, cost ModelCost, contextWindow, maxOutputTokens int) error {
	cost, err := normalizeModelCost(cost)
	if err != nil {
		return err
	}
	if contextWindow < 0 || maxOutputTokens < 0 {
		return fmt.Errorf("%w: context_window and max_output_tokens must not be negative", ErrInvalid)
	}
	cmd, err := s.pool.Exec(ctx, `
UPDATE provider_models SET cost_currency=$3, cost_input=$4, cost_output=$5,
cost_cache_read=$6, cost_cache_write=$7, cost_tiers=$8,
context_window=$9, max_output_tokens=$10
WHERE id=$1 AND provider_id=$2`,
		id, providerID, cost.Currency, cost.Input, cost.Output, cost.CacheRead, cost.CacheWrite,
		marshalCostTiers(cost.Tiers), contextWindow, maxOutputTokens)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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
	var providerSchemaRaw []byte
	var modelID string
	var modelSchemaRaw []byte
	var tiersRaw []byte
	err := s.pool.QueryRow(ctx, `
SELECT t.id, t.virtual_model_id, t.role, p.id, p.name, p.base_url, p.auth_type,
       p.api_key_ciphertext, p.headers, p.timeout_ms, t.upstream_model,
       t.weight, t.priority,
       COALESCE(pm.cost_currency,'USD'), COALESCE(pm.cost_input,0)::float8,
       COALESCE(pm.cost_output,0)::float8, COALESCE(pm.cost_cache_read,0)::float8,
       COALESCE(pm.cost_cache_write,0)::float8, COALESCE(pm.cost_tiers,'[]'::jsonb),
       p.api_schema,
       COALESCE(pm.model_id,''), COALESCE(pm.api_schema,'{}'::jsonb),
       p.proxy_type, p.proxy_url, p.proxy_username, p.proxy_password_ciphertext,
       p.retry_count, p.retry_backoff, p.retry_initial_ms, p.retry_max_ms,
       p.retry_linear_interval_ms
FROM virtual_model_targets t
JOIN providers p ON p.id=t.provider_id
LEFT JOIN provider_models pm ON pm.id=t.provider_model_id
WHERE t.id=$1 AND t.enabled=TRUE AND p.enabled=TRUE
  AND (t.provider_model_id IS NULL OR pm.enabled=TRUE)`, targetID).Scan(
		&t.TargetID, &t.VirtualModelID, &t.Role, &t.ProviderID, &t.ProviderName, &t.BaseURL,
		&t.AuthType, &ct, &headers, &t.TimeoutMS, &t.UpstreamModel,
		&t.Weight, &t.Priority,
		&t.Cost.Currency, &t.Cost.Input, &t.Cost.Output,
		&t.Cost.CacheRead, &t.Cost.CacheWrite, &tiersRaw,
		&providerSchemaRaw, &modelID, &modelSchemaRaw,
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

	if modelID != "" {
		t.UpstreamModel = modelID
	}
	schema := ResolveAPISchema(decodeAPISchema(modelSchemaRaw), decodeAPISchema(providerSchemaRaw))
	t.APIType = ChooseUpstreamAPIType(schema, inboundAPIType)
	t.Endpoint = schema[t.APIType].Endpoint
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
