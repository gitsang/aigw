CREATE TABLE IF NOT EXISTS providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    base_url TEXT NOT NULL,
    auth_type TEXT NOT NULL DEFAULT 'bearer' CHECK (auth_type IN ('bearer', 'x-api-key', 'none')),
    api_key_ciphertext TEXT NOT NULL DEFAULT '',
    api_key_last4 TEXT NOT NULL DEFAULT '',
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    timeout_ms INTEGER NOT NULL DEFAULT 120000 CHECK (timeout_ms > 0),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    api_types TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
    path_overrides JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE providers ADD COLUMN IF NOT EXISTS api_types TEXT[] NOT NULL DEFAULT '{}'::TEXT[];
ALTER TABLE providers ADD COLUMN IF NOT EXISTS path_overrides JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE providers DROP COLUMN IF EXISTS input_price_per_million;
ALTER TABLE providers DROP COLUMN IF EXISTS output_price_per_million;

CREATE TABLE IF NOT EXISTS routing_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    virtual_model TEXT NOT NULL UNIQUE,
    api_types TEXT[] NOT NULL DEFAULT ARRAY['chat.completions','responses','messages']::TEXT[],
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    retry_count INTEGER NOT NULL DEFAULT 3 CHECK (retry_count BETWEEN 1 AND 20),
    retry_policy TEXT NOT NULL DEFAULT 'exponential' CHECK (retry_policy IN ('fixed', 'exponential')),
    fallback_policy TEXT NOT NULL DEFAULT 'fallback-first' CHECK (fallback_policy IN ('fallback-first', 'retry-first')),
    retry_delay_ms INTEGER NOT NULL DEFAULT 250 CHECK (retry_delay_ms >= 0),
    max_backoff_ms INTEGER NOT NULL DEFAULT 5000 CHECK (max_backoff_ms >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS routing_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id UUID NOT NULL REFERENCES routing_rules(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'lb' CHECK (role IN ('lb', 'fallback')),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
    upstream_model TEXT NOT NULL,
    api_types TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
    weight INTEGER NOT NULL DEFAULT 100 CHECK (weight > 0),
    priority INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS target_health (
    target_id UUID PRIMARY KEY REFERENCES routing_targets(id) ON DELETE CASCADE,
    healthy BOOLEAN NOT NULL DEFAULT TRUE,
    checked_at TIMESTAMPTZ,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    status_code INTEGER NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    app TEXT NOT NULL DEFAULT '',
    key_prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    allowed_models TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS request_logs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    request_id TEXT NOT NULL DEFAULT '',
    api_type TEXT NOT NULL,
    model TEXT NOT NULL,
    upstream_model TEXT NOT NULL DEFAULT '',
    app TEXT NOT NULL DEFAULT '',
    api_key_name TEXT NOT NULL DEFAULT '',
    provider_name TEXT NOT NULL DEFAULT '',
    route_name TEXT NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    cost NUMERIC(16,8) NOT NULL DEFAULT 0,
    stream BOOLEAN NOT NULL DEFAULT FALSE,
    attempt_count INTEGER NOT NULL DEFAULT 1,
    fallback_used BOOLEAN NOT NULL DEFAULT FALSE,
    fallback_policy TEXT NOT NULL DEFAULT '',
    retry_policy TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    client_ip TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_request_logs_created_at ON request_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_model ON request_logs(model);
CREATE INDEX IF NOT EXISTS idx_request_logs_app ON request_logs(app);
CREATE INDEX IF NOT EXISTS idx_request_logs_api_type ON request_logs(api_type);
CREATE INDEX IF NOT EXISTS idx_routing_targets_rule_id ON routing_targets(rule_id);
CREATE INDEX IF NOT EXISTS idx_routing_targets_provider_id ON routing_targets(provider_id);


-- ------------------------------------------------------ provider model catalog
CREATE TABLE IF NOT EXISTS provider_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'discovered')),
    api_types TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
    path_overrides JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider_id, model_id)
);
CREATE INDEX IF NOT EXISTS idx_provider_models_provider_id ON provider_models(provider_id);

ALTER TABLE routing_targets
    ADD COLUMN IF NOT EXISTS provider_model_id UUID REFERENCES provider_models(id) ON DELETE SET NULL;
ALTER TABLE routing_targets
    ADD COLUMN IF NOT EXISTS api_types TEXT[] NOT NULL DEFAULT '{}'::TEXT[];
ALTER TABLE routing_targets DROP COLUMN IF EXISTS api_type;
ALTER TABLE routing_targets DROP COLUMN IF EXISTS upstream_path;

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS providers_set_updated_at ON providers;
CREATE TRIGGER providers_set_updated_at BEFORE UPDATE ON providers
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS routing_rules_set_updated_at ON routing_rules;
CREATE TRIGGER routing_rules_set_updated_at BEFORE UPDATE ON routing_rules
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS routing_targets_set_updated_at ON routing_targets;
CREATE TRIGGER routing_targets_set_updated_at BEFORE UPDATE ON routing_targets
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS provider_models_set_updated_at ON provider_models;
CREATE TRIGGER provider_models_set_updated_at BEFORE UPDATE ON provider_models
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ------------------------------------------------- provider behavior settings
ALTER TABLE providers ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';

-- Per-provider request retry, applied around each upstream call. A retry count of
-- 0 keeps the routing-rule policy as the only retry layer.
ALTER TABLE providers ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0
    CHECK (retry_count BETWEEN 0 AND 20);
ALTER TABLE providers ADD COLUMN IF NOT EXISTS retry_backoff TEXT NOT NULL DEFAULT 'exponential'
    CHECK (retry_backoff IN ('exponential', 'linear'));
ALTER TABLE providers ADD COLUMN IF NOT EXISTS retry_initial_ms INTEGER NOT NULL DEFAULT 500
    CHECK (retry_initial_ms >= 0);
ALTER TABLE providers ADD COLUMN IF NOT EXISTS retry_max_ms INTEGER NOT NULL DEFAULT 10000
    CHECK (retry_max_ms >= 0);
ALTER TABLE providers ADD COLUMN IF NOT EXISTS retry_linear_interval_ms INTEGER NOT NULL DEFAULT 5000
    CHECK (retry_linear_interval_ms >= 0);

-- Outbound proxy. proxy_type '' means "connect directly".
ALTER TABLE providers ADD COLUMN IF NOT EXISTS proxy_type TEXT NOT NULL DEFAULT ''
    CHECK (proxy_type IN ('', 'http', 'socks5'));
ALTER TABLE providers ADD COLUMN IF NOT EXISTS proxy_url TEXT NOT NULL DEFAULT '';
ALTER TABLE providers ADD COLUMN IF NOT EXISTS proxy_username TEXT NOT NULL DEFAULT '';
ALTER TABLE providers ADD COLUMN IF NOT EXISTS proxy_password_ciphertext TEXT NOT NULL DEFAULT '';
ALTER TABLE providers ADD COLUMN IF NOT EXISTS proxy_password_last4 TEXT NOT NULL DEFAULT '';

-- Provider-level health check overrides were removed; probes share the global
-- health.interval / health.timeout schedule.
ALTER TABLE providers DROP COLUMN IF EXISTS health_check_url;
ALTER TABLE providers DROP COLUMN IF EXISTS health_check_method;
ALTER TABLE providers DROP COLUMN IF EXISTS health_check_headers;
ALTER TABLE providers DROP COLUMN IF EXISTS health_check_body;
ALTER TABLE providers DROP COLUMN IF EXISTS health_check_interval_ms;
ALTER TABLE providers DROP COLUMN IF EXISTS health_check_timeout_ms;

-- ------------------------------------------------- provider slug + model costs
-- Short human-readable identifier shown next to the provider name. Backfilled
-- from the name for existing rows so the UI always has something to show.
ALTER TABLE providers ADD COLUMN IF NOT EXISTS slug TEXT NOT NULL DEFAULT '';
UPDATE providers SET slug = name WHERE slug = '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_providers_slug ON providers(slug) WHERE slug <> '';

-- Model context window and max output tokens (0 = unknown / provider default).
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS context_window INTEGER NOT NULL DEFAULT 0
    CHECK (context_window >= 0);
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS max_output_tokens INTEGER NOT NULL DEFAULT 0
    CHECK (max_output_tokens >= 0);

-- Flat per-million-token pricing. Currency is an ISO 4217 code.
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS cost_currency TEXT NOT NULL DEFAULT 'USD';
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS cost_input NUMERIC(14,6) NOT NULL DEFAULT 0;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS cost_output NUMERIC(14,6) NOT NULL DEFAULT 0;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS cost_cache_read NUMERIC(14,6) NOT NULL DEFAULT 0;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS cost_cache_write NUMERIC(14,6) NOT NULL DEFAULT 0;

-- Tiered pricing: [{"input_tokens_above":200000,"input":..,"output":..,
-- "cache_read":..,"cache_write":..}], applied once input tokens exceed the
-- threshold. Empty means flat pricing above.
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS cost_tiers JSONB NOT NULL DEFAULT '[]'::jsonb;

-- Seed the new pricing columns from the legacy pair, then retire them. Guarded
-- so re-running the schema on an already-migrated database is a no-op.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'provider_models' AND column_name = 'input_price_per_million') THEN
        UPDATE provider_models
        SET cost_input = input_price_per_million, cost_output = output_price_per_million
        WHERE cost_input = 0 AND cost_output = 0;
        ALTER TABLE provider_models DROP COLUMN input_price_per_million;
        ALTER TABLE provider_models DROP COLUMN output_price_per_million;
    END IF;
END $$;
