package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gitsang/aigw/internal/crypto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaFS embed.FS

var ErrNotFound = errors.New("not found")

type Store struct {
	pool   *pgxpool.Pool
	cipher *crypto.Cipher
}

type HealthProbeTarget struct {
	TargetID          string
	ProviderID        string
	ProviderName      string
	BaseURL           string
	AuthType          string
	APIKey            string
	Headers           map[string]string
	HealthCheckURL    string
	HealthCheckMethod string
	TimeoutMS         int
	UpstreamModel     string
	APIType           string
	UpstreamPath      string
}

// RuntimeTarget is everything needed to perform one upstream request.
type RuntimeTarget struct {
	TargetID      string
	RuleID        string
	Role          string
	ProviderID    string
	ProviderName  string
	BaseURL       string
	AuthType      string
	APIKey        string
	Headers       map[string]string
	TimeoutMS     int
	UpstreamModel string
	APIType       string
	UpstreamPath  string
	Weight        int
	Priority      int
	InputPrice    float64
	OutputPrice   float64
}

func New(ctx context.Context, databaseURL string, maxConns, minConns int32, secretCipher *crypto.Cipher) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	if minConns >= 0 {
		cfg.MinConns = minConns
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{pool: pool, cipher: secretCipher}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, string(schema), pgx.QueryExecModeSimpleProtocol); err != nil {
		return fmt.Errorf("run schema migration: %w", err)
	}
	return nil
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func randomKey() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 36)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return "aigw_" + string(b), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func boolOr(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func marshalHeaders(h map[string]string) []byte {
	if h == nil {
		h = map[string]string{}
	}
	b, _ := json.Marshal(h)
	return b
}

func decodeHeaders(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

// ---------------------------------------------------------------- providers

func (s *Store) ListProviders(ctx context.Context) ([]Provider, error) {
	rows, err := s.pool.Query(ctx, `
SELECT p.id, p.name, p.base_url, p.auth_type, p.api_key_last4,
       (p.api_key_ciphertext <> ''), p.headers, p.health_check_url, p.health_check_method,
       p.timeout_ms, p.input_price_per_million::float8, p.output_price_per_million::float8,
       p.enabled, p.created_at, p.updated_at,
       COUNT(t.id) FILTER (WHERE t.enabled)::int,
       COUNT(t.id) FILTER (WHERE t.enabled AND COALESCE(h.healthy, TRUE))::int
FROM providers p
LEFT JOIN routing_targets t ON t.provider_id = p.id
LEFT JOIN target_health h ON h.target_id = t.id
GROUP BY p.id ORDER BY p.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Provider, 0)
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanProvider(row rowScanner) (Provider, error) {
	var p Provider
	var headers []byte
	err := row.Scan(&p.ID, &p.Name, &p.BaseURL, &p.AuthType, &p.APIKeyLast4, &p.HasAPIKey,
		&headers, &p.HealthCheckURL, &p.HealthCheckMethod, &p.TimeoutMS,
		&p.InputPricePerMillion, &p.OutputPricePerMillion, &p.Enabled, &p.CreatedAt, &p.UpdatedAt,
		&p.TotalTargets, &p.HealthyTargets)
	if err != nil {
		return p, err
	}
	p.Headers = decodeHeaders(headers)
	return p, nil
}

const providerSelect = `
SELECT p.id, p.name, p.base_url, p.auth_type, p.api_key_last4,
       (p.api_key_ciphertext <> ''), p.headers, p.health_check_url, p.health_check_method,
       p.timeout_ms, p.input_price_per_million::float8, p.output_price_per_million::float8,
       p.enabled, p.created_at, p.updated_at,
       COUNT(t.id) FILTER (WHERE t.enabled)::int,
       COUNT(t.id) FILTER (WHERE t.enabled AND COALESCE(h.healthy, TRUE))::int
FROM providers p
LEFT JOIN routing_targets t ON t.provider_id = p.id
LEFT JOIN target_health h ON h.target_id = t.id`

func (s *Store) GetProvider(ctx context.Context, id string) (Provider, error) {
	row := s.pool.QueryRow(ctx, providerSelect+` WHERE p.id=$1 GROUP BY p.id`, id)
	p, err := scanProvider(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (s *Store) CreateProvider(ctx context.Context, in ProviderInput) (Provider, error) {
	enc, err := s.cipher.Encrypt(in.APIKey)
	if err != nil {
		return Provider{}, err
	}
	last4 := ""
	if len(in.APIKey) >= 4 {
		last4 = in.APIKey[len(in.APIKey)-4:]
	}
	var id string
	err = s.pool.QueryRow(ctx, `
INSERT INTO providers (name, base_url, auth_type, api_key_ciphertext, api_key_last4, headers,
health_check_url, health_check_method, timeout_ms, input_price_per_million,
output_price_per_million, enabled)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		in.Name, in.BaseURL, normalizeAuthType(in.AuthType), enc, last4, marshalHeaders(in.Headers),
		in.HealthCheckURL, normalizeHealthMethod(in.HealthCheckMethod), normalizeTimeout(in.TimeoutMS),
		in.InputPricePerMillion, in.OutputPricePerMillion, boolOr(in.Enabled, true)).Scan(&id)
	if err != nil {
		return Provider{}, err
	}
	return s.GetProvider(ctx, id)
}

func (s *Store) UpdateProvider(ctx context.Context, id string, in ProviderInput) (Provider, error) {
	enc, err := s.cipher.Encrypt(in.APIKey)
	if err != nil {
		return Provider{}, err
	}
	last4 := ""
	if len(in.APIKey) >= 4 {
		last4 = in.APIKey[len(in.APIKey)-4:]
	}
	cmd, err := s.pool.Exec(ctx, `
UPDATE providers SET name=$2, base_url=$3, auth_type=$4,
api_key_ciphertext=CASE WHEN $5::text='' THEN api_key_ciphertext ELSE $5 END,
api_key_last4=CASE WHEN $5::text='' THEN api_key_last4 ELSE $6 END,
headers=$7, health_check_url=$8, health_check_method=$9, timeout_ms=$10,
input_price_per_million=$11, output_price_per_million=$12, enabled=$13
WHERE id=$1`,
		id, in.Name, in.BaseURL, normalizeAuthType(in.AuthType), enc, last4,
		marshalHeaders(in.Headers), in.HealthCheckURL, normalizeHealthMethod(in.HealthCheckMethod),
		normalizeTimeout(in.TimeoutMS), in.InputPricePerMillion, in.OutputPricePerMillion,
		boolOr(in.Enabled, true))
	if err != nil {
		return Provider{}, err
	}
	if cmd.RowsAffected() == 0 {
		return Provider{}, ErrNotFound
	}
	return s.GetProvider(ctx, id)
}

func (s *Store) DeleteProvider(ctx context.Context, id string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM providers WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizeAuthType(v string) string {
	switch v {
	case "x-api-key", "none":
		return v
	default:
		return "bearer"
	}
}

func normalizeHealthMethod(v string) string {
	if strings.EqualFold(v, "GET") {
		return "GET"
	}
	return "POST"
}

func normalizeTimeout(v int) int {
	if v <= 0 {
		return 120000
	}
	return v
}

// ------------------------------------------------------------- routing rules

func (s *Store) ListRoutingRules(ctx context.Context) ([]RoutingRule, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, name, virtual_model, api_types, enabled, retry_count, retry_policy,
       fallback_policy, retry_delay_ms, max_backoff_ms, created_at, updated_at
FROM routing_rules ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RoutingRule, 0)
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		r.Targets, err = s.loadTargets(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanRule(row rowScanner) (RoutingRule, error) {
	var r RoutingRule
	err := row.Scan(&r.ID, &r.Name, &r.VirtualModel, &r.APITypes, &r.Enabled, &r.RetryCount,
		&r.RetryPolicy, &r.FallbackPolicy, &r.RetryDelayMS, &r.MaxBackoffMS, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

func (s *Store) GetRoutingRule(ctx context.Context, id string) (RoutingRule, error) {
	row := s.pool.QueryRow(ctx, `
SELECT id, name, virtual_model, api_types, enabled, retry_count, retry_policy,
       fallback_policy, retry_delay_ms, max_backoff_ms, created_at, updated_at
FROM routing_rules WHERE id=$1`, id)
	r, err := scanRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	r.Targets, err = s.loadTargets(ctx, id)
	return r, err
}

func (s *Store) GetRoutingRuleByModel(ctx context.Context, model string) (RoutingRule, error) {
	row := s.pool.QueryRow(ctx, `
SELECT id, name, virtual_model, api_types, enabled, retry_count, retry_policy,
       fallback_policy, retry_delay_ms, max_backoff_ms, created_at, updated_at
FROM routing_rules WHERE virtual_model=$1 AND enabled=TRUE`, model)
	r, err := scanRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	r.Targets, err = s.loadTargets(ctx, r.ID)
	return r, err
}

func (s *Store) loadTargets(ctx context.Context, ruleID string) ([]RouteTarget, error) {
	rows, err := s.pool.Query(ctx, `
SELECT t.id, t.rule_id, t.role, t.provider_id, p.name, p.base_url, t.upstream_model,
       t.api_type, t.upstream_path, t.weight, t.priority, t.enabled,
       COALESCE(h.healthy, TRUE), h.checked_at, COALESCE(h.latency_ms,0),
       COALESCE(h.status_code,0), COALESCE(h.error,'')
FROM routing_targets t
JOIN providers p ON p.id=t.provider_id
LEFT JOIN target_health h ON h.target_id=t.id
WHERE t.rule_id=$1
ORDER BY t.role DESC, t.priority DESC, t.created_at ASC`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RouteTarget, 0)
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func scanTarget(row rowScanner) (RouteTarget, error) {
	var t RouteTarget
	h := TargetHealth{}
	err := row.Scan(&t.ID, &t.RuleID, &t.Role, &t.ProviderID, &t.ProviderName, &t.ProviderBaseURL,
		&t.UpstreamModel, &t.APIType, &t.UpstreamPath, &t.Weight, &t.Priority, &t.Enabled,
		&h.Healthy, &h.CheckedAt, &h.LatencyMS, &h.StatusCode, &h.Error)
	if err != nil {
		return t, err
	}
	t.Health = &h
	return t, nil
}

func (s *Store) CreateRoutingRule(ctx context.Context, in RoutingRuleInput) (RoutingRule, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RoutingRule{}, err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `
INSERT INTO routing_rules (name, virtual_model, api_types, enabled, retry_count,
retry_policy, fallback_policy, retry_delay_ms, max_backoff_ms)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		in.Name, in.VirtualModel, normalizeAPITypes(in.APITypes), boolOr(in.Enabled, true),
		normalizeRetryCount(in.RetryCount), normalizeRetryPolicy(in.RetryPolicy),
		normalizeFallbackPolicy(in.FallbackPolicy), normalizeDelay(in.RetryDelayMS, 250),
		normalizeDelay(in.MaxBackoffMS, 5000)).Scan(&id)
	if err != nil {
		return RoutingRule{}, err
	}
	if err := insertTargets(ctx, tx, id, in.Targets); err != nil {
		return RoutingRule{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RoutingRule{}, err
	}
	return s.GetRoutingRule(ctx, id)
}

func (s *Store) UpdateRoutingRule(ctx context.Context, id string, in RoutingRuleInput) (RoutingRule, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RoutingRule{}, err
	}
	defer tx.Rollback(ctx)
	cmd, err := tx.Exec(ctx, `
UPDATE routing_rules SET name=$2, virtual_model=$3, api_types=$4, enabled=$5,
retry_count=$6, retry_policy=$7, fallback_policy=$8, retry_delay_ms=$9, max_backoff_ms=$10
WHERE id=$1`,
		id, in.Name, in.VirtualModel, normalizeAPITypes(in.APITypes), boolOr(in.Enabled, true),
		normalizeRetryCount(in.RetryCount), normalizeRetryPolicy(in.RetryPolicy),
		normalizeFallbackPolicy(in.FallbackPolicy), normalizeDelay(in.RetryDelayMS, 250),
		normalizeDelay(in.MaxBackoffMS, 5000))
	if err != nil {
		return RoutingRule{}, err
	}
	if cmd.RowsAffected() == 0 {
		return RoutingRule{}, ErrNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM routing_targets WHERE rule_id=$1`, id); err != nil {
		return RoutingRule{}, err
	}
	if err := insertTargets(ctx, tx, id, in.Targets); err != nil {
		return RoutingRule{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RoutingRule{}, err
	}
	return s.GetRoutingRule(ctx, id)
}

func insertTargets(ctx context.Context, tx pgx.Tx, ruleID string, targets []RouteTargetInput) error {
	for _, t := range targets {
		role := RoleLB
		if t.Role == RoleFallback {
			role = RoleFallback
		}
		apiType := normalizeAPIType(t.APIType)
		path := strings.TrimSpace(t.UpstreamPath)
		if path == "" {
			path = DefaultPath(apiType)
		}
		weight := t.Weight
		if weight <= 0 {
			weight = 100
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO routing_targets (rule_id, role, provider_id, upstream_model, api_type,
upstream_path, weight, priority, enabled)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			ruleID, role, t.ProviderID, t.UpstreamModel, apiType, path, weight, t.Priority,
			boolOr(t.Enabled, true)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteRoutingRule(ctx context.Context, id string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM routing_rules WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func DefaultPath(apiType string) string {
	switch apiType {
	case APIResponses:
		return "/v1/responses"
	case APIMessages:
		return "/v1/messages"
	default:
		return "/v1/chat/completions"
	}
}

func normalizeAPIType(v string) string {
	switch v {
	case APIResponses, APIMessages:
		return v
	default:
		return APIChatCompletions
	}
}

func normalizeAPITypes(v []string) []string {
	if len(v) == 0 {
		return []string{APIChatCompletions, APIResponses, APIMessages}
	}
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

func normalizeRetryPolicy(v string) string {
	if v == "fixed" {
		return v
	}
	return "exponential"
}

func normalizeFallbackPolicy(v string) string {
	if v == "retry-first" {
		return v
	}
	return "fallback-first"
}

func normalizeRetryCount(v int) int {
	if v < 1 {
		return 1
	}
	if v > 20 {
		return 20
	}
	return v
}

func normalizeDelay(v, fallback int) int {
	if v < 0 {
		return fallback
	}
	return v
}

// ------------------------------------------------------------ runtime target

func (s *Store) GetRuntimeTarget(ctx context.Context, targetID string) (RuntimeTarget, error) {
	var t RuntimeTarget
	var ct string
	var headers []byte
	err := s.pool.QueryRow(ctx, `
SELECT t.id, t.rule_id, t.role, p.id, p.name, p.base_url, p.auth_type,
       p.api_key_ciphertext, p.headers, p.timeout_ms, t.upstream_model, t.api_type,
       t.upstream_path, t.weight, t.priority,
       p.input_price_per_million::float8, p.output_price_per_million::float8
FROM routing_targets t JOIN providers p ON p.id=t.provider_id
WHERE t.id=$1 AND t.enabled=TRUE AND p.enabled=TRUE`, targetID).Scan(
		&t.TargetID, &t.RuleID, &t.Role, &t.ProviderID, &t.ProviderName, &t.BaseURL,
		&t.AuthType, &ct, &headers, &t.TimeoutMS, &t.UpstreamModel, &t.APIType,
		&t.UpstreamPath, &t.Weight, &t.Priority, &t.InputPrice, &t.OutputPrice)
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
	return t, nil
}

func (s *Store) ListHealthProbeTargets(ctx context.Context) ([]HealthProbeTarget, error) {
	rows, err := s.pool.Query(ctx, `
SELECT t.id, p.id, p.name, p.base_url, p.auth_type, p.api_key_ciphertext, p.headers,
       p.health_check_url, p.health_check_method, p.timeout_ms, t.upstream_model,
       t.api_type, t.upstream_path
FROM routing_targets t JOIN providers p ON p.id=t.provider_id
WHERE t.enabled=TRUE AND p.enabled=TRUE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]HealthProbeTarget, 0)
	for rows.Next() {
		var t HealthProbeTarget
		var ct string
		var headers []byte
		if err := rows.Scan(&t.TargetID, &t.ProviderID, &t.ProviderName, &t.BaseURL, &t.AuthType,
			&ct, &headers, &t.HealthCheckURL, &t.HealthCheckMethod, &t.TimeoutMS,
			&t.UpstreamModel, &t.APIType, &t.UpstreamPath); err != nil {
			return nil, err
		}
		if ct != "" {
			t.APIKey, _ = s.cipher.Decrypt(ct)
		}
		t.Headers = decodeHeaders(headers)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) UpdateTargetHealth(ctx context.Context, targetID string, healthy bool, latencyMS, statusCode int, errText string) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO target_health (target_id, healthy, checked_at, latency_ms, status_code, error)
VALUES ($1,$2,now(),$3,$4,$5)
ON CONFLICT (target_id) DO UPDATE SET healthy=EXCLUDED.healthy,
checked_at=EXCLUDED.checked_at, latency_ms=EXCLUDED.latency_ms,
status_code=EXCLUDED.status_code, error=EXCLUDED.error`,
		targetID, healthy, latencyMS, statusCode, errText)
	return err
}

// ---------------------------------------------------------------- api keys

const apiKeySelect = `SELECT id, name, app, key_prefix, allowed_models, enabled,
expires_at, last_used_at, created_at FROM api_keys`

func scanAPIKey(row rowScanner) (APIKey, error) {
	var k APIKey
	err := row.Scan(&k.ID, &k.Name, &k.App, &k.KeyPrefix, &k.AllowedModels, &k.Enabled,
		&k.ExpiresAt, &k.LastUsedAt, &k.CreatedAt)
	return k, err
}

func (s *Store) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	rows, err := s.pool.Query(ctx, apiKeySelect+` ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]APIKey, 0)
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) CreateAPIKey(ctx context.Context, in APIKeyInput) (APIKey, string, error) {
	plain, err := randomKey()
	if err != nil {
		return APIKey{}, "", err
	}
	prefix := plain
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	models := in.AllowedModels
	if models == nil {
		models = []string{}
	}
	var id string
	err = s.pool.QueryRow(ctx, `
INSERT INTO api_keys (name, app, key_prefix, key_hash, allowed_models, enabled, expires_at)
VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		in.Name, in.App, prefix, hashKey(plain), models, boolOr(in.Enabled, true),
		in.ExpiresAt).Scan(&id)
	if err != nil {
		return APIKey{}, "", err
	}
	k, err := scanAPIKey(s.pool.QueryRow(ctx, apiKeySelect+` WHERE id=$1`, id))
	return k, plain, err
}

func (s *Store) UpdateAPIKey(ctx context.Context, id string, in APIKeyInput) (APIKey, error) {
	models := in.AllowedModels
	if models == nil {
		models = []string{}
	}
	cmd, err := s.pool.Exec(ctx, `
UPDATE api_keys SET name=$2, app=$3, allowed_models=$4, enabled=$5, expires_at=$6
WHERE id=$1`, id, in.Name, in.App, models, boolOr(in.Enabled, true), in.ExpiresAt)
	if err != nil {
		return APIKey{}, err
	}
	if cmd.RowsAffected() == 0 {
		return APIKey{}, ErrNotFound
	}
	return scanAPIKey(s.pool.QueryRow(ctx, apiKeySelect+` WHERE id=$1`, id))
}

func (s *Store) DeleteAPIKey(ctx context.Context, id string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM api_keys WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AuthenticateAPIKey(ctx context.Context, plain string) (APIKey, error) {
	k, err := scanAPIKey(s.pool.QueryRow(ctx, apiKeySelect+`
WHERE key_hash=$1 AND enabled=TRUE AND (expires_at IS NULL OR expires_at > now())`,
		hashKey(plain)))
	if errors.Is(err, pgx.ErrNoRows) {
		return k, ErrNotFound
	}
	return k, err
}

func (s *Store) TouchAPIKey(ctx context.Context, id string) {
	_, _ = s.pool.Exec(ctx, `UPDATE api_keys SET last_used_at=now() WHERE id=$1`, id)
}

// -------------------------------------------------------------------- logs

func (s *Store) InsertRequestLog(ctx context.Context, l RequestLog) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO request_logs (request_id, api_type, model, upstream_model, app, api_key_name,
provider_name, route_name, status_code, latency_ms, prompt_tokens, completion_tokens,
total_tokens, cost, stream, attempt_count, fallback_used, fallback_policy, retry_policy,
error, client_ip)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		l.RequestID, l.APIType, l.Model, l.UpstreamModel, l.App, l.APIKeyName, l.ProviderName,
		l.RouteName, l.StatusCode, l.LatencyMS, l.PromptTokens, l.CompletionTokens, l.TotalTokens,
		l.Cost, l.Stream, l.AttemptCount, l.FallbackUsed, l.FallbackPolicy, l.RetryPolicy,
		l.Error, l.ClientIP)
	return err
}

func (s *Store) ListLogs(ctx context.Context, f LogFilter) ([]RequestLog, int64, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	where := []string{"1=1"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if f.APIType != "" {
		add("api_type=$%d", f.APIType)
	}
	if f.Model != "" {
		add("model=$%d", f.Model)
	}
	if f.App != "" {
		add("app=$%d", f.App)
	}
	if f.KeyName != "" {
		add("api_key_name=$%d", f.KeyName)
	}
	if f.Provider != "" {
		add("provider_name=$%d", f.Provider)
	}
	if f.Status > 0 {
		add("status_code=$%d", f.Status)
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		where = append(where, fmt.Sprintf(
			"(model ILIKE $%d OR app ILIKE $%d OR api_key_name ILIKE $%d OR request_id ILIKE $%d)",
			len(args), len(args), len(args), len(args)))
	}
	if f.From != nil {
		add("created_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("created_at <= $%d", *f.To)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM request_logs WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
SELECT id, created_at, request_id, api_type, model, upstream_model, app, api_key_name,
       provider_name, route_name, status_code, latency_ms, prompt_tokens, completion_tokens,
       total_tokens, cost::float8, stream, attempt_count, fallback_used, fallback_policy,
       retry_policy, error, client_ip
FROM request_logs WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereSQL, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]RequestLog, 0)
	for rows.Next() {
		var l RequestLog
		err := rows.Scan(&l.ID, &l.CreatedAt, &l.RequestID, &l.APIType, &l.Model, &l.UpstreamModel,
			&l.App, &l.APIKeyName, &l.ProviderName, &l.RouteName, &l.StatusCode, &l.LatencyMS,
			&l.PromptTokens, &l.CompletionTokens, &l.TotalTokens, &l.Cost, &l.Stream,
			&l.AttemptCount, &l.FallbackUsed, &l.FallbackPolicy, &l.RetryPolicy, &l.Error, &l.ClientIP)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

func (s *Store) GetStats(ctx context.Context, since time.Time) (Stats, error) {
	st := Stats{
		RequestsByAPI:   map[string]int64{},
		RequestsByModel: map[string]int64{},
		TopApps:         []StatBreakdown{},
		RecentSeries:    []TimeSeriesPoint{},
	}
	err := s.pool.QueryRow(ctx, `
SELECT COUNT(*),
       COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 400),
       COUNT(*) FILTER (WHERE status_code >= 400 OR status_code = 0),
       COALESCE(AVG(latency_ms),0)::float8,
       COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms),0)::float8,
       COALESCE(SUM(total_tokens),0)::bigint,
       COALESCE(SUM(prompt_tokens),0)::bigint,
       COALESCE(SUM(completion_tokens),0)::bigint,
       COALESCE(SUM(cost),0)::float8
FROM request_logs WHERE created_at >= $1`, since).Scan(
		&st.TotalRequests, &st.SuccessRequests, &st.FailedRequests, &st.AvgLatencyMS,
		&st.P95LatencyMS, &st.TotalTokens, &st.PromptTokens, &st.CompletionTokens, &st.TotalCost)
	if err != nil {
		return st, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT api_type, COUNT(*) FROM request_logs WHERE created_at >= $1
GROUP BY api_type`, since)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var k string
		var v int64
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return st, err
		}
		st.RequestsByAPI[k] = v
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `
SELECT model, COUNT(*) FROM request_logs WHERE created_at >= $1
GROUP BY model ORDER BY COUNT(*) DESC LIMIT 10`, since)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var k string
		var v int64
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return st, err
		}
		st.RequestsByModel[k] = v
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `
SELECT app, COUNT(*), COALESCE(SUM(cost),0)::float8 FROM request_logs
WHERE created_at >= $1 AND app <> '' GROUP BY app ORDER BY COUNT(*) DESC LIMIT 10`, since)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var b StatBreakdown
		if err := rows.Scan(&b.Name, &b.Count, &b.Cost); err != nil {
			rows.Close()
			return st, err
		}
		st.TopApps = append(st.TopApps, b)
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `
SELECT date_trunc('hour', created_at) AS bucket, COUNT(*),
       COALESCE(SUM(total_tokens),0)::bigint, COALESCE(SUM(cost),0)::float8
FROM request_logs WHERE created_at >= $1
GROUP BY bucket ORDER BY bucket ASC`, since)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var p TimeSeriesPoint
		if err := rows.Scan(&p.Time, &p.Requests, &p.Tokens, &p.Cost); err != nil {
			return st, err
		}
		st.RecentSeries = append(st.RecentSeries, p)
	}
	return st, rows.Err()
}

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
