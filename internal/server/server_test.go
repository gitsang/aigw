package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gitsang/aigw/internal/app"
	"github.com/gitsang/aigw/internal/config"
	"github.com/go-chi/chi/v5"
)

func TestInteractionsInboundRoutesRegistered(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &Server{cfg: config.Config{}, log: log, gateway: app.New(config.Config{}, nil, log)}

	// Collect the registered patterns; handlers are not invoked, so no store
	// access happens.
	registered := map[string]bool{}
	err := chi.Walk(s.Handler().(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}
	for _, route := range []string{
		"POST /v1beta/interactions",
		"POST /v1/interactions",
	} {
		if !registered[route] {
			t.Errorf("route %q is not registered; got %v", route, registered)
		}
	}
	// The pre-existing protocol routes must stay registered.
	for _, route := range []string{"POST /v1/chat/completions", "POST /v1/responses", "POST /v1/messages"} {
		if !registered[route] {
			t.Errorf("route %q disappeared", route)
		}
	}
}

func TestInteractionsRouteEmitsGoogleErrorShape(t *testing.T) {
	// An unauthenticated request exercises the whole inbound wire: route
	// matching, apiTypeForPath, the Interactions error envelope. Authentication
	// fails before the store is touched, so a nil store is safe here.
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &Server{cfg: config.Config{}, log: log, gateway: app.New(config.Config{}, nil, log)}
	handler := s.Handler()

	for _, path := range []string{"/v1beta/interactions", "/v1/interactions"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gemini-4-hydrogen","input":"hi"}`))
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", path, rec.Code)
		}
		var body struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s decode body %q: %v", path, rec.Body.String(), err)
		}
		if body.Error.Code != http.StatusUnauthorized || body.Error.Status != "UNAUTHENTICATED" {
			t.Fatalf("%s error body = %s, want code 401 / UNAUTHENTICATED", path, rec.Body.String())
		}
		if body.Error.Message == "" {
			t.Fatalf("%s error message is empty", path)
		}
	}
}
