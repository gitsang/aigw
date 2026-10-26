package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ------------------------------------------------------- global API type map

func (s *Store) ListAPITypes(ctx context.Context) ([]APITypeDef, error) {
	rows, err := s.pool.Query(ctx, `
SELECT name, path, enabled, headers, sort_order, updated_at
FROM api_types ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]APITypeDef, 0)
	for rows.Next() {
		var d APITypeDef
		var headers []byte
		if err := rows.Scan(&d.Name, &d.Path, &d.Enabled, &headers, &d.SortOrder, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Headers = decodeHeaders(headers)
		out = append(out, d)
	}
	return out, rows.Err()
}

// UpdateAPITypes upserts the global defaults. Unknown protocol names are
// rejected because the gateway can only translate the three supported formats.
func (s *Store) UpdateAPITypes(ctx context.Context, inputs []APITypeInput) ([]APITypeDef, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	for i, in := range inputs {
		name := strings.TrimSpace(in.Name)
		if !IsSupportedAPIType(name) {
			return nil, fmt.Errorf("unsupported api type %q", name)
		}
		path := strings.TrimSpace(in.Path)
		if path == "" {
			path = DefaultPath(name)
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO api_types (name, path, enabled, headers, sort_order)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (name) DO UPDATE SET path=EXCLUDED.path, enabled=EXCLUDED.enabled,
headers=EXCLUDED.headers, sort_order=EXCLUDED.sort_order, updated_at=now()`,
			name, path, boolOr(in.Enabled, true), marshalHeaders(in.Headers), (i+1)*10); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ListAPITypes(ctx)
}

// apiTypeDefaults returns defaults keyed by name plus the enabled names in order.
func (s *Store) apiTypeDefaults(ctx context.Context) (map[string]APITypeDef, []string, error) {
	defs, err := s.ListAPITypes(ctx)
	if err != nil {
		return nil, nil, err
	}
	byName := make(map[string]APITypeDef, len(defs))
	enabled := make([]string, 0, len(defs))
	for _, d := range defs {
		byName[d.Name] = d
		if d.Enabled {
			enabled = append(enabled, d.Name)
		}
	}
	return byName, enabled, nil
}

func IsSupportedAPIType(name string) bool {
	switch name {
	case APIChatCompletions, APIResponses, APIMessages:
		return true
	}
	return false
}

// ChooseAPIType picks the upstream protocol for an inbound request. When the
// model explicitly supports the inbound protocol it is kept (no translation),
// otherwise the first supported protocol is used.
func ChooseAPIType(supported []string, enabledDefaults []string, inbound string) string {
	pool := supported
	if len(pool) == 0 {
		pool = enabledDefaults
	}
	if len(pool) == 0 {
		pool = []string{APIChatCompletions}
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

// ResolvePath returns the effective upstream path for a protocol.
func ResolvePath(overrides map[string]string, defaults map[string]APITypeDef, apiType string) string {
	if p := strings.TrimSpace(overrides[apiType]); p != "" {
		return p
	}
	if d, ok := defaults[apiType]; ok && strings.TrimSpace(d.Path) != "" {
		return d.Path
	}
	return DefaultPath(apiType)
}

// --------------------------------------------------------- provider models

const providerModelSelect = `
SELECT id, provider_id, model_id, display_name, source, api_types, path_overrides,
       enabled, created_at, updated_at
FROM provider_models`

func scanProviderModel(row rowScanner) (ProviderModel, error) {
	var m ProviderModel
	var overrides []byte
	err := row.Scan(&m.ID, &m.ProviderID, &m.ModelID, &m.DisplayName, &m.Source,
		&m.APITypes, &overrides, &m.Enabled, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return m, err
	}
	m.PathOverrides = map[string]string{}
	if len(overrides) > 0 {
		_ = json.Unmarshal(overrides, &m.PathOverrides)
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
		return in, errors.New("model_id is required")
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
	if in.PathOverrides == nil {
		in.PathOverrides = map[string]string{}
	}
	for k, v := range in.PathOverrides {
		if !IsSupportedAPIType(k) {
			return in, fmt.Errorf("unsupported api type %q in path_overrides", k)
		}
		if !strings.HasPrefix(strings.TrimSpace(v), "/") {
			in.PathOverrides[k] = "/" + strings.TrimSpace(v)
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
path_overrides, enabled)
VALUES ($1,$2,$3,'manual',$4,$5,$6) RETURNING id`,
		providerID, in.ModelID, in.DisplayName, in.APITypes,
		marshalHeaders(in.PathOverrides), boolOr(in.Enabled, true)).Scan(&id)
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
path_overrides=$6, enabled=$7
WHERE id=$1 AND provider_id=$2`,
		id, providerID, in.ModelID, in.DisplayName, in.APITypes,
		marshalHeaders(in.PathOverrides), boolOr(in.Enabled, true))
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

// ResolveTarget loads a runtime target and resolves its upstream protocol and
// path from the provider model catalog + global API type defaults.
func (s *Store) ResolveTarget(ctx context.Context, targetID, inboundAPIType string) (RuntimeTarget, error) {
	var t RuntimeTarget
	var ct string
	var headers []byte
	var modelID, modelRowID string
	var modelAPITypes []string
	var pathOverridesRaw []byte
	err := s.pool.QueryRow(ctx, `
SELECT t.id, t.rule_id, t.role, p.id, p.name, p.base_url, p.auth_type,
       p.api_key_ciphertext, p.headers, p.timeout_ms, t.upstream_model, t.api_type,
       t.upstream_path, t.weight, t.priority,
       p.input_price_per_million::float8, p.output_price_per_million::float8,
       COALESCE(t.provider_model_id::text,''), COALESCE(pm.model_id,''),
       COALESCE(pm.api_types,'{}'::text[]), COALESCE(pm.path_overrides,'{}'::jsonb)
FROM routing_targets t
JOIN providers p ON p.id=t.provider_id
LEFT JOIN provider_models pm ON pm.id=t.provider_model_id
WHERE t.id=$1 AND t.enabled=TRUE AND p.enabled=TRUE
  AND (t.provider_model_id IS NULL OR pm.enabled=TRUE)`, targetID).Scan(
		&t.TargetID, &t.RuleID, &t.Role, &t.ProviderID, &t.ProviderName, &t.BaseURL,
		&t.AuthType, &ct, &headers, &t.TimeoutMS, &t.UpstreamModel, &t.APIType,
		&t.UpstreamPath, &t.Weight, &t.Priority, &t.InputPrice, &t.OutputPrice,
		&modelRowID, &modelID, &modelAPITypes, &pathOverridesRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	if ct != "" {
		t.APIKey, _ = s.cipher.Decrypt(ct)
	}
	t.Headers = decodeHeaders(headers)

	defaults, enabledDefaults, err := s.apiTypeDefaults(ctx)
	if err != nil {
		return t, err
	}
	overrides := map[string]string{}
	if len(pathOverridesRaw) > 0 {
		_ = json.Unmarshal(pathOverridesRaw, &overrides)
	}
	if modelID != "" {
		// Catalog-backed target: derive protocol/path from the model definition,
		// matching the inbound protocol when the model supports it.
		t.UpstreamModel = modelID
		t.APIType = ChooseAPIType(modelAPITypes, enabledDefaults, inboundAPIType)
		t.UpstreamPath = ResolvePath(overrides, defaults, t.APIType)
	} else {
		t.APIType = normalizeAPIType(t.APIType)
		t.UpstreamPath = ResolvePath(nil, defaults, t.APIType)
	}
	return t, nil
}

// ProviderCredentials returns the decrypted connection settings for a provider,
// used by model discovery and health checks.
func (s *Store) ProviderCredentials(ctx context.Context, providerID string) (Provider, string, error) {
	p, err := s.GetProvider(ctx, providerID)
	if err != nil {
		return p, "", err
	}
	var ct string
	if err := s.pool.QueryRow(ctx, `SELECT api_key_ciphertext FROM providers WHERE id=$1`, providerID).Scan(&ct); err != nil {
		return p, "", err
	}
	key, err := s.cipher.Decrypt(ct)
	return p, key, err
}
