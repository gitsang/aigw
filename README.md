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
  - Every inbound protocol can be routed to any supported upstream protocol —
    request bodies, non-streaming responses and SSE streams are converted through
    a provider-neutral IR. An inbound protocol the upstream speaks is forwarded
    untouched; otherwise the gateway falls back to `chat.completions`, `responses`,
    then `messages`.
- **Model routing**
  - Virtual model names map to a configurable set of targets (edited on the
    console's **Virtual Model** page).
  - Weighted LB across healthy targets (smooth exponential-race weighted shuffle).
  - Explicit `fallback` targets used when all LB targets are unhealthy/failing.
  - Retry policies: `fixed` and `exponential` backoff.
  - Fallback strategies: `fallback-first` and `retry-first`.
- **Model catalog** — one page listing every model the gateway can serve:
  - Virtual models (served under their `virtual_model` name).
  - Provider models, exposed as `{providerId}/{modelId}` (provider slug, falling
    back to the provider name) with `{providerName}/{modelName}` as display name.
  - Model ids are joined verbatim: a provider model literally named
    `deepseek/deepseek-flash` under provider `yllm` is exposed as
    `yllm/deepseek/deepseek-flash`.
- **Auto model matching** (opt-in, Settings page)
  - When a request model has no virtual model, the gateway can fall back to the
    first provider model whose public id *contains* the requested name. A request
    for `deepseek/deepseek-flash` therefore resolves to
    `yllm/deepseek/deepseek-flash` when only `yllm` is configured.
  - The matched model keeps its provider-level protocol, path, retry and proxy
    settings; the request log records `auto-match` in `virtual_model_name`.
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
- **Cost accounting** — pricing is per model (input/output per 1M tokens); each log
  row stores computed cost based on the model actually used.
- **Observability**
  - Full request log: time, entry protocol, model, upstream model, app, API key name,
    provider, virtual model, status, latency, prompt/completion/total tokens, cost,
    stream flag, attempt count, fallback used, retry/fallback policy, error, client IP.
  - Dashboard: request counts, success rate, avg/P95 latency, token totals, cost,
    model distribution, per-hour request series, app breakdown.
- **SvelteKit management console**, built on
  [shadcn-svelte](https://www.shadcn-svelte.com) (Tailwind CSS v4) with pages for:
  - Models (virtual model + provider model catalog, read-only)
  - Model Provider
  - Virtual Model
  - API Keys
  - Logs & statistics
  - Settings (auto model matching)
  - Light/dark theme, responsive sidebar, toasts and accessible dialogs out of the box.

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
client → API key auth → protocol parse (IR) → virtual model lookup
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
then add a provider, a virtual model and an API key.

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

The console uses shadcn-svelte components generated into `web/src/lib/components/ui`
and the theme tokens in `web/src/app.css`. To add more primitives:

```bash
cd web
npx shadcn-svelte@latest add <component>   # e.g. accordion, command, calendar
```

The Vite dev server proxies `/api/admin` and `/v1` to `http://localhost:8080`; set
`AIGW_DEV_BACKEND` to point it at another backend, e.g.
`AIGW_DEV_BACKEND=http://localhost:8989 make dev`.

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
| `AIGW_MODELSDEV_URL` | models.dev catalog URL (empty disables price sync) |
| `AIGW_MODELSDEV_TTL` | how long the catalog stays cached, e.g. `6h` |
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

## Provider configuration → models → virtual models

An **upstream protocol** (the `api_types` field in the API) is one of
`chat.completions`, `responses` or `messages` — the request/response schema an
upstream speaks. It is configured **per provider and per model**, never per
target and never per virtual model:

1. **Provider level** — each provider declares which upstream protocols it supports
   (`chat.completions`, `responses`, `messages`) and an optional default path per
   protocol. Empty list means the built-in defaults (all three).
2. **Model level** — models exist only under a provider. They can be pulled from the
   provider's OpenAI-compatible `GET /models` endpoint or added manually. Every model
   may override the provider's protocols, add a path override per protocol, and
   carries its own **input/output price per 1M tokens** used for cost accounting.
3. **Virtual model** — the console's **Virtual Model** page edits a virtual model:
   a virtual model ID (what clients send as `model`) plus its target set (provider +
   one of its models, role and weight) and its retry/fallback settings. A virtual
   model accepts **every** entry protocol; nothing about protocols is configured
   there.

Resolution rules for an inbound request:

- A target always speaks the protocols its model/provider supports, resolved as
  `model protocols > provider protocols > built-in defaults`.
- **The inbound protocol is forwarded as-is** whenever that upstream supports it —
  no translation happens in the common case.
- If the upstream does not support it, the gateway falls back in the fixed order
  **`chat.completions` → `responses` → `messages`** and takes the first protocol
  the upstream supports, translating the request/response/stream as needed.
- Path precedence: model path override > provider path override > built-in
  default. Targets do not carry a path or protocol field.
- Health probes use the same resolved protocol/path (an empty inbound protocol
  resolves to the first supported protocol in the order above).
- Auto model matching (Settings page) resolves a request model that has no
  virtual model to the first provider model whose `{providerId}/{modelId}` id
  contains the requested name, case-insensitively; the matched model only
  contributes its provider/model settings and uses the same inbound-protocol-first
  resolution.

Example: two targets whose upstreams support `chat.completions + responses`
(provider1) and `responses + messages` (provider2).

| Inbound | provider1 target | provider2 target |
| --- | --- | --- |
| Chat Completions | Chat Completions (exact match) | Responses (translated) |
| Responses | Responses (exact match) | Responses (exact match) |
| Messages | `responses` (translated) | Messages (exact match) |

Streaming is a request-body flag (`"stream": true`), not a separate protocol: it
is never part of `api_types`.

### Model pricing from models.dev

The models tab has two price-sync entry points backed by the
[models.dev](https://models.dev/) catalog (USD per 1M tokens):

- **Per model** — `从 models.dev 同步` in the price section of the model editor
  fills in input/output/cache prices, tiered pricing, and any missing context
  window / max output tokens. Nothing is written until the model is saved.
- **Per provider** — `同步价格` previews the whole model list (old price, new
  price, source, action) and writes only after you confirm. Existing prices are
  kept unless `overwrite` is enabled.

Providers are matched to the catalog by slug, then by API base URL host, then by
display name. Aggregators usually match none of these, and models.dev lists the
same model id under many upstreams at genuinely different prices, so the bulk
dialog has a **source picker** listing the catalog providers that cover the most
of your models. Automatic matching only applies a result when a single source
scores best; otherwise the dialog asks you to pick one.

The catalog is downloaded once and cached in memory (`modelsdev.url`,
`modelsdev.ttl`). Set `modelsdev.url` to an empty string to disable the feature;
the endpoints then return `503`.

## Virtual model example

The requested configuration — `deepseek-flash` load-balanced 50/50 over two
providers' `/v1/chat/completions` with a provider3 fallback — maps to:

- Virtual model `deepseek-flash` (`virtual_model: deepseek-flash`)
- LB target A: provider1 → model `deepseek/deepseek-flash`, weight 50
- LB target B: provider2 → model `deepseek/deepseek-flash`, weight 50
- Fallback target: provider3 → model `deepseek/deepseek-v4-flash`

Behavior:

- `fallback-first` with `retry_count: 3`: each cycle tries all healthy LBs then the
  fallback; the whole sequence repeats up to three cycles.
- `retry-first` with `retry_count: 3`: retries each target three times before moving
  to the next target, ending with the fallback.
- When both LB targets fail health checks they are skipped entirely and every
  request goes directly to the fallback.

The example virtual model can be created through the **Virtual Model** page;
providers with a health endpoint and pricing for cost calculation live on the
Model Provider page.

## API (admin)

All admin endpoints require `Authorization: Bearer <admin_token>`:

| Method | Path | Description |
| --- | --- | --- |
| GET | `/api/admin/session` | token check |
| GET | `/api/admin/models` | list all virtual model and provider models |
| GET | `/api/admin/settings` | read gateway settings |
| PUT | `/api/admin/settings` | update gateway settings (auto model match) |
| GET | `/api/admin/providers` | list providers |
| POST | `/api/admin/providers` | create provider |
| PUT | `/api/admin/providers/{id}` | update provider |
| DELETE | `/api/admin/providers/{id}` | delete provider |
| GET | `/api/admin/providers/{id}/models` | list provider models |
| POST | `/api/admin/providers/{id}/models` | add a model manually |
| POST | `/api/admin/providers/{id}/models/discover` | sync models from `/models` |
| GET | `/api/admin/providers/{id}/models/modelsdev?model_id=...` | look up a model's prices on models.dev |
| POST | `/api/admin/providers/{id}/models/sync-prices` | preview (`dry_run`) or apply models.dev prices |
| PUT | `/api/admin/providers/{id}/models/{modelID}` | update model protocols/paths |
| DELETE | `/api/admin/providers/{id}/models/{modelID}` | delete model |
| GET | `/api/admin/virtual-models` | list virtual models |
| POST | `/api/admin/virtual-models` | create virtual model |
| PUT | `/api/admin/virtual-models/{id}` | update virtual model |
| DELETE | `/api/admin/virtual-models/{id}` | delete virtual model |
| GET | `/api/admin/keys` | list API keys |
| POST | `/api/admin/keys` | create key (returns plaintext once) |
| PUT | `/api/admin/keys/{id}` | update key |
| DELETE | `/api/admin/keys/{id}` | delete key |
| GET | `/api/admin/logs` | paginated/filterable request logs |
| GET | `/api/admin/stats?range=24h` | aggregated statistics |
| GET | `/healthz` | liveness |
| GET | `/readyz` | readiness (checks PostgreSQL) |

## Upgrading from "routing"

The configuration entity used to be called a **routing rule** (`routing_rules` /
`routing_targets`, `/api/admin/routes`, log field `route_name`, settings key
`auto_route_match`). It is now a **virtual model** end to end. The migration is
automatic on startup (`database.migrate: true`) and keeps existing data: the
tables and columns are renamed in place. Compatibility shims:

| Legacy | New | Kept for |
| --- | --- | --- |
| console URL `/routing` | `/virtual-models` | a 301 redirect |
| `GET/POST/PUT/DELETE /api/admin/routes` | `/api/admin/virtual-models` | a server-side alias |
| `auto_route_match` settings key | `auto_model_match` | read on load (migrated transparently), written back under the new key |
| `route_name` log column | `virtual_model_name` | renamed in place, history preserved |
| config block `routing:` | `gateway:` | still parsed when no `gateway` block is present |

## Security notes

- Change `server.admin_token` and `security.encryption_key` before deployment.
- Provider API keys are encrypted at rest; client API keys are stored as hashes.
- If `security.encryption_key` changes, existing provider secrets can no longer be
  decrypted and must be re-entered.
- The gateway authenticates callers with either `Authorization: Bearer ...` or
  `x-api-key: ...`.

## License

MIT
