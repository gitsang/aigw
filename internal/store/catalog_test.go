package store

import (
	"errors"
	"testing"
)

// schema builds an api_schema from a list of enabled protocols; every enabled
// protocol uses the built-in endpoint unless an override is given.
func schema(enabled []string, endpoints map[string]string) APISchema {
	out := make(APISchema, len(DefaultProtocols()))
	for _, name := range DefaultProtocols() {
		on := false
		for _, e := range enabled {
			if e == name {
				on = true
			}
		}
		if !on {
			continue
		}
		out[name] = APISchemaEntry{Enabled: true, Endpoint: endpoints[name]}
	}
	return out
}

func TestResolveAPISchema(t *testing.T) {
	providerOnly := schema([]string{APIMessages}, nil)
	tests := []struct {
		name        string
		model       APISchema
		provider    APISchema
		wantEnabled []string
		wantPath    map[string]string
	}{
		{
			name:        "nothing configured enables all built-ins",
			wantEnabled: DefaultProtocols(),
			wantPath: map[string]string{
				APIChatCompletions: "/v1/chat/completions",
				APIResponses:       "/v1/responses",
				APIMessages:        "/v1/messages",
			},
		},
		{
			name:        "provider schema defines support when the model has none",
			provider:    providerOnly,
			wantEnabled: []string{APIMessages},
			wantPath:    map[string]string{APIMessages: "/v1/messages"},
		},
		{
			name:        "model schema replaces provider support wholesale",
			model:       schema([]string{APIResponses}, nil),
			provider:    DefaultAPISchema(),
			wantEnabled: []string{APIResponses},
			wantPath:    map[string]string{APIResponses: "/v1/responses"},
		},
		{
			name:        "model endpoint wins over provider endpoint",
			model:       schema([]string{APIMessages}, map[string]string{APIMessages: "/model/messages"}),
			provider:    schema([]string{APIMessages}, map[string]string{APIMessages: "/provider/messages"}),
			wantEnabled: []string{APIMessages},
			wantPath:    map[string]string{APIMessages: "/model/messages"},
		},
		{
			name:        "provider endpoint applies when the model sets none",
			model:       schema([]string{APIMessages}, nil),
			provider:    schema([]string{APIMessages}, map[string]string{APIMessages: "/provider/messages"}),
			wantEnabled: []string{APIMessages},
			wantPath:    map[string]string{APIMessages: "/provider/messages"},
		},
		{
			// Support comes from the model alone, but endpoints fall back per
			// protocol regardless of which schema enabled it.
			name:  "provider endpoint applies even when the provider disables the protocol",
			model: schema([]string{APIMessages}, nil),
			provider: APISchema{
				APIMessages: {Enabled: false, Endpoint: "/provider/messages"},
			},
			wantEnabled: []string{APIMessages},
			wantPath:    map[string]string{APIMessages: "/provider/messages"},
		},
		{
			name:        "disabled protocols report no endpoint",
			provider:    providerOnly,
			wantEnabled: []string{APIMessages},
			wantPath:    map[string]string{APIResponses: "", APIChatCompletions: ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAPISchema(tt.model, tt.provider)
			if enabled := got.Protocols(); !equalStrings(enabled, tt.wantEnabled) {
				t.Fatalf("enabled=%v want %v", enabled, tt.wantEnabled)
			}
			for name, want := range tt.wantPath {
				if got[name].Endpoint != want {
					t.Fatalf("endpoint[%s]=%q want %q", name, got[name].Endpoint, want)
				}
			}
			if len(got) != len(DefaultProtocols()) {
				t.Fatalf("resolved schema must always carry every protocol, got %v", got)
			}
		})
	}
}

func TestChooseUpstreamAPIType(t *testing.T) {
	all := DefaultAPISchema()
	tests := []struct {
		name    string
		schema  APISchema
		inbound string
		want    string
	}{
		{
			name:   "inbound protocol is forwarded as-is when supported",
			schema: all, inbound: APIMessages, want: APIMessages,
		},
		{
			name:   "inbound responses is forwarded as-is",
			schema: all, inbound: APIResponses, want: APIResponses,
		},
		{
			name:   "empty inbound takes chat.completions first",
			schema: all, inbound: "", want: APIChatCompletions,
		},
		{
			name:    "upstream without chat.completions falls back to responses",
			schema:  schema([]string{APIMessages, APIResponses}, nil),
			inbound: APIChatCompletions, want: APIResponses,
		},
		{
			name:    "upstream with only messages routes there",
			schema:  schema([]string{APIMessages}, nil),
			inbound: APIResponses, want: APIMessages,
		},
		{
			name:   "empty schema keeps chat.completions",
			schema: APISchema{}, inbound: APIMessages, want: APIChatCompletions,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ChooseUpstreamAPIType(tt.schema, tt.inbound); got != tt.want {
				t.Fatalf("ChooseUpstreamAPIType()=%q want %q", got, tt.want)
			}
		})
	}
}

func TestAPISchemaAPITypesOrder(t *testing.T) {
	// A hand-written or legacy schema is not trusted to be in canonical order.
	raw := APISchema{
		APIMessages:        {Enabled: true},
		APIResponses:       {Enabled: false},
		APIChatCompletions: {Enabled: true},
	}
	if got := raw.Protocols(); !equalStrings(got, []string{APIChatCompletions, APIMessages}) {
		t.Fatalf("APITypes()=%v", got)
	}
}

func TestNormalizeAPISchema(t *testing.T) {
	if got, err := normalizeAPISchema(APISchema{}); err != nil || len(got) != 0 {
		t.Fatalf("empty schema must stay unset: %v %v", got, err)
	}

	got, err := normalizeAPISchema(APISchema{
		APIMessages:  {Enabled: true, Endpoint: "  v1/messages "},
		APIResponses: {Enabled: false, Endpoint: "/v1/responses"},
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got[APIMessages].Endpoint != "/v1/messages" {
		t.Fatalf("endpoint should gain a leading slash, got %q", got[APIMessages].Endpoint)
	}
	if got[APIResponses].Enabled {
		t.Fatalf("disabled entry must stay disabled")
	}

	if _, err := normalizeAPISchema(APISchema{"bogus": {Enabled: true}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown protocol should be rejected, got %v", err)
	}
	if _, err := normalizeAPISchema(APISchema{APIMessages: {Enabled: false}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a schema disabling every protocol should be rejected, got %v", err)
	}
}

func TestUnionAPISchemas(t *testing.T) {
	got := UnionAPISchemas([]APISchema{
		schema([]string{APIChatCompletions}, nil),
		schema([]string{APIMessages}, map[string]string{APIMessages: "/custom/messages"}),
	})
	if enabled := got.Protocols(); !equalStrings(enabled, []string{APIChatCompletions, APIMessages}) {
		t.Fatalf("enabled=%v", enabled)
	}
	if got[APIMessages].Endpoint != "/custom/messages" {
		t.Fatalf("endpoint should come from the enabling target, got %q", got[APIMessages].Endpoint)
	}
}

func TestDefaultEndpoint(t *testing.T) {
	tests := map[string]string{
		APIChatCompletions: "/v1/chat/completions",
		APIResponses:       "/v1/responses",
		APIMessages:        "/v1/messages",
	}
	for name, want := range tests {
		if got := DefaultEndpoint(name); got != want {
			t.Fatalf("DefaultEndpoint(%q)=%q want %q", name, got, want)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNormalizeAuthTypeKeepsGoogleScheme(t *testing.T) {
	// The provider form offers x-goog-api-key for Gemini. Normalization must not
	// rewrite it to bearer, otherwise upstream calls use the wrong auth header.
	for _, tt := range []struct{ in, want string }{
		{"x-goog-api-key", "x-goog-api-key"},
		{"x-api-key", "x-api-key"},
		{"none", "none"},
		{"bearer", "bearer"},
		{"", "bearer"},
		{"something-else", "bearer"},
	} {
		if got := normalizeAuthType(tt.in); got != tt.want {
			t.Fatalf("normalizeAuthType(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestDefaultProtocolsIncludeInteractions(t *testing.T) {
	got := DefaultProtocols()
	if !equalStrings(got, []string{APIChatCompletions, APIResponses, APIMessages, APIInteractions}) {
		t.Fatalf("DefaultProtocols()=%v", got)
	}
	if ep := DefaultEndpoint(APIInteractions); ep != "/v1beta/interactions" {
		t.Fatalf("DefaultEndpoint(interactions)=%q", ep)
	}
	if got := DefaultAPISchema(); !got[APIInteractions].Enabled {
		t.Fatalf("default schema must enable interactions: %v", got)
	}
	// interactions must be the last fallback, never displacing the others.
	if protocols := DefaultAPISchema().Protocols(); protocols[len(protocols)-1] != APIInteractions {
		t.Fatalf("interactions must be the final fallback: %v", protocols)
	}
}
