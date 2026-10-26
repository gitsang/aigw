# aigw

AI Gateway — one unified API entrypoint for LLM traffic, with translation between
OpenAI-compatible and Anthropic-compatible APIs, weighted model routing, health
checks, retries/fallbacks, and a SvelteKit management console.

> Status: working implementation, covered by unit tests and an end-to-end smoke test
> against mock providers and PostgreSQL 18.

## Features

- **Unified LLM entrypoint**
  - `POST /v1/chat/completions` (OpenAI Chat Completions)
  - `POST /v1/responses` (OpenAI Responses)
  - `POST /v1/messages` and `POST /messages` (Anthropic Messages)
- **Bidirectional protocol translation**
  - Every inbound API type can be routed to any configured upstream API type —
    request bodies, non-streaming responses and SSE streams are converted through
    a provider-neutral IR.
- **Model routing**
  - Virtual model names map to a routing rule.
  - Weighted LB across healthy targets (smooth exponential-race weighted shuffle).
  - Explicit `fallback` targets used when all LB targets are unhealthy/failing.
  - Retry policies: `fixed` and `exponential` backoff.
  - Fallback strategies: `fallback-first` and `retry-first`.
- **Health checks**
  - Background prober runs every minute (`health.interval`), probe timeout and
    concurrency are configurable.
  - Unhealthy targets are removed from LB immediately; fallback remains available.
  - Health probe sends a minimal `max_tokens: 1` request to the upstream and checks
    for a 2xx response without an `error` payload. Per-target health, last check
    time, latency, status code and error are stored and displayed in the UI.
- **Multi API key management**
  - Create/list/update/delete client API keys from the UI.
  - Only a SHA-256 hash is stored; the plaintext key is shown exactly once.
  - Optional app label, allowed-model allowlist, expiry time and enable flag.
  - Provider secrets are encrypted at rest with AES-256-GCM.
- **Observability**
  - Full request log: time, ApiType, model, upstream model, app, API key name,
    provider, route, status, latency, prompt/completion/total tokens, cost,
    stream flag, attempt count, fallback used, retry/fallback policy, error, client IP.
  - Dashboard: request counts, success rate, avg/P95 latency, token totals, cost,
    model distribution, per-hour request series, app breakdown.
- **SvelteKit management console** with pages for:
  - Model Provider
  - Routing
  - API Keys
  - Logs & statistics

## Architecture

```
cmd/aigw              main entrypoint
configs/              example YAML config
internal/
  admin/              authenticated management REST API
  app/                gateway request pipeline, retries, logging
  config/             YAML + env config
  crypto/             AES-256-GCM secret encryption
  health/             periodic upstream health prober
  router/             weighted LB and retry/fallback policies
  server/             chi router, middleware, static frontend serving
  store/              pgx pool, schema, queries
  translate/          Chat Completions ⇄ Responses ⇄ Messages IR + SSE conversion
  upstream/           outbound HTTP client and URL/auth handling
pkg/sse/              SSE reader/writer helpers
web/                  SvelteKit frontend
```

Request flow:

```
client → API key auth → protocol parse (IR) → routing rule lookup
       → weighted healthy LB / fallback with retry & backoff
       → upstream protocol build → upstream call
       → response or SSE translation → client
       → async usage/cost/attempt logging → PostgreSQL
```

## Quick start

### Docker Compose (recommended)

```bash
cp compose.example.yml compose.yml
# edit the admin token and encryption key
docker compose -f compose.yml up -d --build
```

Open <http://localhost:8080>, log in with `AIGW_ADMIN_TOKEN` (default `change-me`),
then add a provider, a routing rule and an API key.

### Local development

Requirements: Go 1.25+, Node 22+, PostgreSQL 18.

```bash
cp configs/config.example.yml configs/config.yml
# edit database.url and secrets

make web-install
make web            # build frontend into web/build
make run            # starts the Go backend on :8080
# or: make dev       # backend + Vite dev server on :5173
```

Run tests:

```bash
make test
cd web && npm run check
```

### Configuration

See [`configs/config.example.yml`](configs/config.example.yml). Every important
value can also be set with environment variables such as:

| Env var | Purpose |
| --- | --- |
| `AIGW_CONFIG` | config file path |
| `AIGW_SERVER_ADDRESS` | listen address |
| `AIGW_ADMIN_TOKEN` | admin console/API token |
| `AIGW_DATABASE_URL` | PostgreSQL URL |
| `AIGW_ENCRYPTION_KEY` | AES key material for provider secrets |
| `AIGW_HEALTH_INTERVAL` | health check interval, e.g. `1m` |
| `AIGW_REQUEST_TIMEOUT` | per-request timeout |
| `AIGW_CORS_ORIGINS` | comma-separated CORS origins |

## Usage

Create an API key in the console, then:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer aigw_..." \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-flash","messages":[{"role":"user","content":"hello"}]}'

curl http://localhost:8080/v1/responses \
  -H "Authorization: Bearer aigw_..." \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-flash","input":"hello"}'

curl http://localhost:8080/v1/messages \
  -H "x-api-key: aigw_..." \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-flash","max_tokens":128,"messages":[{"role":"user","content":"hello"}]}'
```

Streaming (`"stream": true`) is supported for all three entrypoints, including
cross-protocol streaming conversion.

## Upstream API type vs Path

Each routing target has two related but independent fields:

- **`api_type` — upstream protocol.** `chat.completions`, `responses` or `messages`.
  It selects the JSON schema used to build the upstream request, parse the upstream
  response and convert its SSE stream. It also controls protocol-specific headers
  (for example `anthropic-version` for the Messages API).
- **`upstream_path` — transport path.** Appended to the provider `base_url` to form
  the final URL. It is only an optional override: when left empty the gateway uses
  the protocol default (`/v1/chat/completions`, `/v1/responses`, `/v1/messages`).
  Custom paths are needed for providers such as Azure OpenAI, Zhipu, or any
  self-hosted gateway that exposes the same protocol under a different route.

The management UI shows Path as optional and displays the protocol default as its
placeholder. The periodic health probe uses the same path unless a provider-level
`health_check_url` override is configured.

## Routing example

The requested configuration — `deepseek-flash` load-balanced 50/50 over two
providers' `/v1/chat/completions` with a provider3 fallback — maps to:

- Rule `virtual_model: deepseek-flash`
- LB target A: provider1, upstream model `deepseek/deepseek-flash`, weight 50
- LB target B: provider2, upstream model `deepseek/deepseek-flash`, weight 50
- Fallback target: provider3, upstream model `deepseek/deepseek-v4-flash`

Behavior:

- `fallback-first` with `retry_count: 3`: each cycle tries all healthy LBs then the
  fallback; the whole sequence repeats up to three cycles.
- `retry-first` with `retry_count: 3`: retries each route three times before moving
  to the next route, ending with the fallback.
- When both LB targets fail health checks they are skipped entirely and every
  request goes directly to the fallback.

The example rule can be created through the Routing page; providers with a
health endpoint and pricing for cost calculation live on the Model Provider page.

## API (admin)

All admin endpoints require `Authorization: Bearer <admin_token>`:

| Method | Path | Description |
| --- | --- | --- |
| GET | `/api/admin/session` | token check |
| GET | `/api/admin/providers` | list providers |
| POST | `/api/admin/providers` | create provider |
| PUT | `/api/admin/providers/{id}` | update provider |
| DELETE | `/api/admin/providers/{id}` | delete provider |
| GET | `/api/admin/routes` | list routing rules |
| POST | `/api/admin/routes` | create rule |
| PUT | `/api/admin/routes/{id}` | update rule |
| DELETE | `/api/admin/routes/{id}` | delete rule |
| GET | `/api/admin/keys` | list API keys |
| POST | `/api/admin/keys` | create key (returns plaintext once) |
| PUT | `/api/admin/keys/{id}` | update key |
| DELETE | `/api/admin/keys/{id}` | delete key |
| GET | `/api/admin/logs` | paginated/filterable request logs |
| GET | `/api/admin/stats?range=24h` | aggregated statistics |
| GET | `/healthz` | liveness |
| GET | `/readyz` | readiness (checks PostgreSQL) |

## Security notes

- Change `server.admin_token` and `security.encryption_key` before deployment.
- Provider API keys are encrypted at rest; client API keys are stored as hashes.
- If `security.encryption_key` changes, existing provider secrets can no longer be
  decrypted and must be re-entered.
- The gateway authenticates callers with either `Authorization: Bearer ...` or
  `x-api-key: ...`.

## License

MIT
