package store

import "testing"

func TestChooseAPITypeOrdering(t *testing.T) {
	all := DefaultAPITypes()
	tests := []struct {
		name          string
		targetTypes   []string
		modelTypes    []string
		providerTypes []string
		inbound       string
		want          string
	}{
		{
			name:          "target order wins when inbound unsupported",
			targetTypes:   []string{APIResponses, APIChatCompletions},
			providerTypes: all, inbound: APIMessages, want: APIResponses,
		},
		{
			name:          "inbound match inside target list",
			targetTypes:   []string{APIResponses, APIChatCompletions},
			providerTypes: all, inbound: APIChatCompletions, want: APIChatCompletions,
		},
		{
			name:          "inbound responses matches target",
			targetTypes:   []string{APIResponses, APIChatCompletions},
			providerTypes: all, inbound: APIResponses, want: APIResponses,
		},
		{
			name:        "model support filters target list",
			targetTypes: []string{APIChatCompletions, APIMessages},
			modelTypes:  []string{APIMessages}, inbound: APIChatCompletions, want: APIMessages,
		},
		{
			name:        "empty target inherits model order",
			targetTypes: nil, modelTypes: []string{APIResponses, APIMessages}, inbound: APIChatCompletions, want: APIResponses,
		},
		{
			name:        "empty target inherits provider order",
			targetTypes: nil, providerTypes: []string{APIMessages, APIChatCompletions}, inbound: APIChatCompletions, want: APIChatCompletions,
		},
		{
			name:    "everything empty uses builtins",
			inbound: APIMessages, want: APIMessages,
		},
		{
			name:        "target fully unsupported falls back to available list",
			targetTypes: []string{APIMessages}, modelTypes: []string{APIChatCompletions}, inbound: "", want: APIChatCompletions,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ChooseAPIType(tt.targetTypes, tt.modelTypes, tt.providerTypes, tt.inbound); got != tt.want {
				t.Fatalf("ChooseAPIType()=%q want %q", got, tt.want)
			}
		})
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

func TestNormalizeOrderedAPITypes(t *testing.T) {
	got := NormalizeOrderedAPITypes([]string{"responses", "chat.completions", "responses", "bogus"})
	want := []string{APIResponses, APIChatCompletions}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}
