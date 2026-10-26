// Package server wires HTTP routes, middleware, management API and frontend serving.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitsang/aigw/internal/admin"
	"github.com/gitsang/aigw/internal/app"
	"github.com/gitsang/aigw/internal/config"
	"github.com/gitsang/aigw/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	cfg     config.Config
	log     *slog.Logger
	gateway *app.App
	admin   *admin.Handler
	store   *store.Store
}

func New(cfg config.Config, st *store.Store, log *slog.Logger) *Server {
	return &Server{
		cfg: cfg, log: log, store: st,
		gateway: app.New(cfg, st, log),
		admin:   admin.New(st, cfg.Server.AdminToken, log),
	}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(s.requestLogger)
	r.Use(s.cors)
	r.Use(middleware.Heartbeat("/healthz"))

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := s.store.Ping(ctx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.HandleFunc("/v1/chat/completions", s.gateway.Gateway("chat.completions"))
	r.HandleFunc("/v1/responses", s.gateway.Gateway("responses"))
	r.HandleFunc("/v1/messages", s.gateway.Gateway("messages"))
	r.HandleFunc("/messages", s.gateway.Gateway("messages"))

	s.admin.Routes(r)
	r.NotFound(s.serveFrontend)
	return r
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.cfg.Server.Address,
		Handler:      s.Handler(),
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
		IdleTimeout:  s.cfg.Server.IdleTimeout,
	}
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("http server listening", "address", s.cfg.Server.Address)
		errCh <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		if strings.HasPrefix(r.URL.Path, "/api/admin") || strings.HasPrefix(r.URL.Path, "/v1/") {
			s.log.Info("http request", "method", r.Method, "path", r.URL.Path,
				"status", ww.Status(), "duration", time.Since(start).String())
		}
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, origin := range s.cfg.Server.CORSOrigins {
		allowed[origin] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (allowed[origin] || allowed["*"]) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Admin-Token, X-Api-Key, Anthropic-Version, Anthropic-Beta")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	root := s.cfg.Web.Dir
	clean := path.Clean("/" + r.URL.Path)
	file := filepath.Join(root, filepath.FromSlash(clean))
	if !strings.HasPrefix(file, filepath.Clean(root)+string(os.PathSeparator)) && file != filepath.Clean(root) {
		http.NotFound(w, r)
		return
	}
	if info, err := os.Stat(file); err == nil && !info.IsDir() {
		http.ServeFile(w, r, file)
		return
	}
	index := filepath.Join(root, "index.html")
	if _, err := os.Stat(index); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, "aigw API is running. Frontend build not found; run `make web` or use the Vite dev server.")
		return
	}
	http.ServeFile(w, r, index)
}
