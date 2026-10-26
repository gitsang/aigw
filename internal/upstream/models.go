package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gitsang/aigw/internal/store"
)

// DiscoveredModel is a model advertised by a provider's /models endpoint.
type DiscoveredModel struct {
	ID      string
	Name    string
	OwnedBy string
}

// ListModels calls the provider's OpenAI-compatible /models endpoint.
func (c *Client) ListModels(ctx context.Context, provider store.Provider, apiKey string, headers map[string]string) ([]DiscoveredModel, error) {
	endpoint, err := BuildURL(provider.BaseURL, "/v1/models")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	ApplyAuth(req.Header, store.RuntimeTarget{
		AuthType: provider.AuthType, APIKey: apiKey, APIType: store.APIChatCompletions,
	})
	for k, v := range headers {
		if isSafeHeader(k) {
			req.Header.Set(k, v)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider /models returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode /models response: %w", err)
	}
	out := make([]DiscoveredModel, 0, len(payload.Data))
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = id
		}
		out = append(out, DiscoveredModel{ID: id, Name: name, OwnedBy: m.OwnedBy})
	}
	return out, nil
}
