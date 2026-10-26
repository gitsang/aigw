package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gitsang/aigw/internal/config"
	"github.com/gitsang/aigw/internal/router"
	"github.com/gitsang/aigw/internal/store"
	"github.com/gitsang/aigw/internal/translate"
	"github.com/gitsang/aigw/internal/upstream"
	"github.com/gitsang/aigw/pkg/sse"
	"github.com/google/uuid"
)

type App struct {
	cfg      config.Config
	store    *store.Store
	upstream *upstream.Client
	runner   *router.Runner
	log      *slog.Logger
	started  time.Time
}

func New(cfg config.Config, st *store.Store, log *slog.Logger) *App {
	return &App{
		cfg: cfg, store: st, upstream: upstream.NewClient(), runner: router.NewRunner(),
		log: log, started: time.Now(),
	}
}

type attemptState struct {
	target     store.RuntimeTarget
	reader     *sse.Reader
	firstEvent *sse.Event
	parsed     translate.Response
	usage      translate.Usage
}

func apiTypeForPath(path string) string {
	switch path {
	case "/v1/responses":
		return translate.APIResponses
	case "/v1/messages", "/messages":
		return translate.APIMessages
	default:
		return translate.APIChat
	}
}

// Gateway handles an inbound LLM API request.
func (a *App) Gateway(apiType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := "req_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		w.Header().Set("X-Request-ID", requestID)

		apiKey, err := a.authenticateAPIKey(r)
		if err != nil {
			writeGatewayError(w, apiType, http.StatusUnauthorized, "invalid_api_key", "missing or invalid API key")
			return
		}
		defer a.store.TouchAPIKey(context.Background(), apiKey.ID)

		r.Body = http.MaxBytesReader(w, r.Body, a.cfg.Routing.MaxBodyBytes)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeGatewayError(w, apiType, http.StatusRequestEntityTooLarge, "invalid_request_error", "request body is too large or unreadable")
			return
		}

		req, err := translate.ParseRequest(apiType, body)
		if err != nil {
			writeGatewayError(w, apiType, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		if strings.TrimSpace(req.Model) == "" {
			writeGatewayError(w, apiType, http.StatusBadRequest, "invalid_request_error", "model is required")
			return
		}
		if len(apiKey.AllowedModels) > 0 && !contains(apiKey.AllowedModels, req.Model) {
			writeGatewayError(w, apiType, http.StatusForbidden, "model_not_allowed",
				fmt.Sprintf("API key is not allowed to use model %q", req.Model))
			return
		}

		rule, err := a.store.GetRoutingRuleByModel(r.Context(), req.Model)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeGatewayError(w, apiType, http.StatusNotFound, "model_not_found",
					fmt.Sprintf("no routing rule configured for model %q", req.Model))
				return
			}
			a.log.Error("load routing rule", "error", err)
			writeGatewayError(w, apiType, http.StatusInternalServerError, "gateway_error", "failed to load routing rule")
			return
		}
		if !contains(rule.APITypes, apiType) {
			writeGatewayError(w, apiType, http.StatusBadRequest, "invalid_request_error",
				fmt.Sprintf("model %q is not configured for API type %s", req.Model, apiType))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), a.cfg.Routing.RequestTimeout)
		defer cancel()

		state := &attemptState{}
		result := a.runner.Execute(ctx, rule, func(execCtx context.Context, target store.RouteTarget) router.AttemptOutcome {
			return a.attempt(execCtx, req, target, state)
		})

		logEntry := store.RequestLog{
			RequestID: requestID, APIType: apiType, Model: req.Model, App: apiKey.App,
			APIKeyName: apiKey.Name, RouteName: rule.Name, Stream: req.Stream,
			AttemptCount: len(result.Attempts), FallbackUsed: result.FallbackUsed,
			FallbackPolicy: rule.FallbackPolicy, RetryPolicy: rule.RetryPolicy,
			ClientIP: clientIP(r),
		}
		status := 0
		if result.Err == nil {
			status = http.StatusOK
		}
		defer func() {
			elapsed := time.Since(start)
			logEntry.StatusCode = status
			if logEntry.StatusCode == 0 {
				logEntry.StatusCode = http.StatusBadGateway
			}
			logEntry.LatencyMS = int(elapsed.Milliseconds())
			if result.Err != nil {
				logEntry.Error = result.Err.Error()
			}
			if state.target.ProviderName != "" {
				logEntry.ProviderName = state.target.ProviderName
			}
			if state.target.UpstreamModel != "" {
				logEntry.UpstreamModel = state.target.UpstreamModel
			}
			usage := state.usage
			if result.Err == nil && req.Stream {
				usage = state.usage
			}
			logEntry.PromptTokens = usage.PromptTokens
			logEntry.CompletionTokens = usage.CompletionTokens
			logEntry.TotalTokens = usage.TotalTokens
			if logEntry.TotalTokens == 0 {
				logEntry.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}
			logEntry.Cost = calculateCost(state.target, usage)
			if err := a.store.InsertRequestLog(context.Background(), logEntry); err != nil {
				a.log.Error("insert request log", "error", err)
			}
		}()

		if result.Err != nil {
			status = result.StatusCode
			if status < 400 || status > 599 {
				if errors.Is(result.Err, context.DeadlineExceeded) {
					status = http.StatusGatewayTimeout
				} else if errors.Is(result.Err, router.ErrNoHealthyRoute) {
					status = http.StatusServiceUnavailable
				} else {
					status = http.StatusBadGateway
				}
			}
			writeGatewayError(w, apiType, status, "upstream_error", result.Err.Error())
			return
		}

		if req.Stream {
			status = a.streamResponse(w, r, apiType, req, state, rule)
			return
		}
		downstream, err := translate.BuildResponse(apiType, state.parsed, req.Model)
		if err != nil {
			status = http.StatusBadGateway
			writeGatewayError(w, apiType, status, "gateway_error", err.Error())
			return
		}
		status = http.StatusOK
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(downstream)
	}
}

func (a *App) attempt(ctx context.Context, req translate.Request, target store.RouteTarget, state *attemptState) router.AttemptOutcome {
	runtimeTarget, err := a.store.GetRuntimeTarget(ctx, target.ID)
	if err != nil {
		return router.AttemptOutcome{Retryable: true, StatusCode: http.StatusBadGateway, Err: fmt.Errorf("load target: %w", err)}
	}
	// Track the attempted provider even if this attempt fails, so the request
	// log can attribute the failure to a provider.
	*state = attemptState{target: runtimeTarget}
	body, err := translate.BuildUpstreamRequest(runtimeTarget.APIType, req, runtimeTarget.UpstreamModel)
	if err != nil {
		return router.AttemptOutcome{Retryable: false, StatusCode: http.StatusBadGateway, Err: err}
	}
	resp, err := a.upstream.Do(ctx, runtimeTarget, body)
	if err != nil {
		if ctx.Err() != nil {
			return router.AttemptOutcome{Retryable: false, StatusCode: http.StatusGatewayTimeout, Err: ctx.Err()}
		}
		return router.AttemptOutcome{Retryable: true, StatusCode: http.StatusBadGateway, Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		resp.Body.Close()
		err := fmt.Errorf("provider %s returned %d: %s", runtimeTarget.ProviderName, resp.StatusCode, strings.TrimSpace(string(raw)))
		return router.AttemptOutcome{Retryable: isRetryableStatus(resp.StatusCode), StatusCode: resp.StatusCode, Err: err}
	}
	*state = attemptState{target: runtimeTarget}
	if !req.Stream {
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		resp.Body.Close()
		if err != nil {
			return router.AttemptOutcome{Retryable: true, StatusCode: http.StatusBadGateway, Err: err}
		}
		parsed, err := translate.ParseResponse(runtimeTarget.APIType, raw)
		if err != nil {
			return router.AttemptOutcome{Retryable: true, StatusCode: http.StatusBadGateway, Err: err}
		}
		state.parsed = parsed
		state.usage = parsed.Usage
		return router.AttemptOutcome{Success: true, StatusCode: http.StatusOK}
	}

	reader := sse.NewReader(resp.Body)
	first, err := reader.Next()
	if err != nil {
		resp.Body.Close()
		return router.AttemptOutcome{Retryable: true, StatusCode: http.StatusBadGateway, Err: fmt.Errorf("read upstream stream: %w", err)}
	}
	if isStreamError(first) {
		resp.Body.Close()
		return router.AttemptOutcome{Retryable: true, StatusCode: http.StatusBadGateway,
			Err: fmt.Errorf("upstream stream error: %s", truncate(string(first.Data), 300))}
	}
	state.reader = reader
	state.firstEvent = &first
	return router.AttemptOutcome{Success: true, StatusCode: http.StatusOK}
}

func (a *App) streamResponse(w http.ResponseWriter, r *http.Request, apiType string, req translate.Request, state *attemptState, rule store.RoutingRule) int {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeGatewayError(w, apiType, http.StatusInternalServerError, "gateway_error", "streaming is not supported by the server")
		return http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	conv := translate.NewStreamConverter(state.target.APIType, apiType, req.Model)
	writer := sse.NewWriter(w)
	status := http.StatusOK

	writeEvents := func(events []sse.Event) {
		for _, ev := range events {
			if err := writer.WriteEvent(ev.Event, ev.Data); err != nil {
				return
			}
		}
		flusher.Flush()
	}

	process := func(ev sse.Event) bool {
		events, err := conv.Feed(ev)
		if err != nil {
			a.log.Warn("stream conversion failed", "error", err, "request_id", r.Header.Get("X-Request-ID"))
			writeStreamError(writer, apiType, err.Error())
			flusher.Flush()
			status = http.StatusBadGateway
			return false
		}
		writeEvents(events)
		return true
	}

	if state.firstEvent != nil {
		if !process(*state.firstEvent) {
			state.usage = conv.Usage()
			return status
		}
	}
	for {
		ev, err := state.reader.Next()
		if err == io.EOF {
			// Let the converter emit a terminating [DONE] for chat streams.
			process(sse.Event{Data: []byte("[DONE]")})
			break
		}
		if err != nil {
			a.log.Warn("read upstream stream", "error", err)
			writeStreamError(writer, apiType, err.Error())
			flusher.Flush()
			status = http.StatusBadGateway
			break
		}
		if !process(ev) {
			break
		}
	}
	state.usage = conv.Usage()
	_ = rule
	return status
}

func writeStreamError(w *sse.Writer, apiType, msg string) {
	switch apiType {
	case translate.APIMessages:
		_ = w.WriteEvent("error", map[string]any{
			"type": "error", "error": map[string]any{"type": "api_error", "message": msg},
		})
	case translate.APIResponses:
		_ = w.WriteEvent("error", map[string]any{"type": "error", "message": msg, "code": "upstream_error"})
	default:
		_ = w.WriteEvent("", map[string]any{
			"error": map[string]any{"message": msg, "type": "upstream_error", "code": "upstream_error"},
		})
	}
}

func isStreamError(ev sse.Event) bool {
	if ev.Event == "error" {
		return true
	}
	data := strings.TrimSpace(string(ev.Data))
	return strings.HasPrefix(data, `{"type":"error"`) || strings.HasPrefix(data, `{"error":`)
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooManyRequests:
		return true
	}
	return status >= 500
}

func calculateCost(target store.RuntimeTarget, usage translate.Usage) float64 {
	return float64(usage.PromptTokens)/1e6*target.InputPrice + float64(usage.CompletionTokens)/1e6*target.OutputPrice
}

func (a *App) authenticateAPIKey(r *http.Request) (store.APIKey, error) {
	key := ""
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		key = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	if key == "" {
		key = strings.TrimSpace(r.Header.Get("x-api-key"))
	}
	if key == "" {
		return store.APIKey{}, errors.New("missing API key")
	}
	return a.store.AuthenticateAPIKey(r.Context(), key)
}

func writeGatewayError(w http.ResponseWriter, apiType string, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if apiType == translate.APIMessages {
		writeJSON(w, map[string]any{
			"type":  "error",
			"error": map[string]any{"type": code, "message": message},
		})
		return
	}
	writeJSON(w, map[string]any{
		"error": map[string]any{"message": message, "type": code, "code": code},
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	_ = jsonEncoder(w).Encode(v)
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := splitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func parsePositiveInt(v string, fallback int) int {
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
