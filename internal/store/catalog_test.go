package store

import "testing"

func TestChooseUpstreamAPIType(t *testing.T) {
	tests := []struct {
		name          string
		modelTypes    []string
		providerTypes []string
		inbound       string
		want          string
	}{
		{
			name:          "inbound protocol is forwarded as-is when supported",
			providerTypes: DefaultAPITypes(), inbound: APIMessages, want: APIMessages,
		},
		{
			name:          "inbound responses is forwarded as-is",
			providerTypes: DefaultAPITypes(), inbound: APIResponses, want: APIResponses,
		},
		{
			name:          "unsupported inbound falls back to chat.completions first",
			providerTypes: DefaultAPITypes(), inbound: "", want: APIChatCompletions,
		},
		{
			name:          "provider without chat.completions falls back to responses",
			providerTypes: []string{APIMessages, APIResponses}, inbound: APIChatCompletions, want: APIResponses,
		},
		{
			name:          "provider with only messages routes there",
			providerTypes: []string{APIMessages}, inbound: APIResponses, want: APIMessages,
		},
		{
			name:       "model protocols win over provider protocols",
			modelTypes: []string{APIMessages}, providerTypes: DefaultAPITypes(), inbound: APIChatCompletions, want: APIMessages,
		},
		{
			name:          "unsorted provider list is normalized to canonical order",
			providerTypes: []string{APIMessages, APIChatCompletions}, inbound: "", want: APIChatCompletions,
		},
		{
			name: "everything empty uses builtins", inbound: APIResponses, want: APIResponses,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ChooseUpstreamAPIType(tt.modelTypes, tt.providerTypes, tt.inbound); got != tt.want {
				t.Fatalf("ChooseUpstreamAPIType()=%q want %q", got, tt.want)
			}
		})
	}
}

func TestCanonicalAPITypes(t *testing.T) {
	got := CanonicalAPITypes([]string{APIMessages, "bogus", APIChatCompletions, APIMessages})
	want := []string{APIChatCompletions, APIMessages}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestResolvePathPrecedence(t *testing.T) {
	provider := map[string]string{APIMessages: "/provider/messages"}
	model := map[string]string{APIMessages: "/model/messages"}
	if got := ResolvePath(model, provider, APIMessages); got != "/model/messages" {
		t.Fatalf("model override = %q", got)
	}
	if got := ResolvePath(nil, provider, APIMessages); got != "/provider/messages" {
		t.Fatalf("provider override = %q", got)
	}
	if got := ResolvePath(nil, nil, APIMessages); got != "/v1/messages" {
		t.Fatalf("builtin default = %q", got)
	}
}
