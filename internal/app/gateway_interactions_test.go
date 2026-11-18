package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gitsang/aigw/internal/translate"
	"github.com/gitsang/aigw/pkg/sse"
)

func TestAPITypeForPathInteractions(t *testing.T) {
	tests := []struct{ path, want string }{
		{"/v1beta/interactions", translate.APIInteractions},
		{"/v1/interactions", translate.APIInteractions},
		// The existing mappings must not change.
		{"/v1/responses", translate.APIResponses},
		{"/v1/messages", translate.APIMessages},
		{"/messages", translate.APIMessages},
		{"/v1/chat/completions", translate.APIChat},
		{"/anything/else", translate.APIChat},
	}
	for _, tt := range tests {
		if got := apiTypeForPath(tt.path); got != tt.want {
			t.Errorf("apiTypeForPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestWriteGatewayErrorInteractionsShape(t *testing.T) {
	tests := []struct {
		status     int
		wantStatus string
	}{
		{http.StatusBadRequest, "INVALID_ARGUMENT"},
		{http.StatusUnauthorized, "UNAUTHENTICATED"},
		{http.StatusForbidden, "PERMISSION_DENIED"},
		{http.StatusNotFound, "NOT_FOUND"},
		{http.StatusConflict, "ABORTED"},
		{http.StatusTooManyRequests, "RESOURCE_EXHAUSTED"},
		{http.StatusInternalServerError, "INTERNAL"},
		{http.StatusServiceUnavailable, "UNAVAILABLE"},
		{http.StatusGatewayTimeout, "DEADLINE_EXCEEDED"},
		{http.StatusTeapot, "UNKNOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.wantStatus, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeGatewayError(rec, translate.APIInteractions, tt.status, "invalid_request_error", "boom")

			if rec.Code != tt.status {
				t.Fatalf("status code = %d, want %d", rec.Code, tt.status)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("content type = %q", ct)
			}

			var got struct {
				Error struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
					Status  string `json:"status"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
			}
			if got.Error.Code != tt.status {
				t.Errorf("error.code = %d, want %d", got.Error.Code, tt.status)
			}
			if got.Error.Status != tt.wantStatus {
				t.Errorf("error.status = %q, want %q", got.Error.Status, tt.wantStatus)
			}
			if got.Error.Message != "boom" {
				t.Errorf("error.message = %q, want boom", got.Error.Message)
			}
		})
	}
}

func TestWriteGatewayErrorExistingProtocolsUnchanged(t *testing.T) {
	// OpenAI-compatible protocols keep the {"error":{message,type,code}} shape.
	rec := httptest.NewRecorder()
	writeGatewayError(rec, translate.APIChat, http.StatusBadRequest, "invalid_request_error", "bad")
	var openai struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &openai); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if openai.Error["type"] != "invalid_request_error" || openai.Error["code"] != "invalid_request_error" {
		t.Fatalf("openai error body changed: %s", rec.Body.String())
	}
	if _, ok := openai.Error["status"]; ok {
		t.Fatalf("openai error body must not carry a Google status: %s", rec.Body.String())
	}

	// Anthropic keeps its {"type":"error","error":{type,message}} envelope.
	rec = httptest.NewRecorder()
	writeGatewayError(rec, translate.APIMessages, http.StatusBadRequest, "invalid_request_error", "bad")
	var anthropic struct {
		Type  string         `json:"type"`
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &anthropic); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if anthropic.Type != "error" || anthropic.Error["message"] != "bad" {
		t.Fatalf("anthropic error body changed: %s", rec.Body.String())
	}
}

func TestWriteStreamErrorInteractions(t *testing.T) {
	var buf bytes.Buffer
	writeStreamError(sse.NewWriter(&buf), translate.APIInteractions, "upstream exploded")

	out := buf.String()
	if !strings.HasPrefix(out, "event: error\n") {
		t.Fatalf("expected an SSE error event, got %q", out)
	}
	data := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "data: ") {
			data += strings.TrimPrefix(line, "data: ")
		}
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	if got["event_type"] != "error" {
		t.Errorf("event_type = %v, want error", got["event_type"])
	}
	errBody, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error envelope: %v", got)
	}
	if errBody["message"] != "upstream exploded" {
		t.Errorf("error.message = %v", errBody["message"])
	}
	if _, ok := errBody["status"].(string); !ok {
		t.Errorf("error.status missing: %v", errBody)
	}
	// The SSE stream is already open with HTTP 200, so a stream failure is an
	// internal upstream failure.
	if errBody["code"] != float64(http.StatusInternalServerError) {
		t.Errorf("error.code = %v, want 500", errBody["code"])
	}
}

func TestWriteStreamErrorExistingProtocolsUnchanged(t *testing.T) {
	var buf bytes.Buffer
	writeStreamError(sse.NewWriter(&buf), translate.APIChat, "boom")
	if got := buf.String(); !strings.Contains(got, `"type":"upstream_error"`) {
		t.Fatalf("chat stream error changed: %q", got)
	}

	buf.Reset()
	writeStreamError(sse.NewWriter(&buf), translate.APIMessages, "boom")
	if got := buf.String(); !strings.Contains(got, `"type":"api_error"`) {
		t.Fatalf("messages stream error changed: %q", got)
	}
}

func TestInboundAPIKeySources(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			name:    "google header",
			headers: map[string]string{"x-goog-api-key": "google-key"},
			want:    "google-key",
		},
		{
			name:    "bearer wins over the others",
			headers: map[string]string{"Authorization": "Bearer bearer-key", "x-api-key": "a", "x-goog-api-key": "g"},
			want:    "bearer-key",
		},
		{
			name:    "x-api-key still works",
			headers: map[string]string{"x-api-key": "anthropic-key"},
			want:    "anthropic-key",
		},
		{
			name:    "x-api-key wins over google",
			headers: map[string]string{"x-api-key": "anthropic-key", "x-goog-api-key": "google-key"},
			want:    "anthropic-key",
		},
		{
			name:    "no header",
			headers: map[string]string{},
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/v1beta/interactions", http.NoBody)
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}
			if got := inboundAPIKey(r); got != tt.want {
				t.Fatalf("inboundAPIKey = %q, want %q", got, tt.want)
			}
		})
	}
}
