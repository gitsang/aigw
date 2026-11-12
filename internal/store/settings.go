package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Settings is the gateway-wide runtime configuration edited in the console.
type Settings struct {
	// AutoModelMatch dispatches a request to the first provider model whose
	// public "{providerId}/{modelId}" id contains the requested model name when
	// no virtual model matches it.
	AutoModelMatch bool `json:"auto_model_match"`
}

// UnmarshalJSON also accepts the pre-rename key "auto_route_match" so a settings
// row written by an older build keeps its value instead of silently resetting.
func (s *Settings) UnmarshalJSON(data []byte) error {
	var doc struct {
		AutoModelMatch *bool `json:"auto_model_match"`
		LegacyRoute    *bool `json:"auto_route_match"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	s.AutoModelMatch = false
	if doc.AutoModelMatch != nil {
		s.AutoModelMatch = *doc.AutoModelMatch
	} else if doc.LegacyRoute != nil {
		s.AutoModelMatch = *doc.LegacyRoute
	}
	return nil
}

// DefaultSettings is used when the database has no row yet.
func DefaultSettings() Settings {
	return Settings{AutoModelMatch: false}
}

// GetSettings loads the gateway settings, falling back to the defaults when the
// row has not been written yet.
func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	out := DefaultSettings()
	var raw []byte
	var updatedAt time.Time
	err := s.pool.QueryRow(ctx, `
SELECT value, updated_at FROM settings WHERE key = 'gateway'`).Scan(&raw, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, nil
		}
		return out, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			return DefaultSettings(), err
		}
	}
	return out, nil
}

// UpdateSettings persists the gateway settings.
func (s *Store) UpdateSettings(ctx context.Context, in Settings) (Settings, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return Settings{}, err
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO settings (key, value) VALUES ('gateway', $1)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, raw)
	if err != nil {
		return Settings{}, err
	}
	return s.GetSettings(ctx)
}

// AutoModelMatchEnabled is a hot-path helper used by the gateway when a request
// model has no virtual model.
func (s *Store) AutoModelMatchEnabled(ctx context.Context) bool {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return false
	}
	return settings.AutoModelMatch
}
