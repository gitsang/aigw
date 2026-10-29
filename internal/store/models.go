package store

import (
	"time"
)

const (
	APIChatCompletions = "chat.completions"
	APIResponses       = "responses"
	APIMessages        = "messages"

	RoleLB       = "lb"
	RoleFallback = "fallback"
)

// Proxy types accepted by a provider. An empty ProxyType disables the proxy.
const (
	ProxyNone   = ""
	ProxyHTTP   = "http"
	ProxySOCKS5 = "socks5"
)

// Retry backoff strategies for per-provider request retries.
const (
	BackoffExponential = "exponential"
	BackoffLinear      = "linear"
)

type Provider struct {
	ID                string            `json:"id"`
	Slug              string            `json:"slug"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	BaseURL           string            `json:"base_url"`
	AuthType          string            `json:"auth_type"`
	APIKeyLast4       string            `json:"api_key_last4"`
	HasAPIKey         bool              `json:"has_api_key"`
	Headers           map[string]string `json:"headers"`
	HealthCheckURL    string            `json:"health_check_url"`
	HealthCheckMethod string            `json:"health_check_method"`
	TimeoutMS         int               `json:"timeout_ms"`
	APITypes          []string          `json:"api_types"`
	PathOverrides     map[string]string `json:"path_overrides"`
	Enabled           bool              `json:"enabled"`

	// Per-provider request retry. RetryCount of 0 leaves the routing rule as the
	// only retry layer.
	RetryCount            int    `json:"retry_count"`
	RetryBackoff          string `json:"retry_backoff"`
	RetryInitialMS        int    `json:"retry_initial_ms"`
	RetryMaxMS            int    `json:"retry_max_ms"`
	RetryLinearIntervalMS int    `json:"retry_linear_interval_ms"`

	// Outbound proxy settings.
	ProxyType          string `json:"proxy_type"`
	ProxyURL           string `json:"proxy_url"`
	ProxyUsername      string `json:"proxy_username"`
	HasProxyPassword   bool   `json:"has_proxy_password"`
	ProxyPasswordLast4 string `json:"proxy_password_last4"`

	// Health check overrides. An interval or timeout of 0 uses the global config.
	HealthCheckHeaders    map[string]string `json:"health_check_headers"`
	HealthCheckBody       string            `json:"health_check_body"`
	HealthCheckIntervalMS int               `json:"health_check_interval_ms"`
	HealthCheckTimeoutMS  int               `json:"health_check_timeout_ms"`

	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	HealthyTargets int       `json:"healthy_targets,omitempty"`
	TotalTargets   int       `json:"total_targets,omitempty"`
}

type ProviderInput struct {
	Slug              string            `json:"slug"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	BaseURL           string            `json:"base_url"`
	AuthType          string            `json:"auth_type"`
	APIKey            string            `json:"api_key"`
	Headers           map[string]string `json:"headers"`
	HealthCheckURL    string            `json:"health_check_url"`
	HealthCheckMethod string            `json:"health_check_method"`
	TimeoutMS         int               `json:"timeout_ms"`
	APITypes          []string          `json:"api_types"`
	PathOverrides     map[string]string `json:"path_overrides"`
	Enabled           *bool             `json:"enabled"`

	RetryCount            int    `json:"retry_count"`
	RetryBackoff          string `json:"retry_backoff"`
	RetryInitialMS        int    `json:"retry_initial_ms"`
	RetryMaxMS            int    `json:"retry_max_ms"`
	RetryLinearIntervalMS int    `json:"retry_linear_interval_ms"`

	ProxyType     string `json:"proxy_type"`
	ProxyURL      string `json:"proxy_url"`
	ProxyUsername string `json:"proxy_username"`
	ProxyPassword string `json:"proxy_password"`

	HealthCheckHeaders    map[string]string `json:"health_check_headers"`
	HealthCheckBody       string            `json:"health_check_body"`
	HealthCheckIntervalMS int               `json:"health_check_interval_ms"`
	HealthCheckTimeoutMS  int               `json:"health_check_timeout_ms"`
}

type RouteTarget struct {
	ID                    string            `json:"id"`
	RuleID                string            `json:"rule_id"`
	Role                  string            `json:"role"`
	ProviderID            string            `json:"provider_id"`
	ProviderName          string            `json:"provider_name"`
	ProviderBaseURL       string            `json:"provider_base_url,omitempty"`
	ProviderModelID       string            `json:"provider_model_id,omitempty"`
	UpstreamModel         string            `json:"upstream_model"`
	APITypes              []string          `json:"api_types"`
	SupportedAPITypes     []string          `json:"supported_api_types,omitempty"`
	PathOverrides         map[string]string `json:"path_overrides,omitempty"`
	ProviderAPITypes      []string          `json:"provider_api_types,omitempty"`
	ProviderPathOverrides map[string]string `json:"provider_path_overrides,omitempty"`
	Weight                int               `json:"weight"`
	Priority              int               `json:"priority"`
	Enabled               bool              `json:"enabled"`
	Health                *TargetHealth     `json:"health,omitempty"`
}

type TargetHealth struct {
	Healthy    bool       `json:"healthy"`
	CheckedAt  *time.Time `json:"checked_at"`
	LatencyMS  int        `json:"latency_ms"`
	StatusCode int        `json:"status_code"`
	Error      string     `json:"error"`
}

type RoutingRule struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	VirtualModel   string        `json:"virtual_model"`
	APITypes       []string      `json:"api_types"`
	Enabled        bool          `json:"enabled"`
	RetryCount     int           `json:"retry_count"`
	RetryPolicy    string        `json:"retry_policy"`
	FallbackPolicy string        `json:"fallback_policy"`
	RetryDelayMS   int           `json:"retry_delay_ms"`
	MaxBackoffMS   int           `json:"max_backoff_ms"`
	Targets        []RouteTarget `json:"targets"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type RoutingRuleInput struct {
	Name           string             `json:"name"`
	VirtualModel   string             `json:"virtual_model"`
	APITypes       []string           `json:"api_types"`
	Enabled        *bool              `json:"enabled"`
	RetryCount     int                `json:"retry_count"`
	RetryPolicy    string             `json:"retry_policy"`
	FallbackPolicy string             `json:"fallback_policy"`
	RetryDelayMS   int                `json:"retry_delay_ms"`
	MaxBackoffMS   int                `json:"max_backoff_ms"`
	Targets        []RouteTargetInput `json:"targets"`
}

type RouteTargetInput struct {
	ID              string   `json:"id,omitempty"`
	Role            string   `json:"role"`
	ProviderID      string   `json:"provider_id"`
	ProviderModelID string   `json:"provider_model_id,omitempty"`
	UpstreamModel   string   `json:"upstream_model"`
	APITypes        []string `json:"api_types"`
	Weight          int      `json:"weight"`
	Priority        int      `json:"priority"`
	Enabled         *bool    `json:"enabled"`
}

// CostTier is a price bracket that applies once the request's input token
// count exceeds InputTokensAbove.
type CostTier struct {
	InputTokensAbove int     `json:"input_tokens_above"`
	Input            float64 `json:"input"`
	Output           float64 `json:"output"`
	CacheRead        float64 `json:"cache_read"`
	CacheWrite       float64 `json:"cache_write"`
}

// ModelCost is the per-million-token price list for a model, in an ISO 4217
// currency. Tiers override the flat prices once the input token count is
// exceeded; an empty tier list means flat pricing always applies.
type ModelCost struct {
	Currency   string     `json:"currency"`
	Input      float64    `json:"input"`
	Output     float64    `json:"output"`
	CacheRead  float64    `json:"cache_read"`
	CacheWrite float64    `json:"cache_write"`
	Tiers      []CostTier `json:"tiers"`
}

// DefaultModelCost is the price list used when a model has none configured.
func DefaultModelCost() ModelCost {
	return ModelCost{Currency: "USD", Tiers: []CostTier{}}
}

// ProviderModel is one upstream model exposed by a provider.
type ProviderModel struct {
	ID              string            `json:"id"`
	ProviderID      string            `json:"provider_id"`
	ModelID         string            `json:"model_id"`
	DisplayName     string            `json:"display_name"`
	Source          string            `json:"source"`
	APITypes        []string          `json:"api_types"`
	PathOverrides   map[string]string `json:"path_overrides"`
	ContextWindow   int               `json:"context_window"`
	MaxOutputTokens int               `json:"max_output_tokens"`
	Cost            ModelCost         `json:"cost"`
	Enabled         bool              `json:"enabled"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type ProviderModelInput struct {
	ModelID         string            `json:"model_id"`
	DisplayName     string            `json:"display_name"`
	APITypes        []string          `json:"api_types"`
	PathOverrides   map[string]string `json:"path_overrides"`
	ContextWindow   int               `json:"context_window"`
	MaxOutputTokens int               `json:"max_output_tokens"`
	Cost            ModelCost         `json:"cost"`
	Enabled         *bool             `json:"enabled"`
}

// APITypeResolution is the effective protocol/path for one model or target.
type APITypeResolution struct {
	APIType string `json:"api_type"`
	Path    string `json:"path"`
	Model   string `json:"model,omitempty"`
}

type APIKey struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	App           string     `json:"app"`
	KeyPrefix     string     `json:"key_prefix"`
	AllowedModels []string   `json:"allowed_models"`
	Enabled       bool       `json:"enabled"`
	ExpiresAt     *time.Time `json:"expires_at"`
	LastUsedAt    *time.Time `json:"last_used_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

type APIKeyInput struct {
	Name          string     `json:"name"`
	App           string     `json:"app"`
	AllowedModels []string   `json:"allowed_models"`
	Enabled       *bool      `json:"enabled"`
	ExpiresAt     *time.Time `json:"expires_at"`
}

type RequestLog struct {
	ID               int64     `json:"id"`
	CreatedAt        time.Time `json:"time"`
	RequestID        string    `json:"request_id"`
	APIType          string    `json:"api_type"`
	Model            string    `json:"model"`
	UpstreamModel    string    `json:"upstream_model"`
	App              string    `json:"app"`
	APIKeyName       string    `json:"api_key_name"`
	ProviderName     string    `json:"provider_name"`
	RouteName        string    `json:"route_name"`
	StatusCode       int       `json:"status_code"`
	LatencyMS        int       `json:"latency_ms"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	Cost             float64   `json:"cost"`
	Stream           bool      `json:"stream"`
	AttemptCount     int       `json:"attempt_count"`
	FallbackUsed     bool      `json:"fallback_used"`
	FallbackPolicy   string    `json:"fallback_policy"`
	RetryPolicy      string    `json:"retry_policy"`
	Error            string    `json:"error"`
	ClientIP         string    `json:"client_ip"`
}

type LogFilter struct {
	Limit    int
	Offset   int
	APIType  string
	Model    string
	App      string
	KeyName  string
	Provider string
	Status   int
	Search   string
	From     *time.Time
	To       *time.Time
}

type Stats struct {
	TotalRequests    int64             `json:"total_requests"`
	SuccessRequests  int64             `json:"success_requests"`
	FailedRequests   int64             `json:"failed_requests"`
	AvgLatencyMS     float64           `json:"avg_latency_ms"`
	P95LatencyMS     float64           `json:"p95_latency_ms"`
	TotalTokens      int64             `json:"total_tokens"`
	PromptTokens     int64             `json:"prompt_tokens"`
	CompletionTokens int64             `json:"completion_tokens"`
	TotalCost        float64           `json:"total_cost"`
	RequestsByAPI    map[string]int64  `json:"requests_by_api"`
	RequestsByModel  map[string]int64  `json:"requests_by_model"`
	TopApps          []StatBreakdown   `json:"top_apps"`
	RecentSeries     []TimeSeriesPoint `json:"recent_series"`
}

type StatBreakdown struct {
	Name  string  `json:"name"`
	Count int64   `json:"count"`
	Cost  float64 `json:"cost"`
}

type TimeSeriesPoint struct {
	Time     time.Time `json:"time"`
	Requests int64     `json:"requests"`
	Tokens   int64     `json:"tokens"`
	Cost     float64   `json:"cost"`
}
