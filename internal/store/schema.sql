CREATE TABLE IF NOT EXISTS providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    base_url TEXT NOT NULL,
    auth_type TEXT NOT NULL DEFAULT 'bearer' CHECK (auth_type IN ('bearer', 'x-api-key', 'none')),
    api_key_ciphertext TEXT NOT NULL DEFAULT '',
    api_key_last4 TEXT NOT NULL DEFAULT '',
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    health_check_url TEXT NOT NULL DEFAULT '',
    health_check_method TEXT NOT NULL DEFAULT 'POST',
    timeout_ms INTEGER NOT NULL DEFAULT 120000 CHECK (timeout_ms > 0),
    input_price_per_million NUMERIC(14,6) NOT NULL DEFAULT 0,
    output_price_per_million NUMERIC(14,6) NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    api_types TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
    path_overrides JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE providers ADD COLUMN IF NOT EXISTS api_types TEXT[] NOT NULL DEFAULT '{}'::TEXT[];
ALTER TABLE providers ADD COLUMN IF NOT EXISTS path_overrides JSONB NOT NULL DEFAULT '{}'::jsonb;

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
    api_type TEXT NOT NULL DEFAULT 'chat.completions' CHECK (api_type IN ('chat.completions', 'responses', 'messages')),
    upstream_path TEXT NOT NULL DEFAULT '/v1/chat/completions',
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
