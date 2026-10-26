// Package admin implements the management REST API used by the SvelteKit UI.
package admin

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gitsang/aigw/internal/store"
	"github.com/gitsang/aigw/internal/upstream"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	store *store.Store
	log   *slog.Logger
	token string
}

func New(st *store.Store, token string, log *slog.Logger) *Handler {
	return &Handler{store: st, token: token, log: log}
}

func (h *Handler) Routes(r chi.Router) {
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(h.auth)
		r.Get("/session", h.session)
		r.Get("/stats", h.stats)
		r.Get("/logs", h.logs)
		r.Get("/api-types", h.listAPITypes)
		r.Put("/api-types", h.updateAPITypes)
		r.Post("/health/check", h.healthCheck)
		r.Route("/providers", func(r chi.Router) {
			r.Get("/", h.listProviders)
			r.Post("/", h.createProvider)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.getProvider)
				r.Put("/", h.updateProvider)
				r.Delete("/", h.deleteProvider)
				r.Get("/models", h.listProviderModels)
				r.Post("/models", h.createProviderModel)
				r.Post("/models/discover", h.discoverProviderModels)
				r.Put("/models/{modelID}", h.updateProviderModel)
				r.Delete("/models/{modelID}", h.deleteProviderModel)
			})
		})
		r.Route("/routes", func(r chi.Router) {
			r.Get("/", h.listRoutes)
			r.Post("/", h.createRoute)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.getRoute)
				r.Put("/", h.updateRoute)
				r.Delete("/", h.deleteRoute)
			})
		})
		r.Route("/keys", func(r chi.Router) {
			r.Get("/", h.listKeys)
			r.Post("/", h.createKey)
			r.Route("/{id}", func(r chi.Router) {
				r.Put("/", h.updateKey)
				r.Delete("/", h.deleteKey)
			})
		})
	})
}

func (h *Handler) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.token == "" {
			writeError(w, http.StatusUnauthorized, "admin token is not configured")
			return
		}
		provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if provided == "" {
			provided = strings.TrimSpace(r.Header.Get("X-Admin-Token"))
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(h.token)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid admin token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) session(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true})
}

func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListProviders(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) getProvider(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetProvider(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) createProvider(w http.ResponseWriter, r *http.Request) {
	var in store.ProviderInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.BaseURL) == "" {
		writeError(w, http.StatusBadRequest, "name and base_url are required")
		return
	}
	item, err := h.store.CreateProvider(r.Context(), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) updateProvider(w http.ResponseWriter, r *http.Request) {
	var in store.ProviderInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.BaseURL) == "" {
		writeError(w, http.StatusBadRequest, "name and base_url are required")
		return
	}
	item, err := h.store.UpdateProvider(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) deleteProvider(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteProvider(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listRoutes(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListRoutingRules(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) getRoute(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetRoutingRule(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) createRoute(w http.ResponseWriter, r *http.Request) {
	var in store.RoutingRuleInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.VirtualModel) == "" {
		writeError(w, http.StatusBadRequest, "name and virtual_model are required")
		return
	}
	if len(in.Targets) == 0 {
		writeError(w, http.StatusBadRequest, "at least one target is required")
		return
	}
	item, err := h.store.CreateRoutingRule(r.Context(), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) updateRoute(w http.ResponseWriter, r *http.Request) {
	var in store.RoutingRuleInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.VirtualModel) == "" {
		writeError(w, http.StatusBadRequest, "name and virtual_model are required")
		return
	}
	if len(in.Targets) == 0 {
		writeError(w, http.StatusBadRequest, "at least one target is required")
		return
	}
	item, err := h.store.UpdateRoutingRule(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) deleteRoute(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteRoutingRule(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listKeys(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListAPIKeys(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) createKey(w http.ResponseWriter, r *http.Request) {
	var in store.APIKeyInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	item, key, err := h.store.CreateAPIKey(r.Context(), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"api_key": item, "key": key})
}

func (h *Handler) updateKey(w http.ResponseWriter, r *http.Request) {
	var in store.APIKeyInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	item, err := h.store.UpdateAPIKey(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) deleteKey(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteAPIKey(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) logs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.LogFilter{
		Limit:    atoi(q.Get("limit"), 50),
		Offset:   atoi(q.Get("offset"), 0),
		APIType:  q.Get("api_type"),
		Model:    q.Get("model"),
		App:      q.Get("app"),
		KeyName:  q.Get("api_key_name"),
		Provider: q.Get("provider"),
		Status:   atoi(q.Get("status"), 0),
		Search:   q.Get("search"),
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.From = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.To = &t
		}
	}
	items, total, err := h.store.ListLogs(r.Context(), filter)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "limit": filter.Limit, "offset": filter.Offset,
	})
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-24 * time.Hour)
	switch r.URL.Query().Get("range") {
	case "1h":
		since = time.Now().Add(-time.Hour)
	case "7d":
		since = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		since = time.Now().Add(-30 * 24 * time.Hour)
	}
	stats, err := h.store.GetStats(r.Context(), since)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	// The background prober runs every minute. This endpoint only reports that.
	writeJSON(w, http.StatusAccepted, map[string]any{"message": "health checks run in the background every minute"})
}

func (h *Handler) listAPITypes(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListAPITypes(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) updateAPITypes(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Types []store.APITypeInput `json:"types"`
	}
	if !decode(w, r, &in) {
		return
	}
	items, err := h.store.UpdateAPITypes(r.Context(), in.Types)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) listProviderModels(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListProviderModels(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) createProviderModel(w http.ResponseWriter, r *http.Request) {
	var in store.ProviderModelInput
	if !decode(w, r, &in) {
		return
	}
	item, err := h.store.CreateProviderModel(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) updateProviderModel(w http.ResponseWriter, r *http.Request) {
	var in store.ProviderModelInput
	if !decode(w, r, &in) {
		return
	}
	item, err := h.store.UpdateProviderModel(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "modelID"), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) deleteProviderModel(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteProviderModel(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "modelID")); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) discoverProviderModels(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "id")
	provider, apiKey, err := h.store.ProviderCredentials(r.Context(), providerID)
	if err != nil {
		h.fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	discovered, err := upstream.NewClient().ListModels(ctx, provider, apiKey, provider.Headers)
	if err != nil {
		h.fail(w, err)
		return
	}
	models := make([]store.DiscoveredModel, 0, len(discovered))
	for _, m := range discovered {
		models = append(models, store.DiscoveredModel{ID: m.ID, DisplayName: m.Name})
	}
	items, err := h.store.UpsertDiscoveredModels(r.Context(), providerID, models)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"discovered": len(models), "models": items})
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	h.log.Error("admin request failed", "error", err)
	writeError(w, http.StatusInternalServerError, err.Error())
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func atoi(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
