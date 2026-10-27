package store

import "testing"

func TestChooseAPITypePrecedence(t *testing.T) {
	tests := []struct {
		name          string
		modelTypes    []string
		providerTypes []string
		inbound       string
		want          string
	}{
		{"model wins over provider", []string{APIMessages}, []string{APIChatCompletions}, APIChatCompletions, APIMessages},
		{"model keeps inbound when supported", []string{APIChatCompletions, APIMessages}, nil, APIMessages, APIMessages},
		{"provider used when model unset", nil, []string{APIResponses, APIChatCompletions}, APIChatCompletions, APIChatCompletions},
		{"provider first when inbound unsupported", nil, []string{APIResponses}, APIChatCompletions, APIResponses},
		{"builtin defaults when both unset", nil, nil, APIMessages, APIMessages},
		{"builtin first when inbound unknown", nil, nil, "", APIChatCompletions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ChooseAPIType(tt.modelTypes, tt.providerTypes, tt.inbound); got != tt.want {
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
	if got := ResolvePath(nil, provider, APIChatCompletions); got != "/v1/chat/completions" {
		t.Fatalf("unrelated override leaked = %q", got)
	}
}

func TestNormalizeModelInput(t *testing.T) {
	in, err := normalizeModelInput(ProviderModelInput{
		ModelID: " deepseek/deepseek-flash ", APITypes: []string{"messages", "messages", "bogus"},
		PathOverrides: map[string]string{"messages": "custom/messages"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if in.ModelID != "deepseek/deepseek-flash" || in.DisplayName != in.ModelID {
		t.Fatalf("bad normalize: %+v", in)
	}
	if len(in.APITypes) != 2 || in.APITypes[0] != APIMessages || in.APITypes[1] != APIChatCompletions {
		t.Fatalf("bad api types: %+v", in.APITypes)
	}
	if in.PathOverrides[APIMessages] != "/custom/messages" {
		t.Fatalf("bad path override: %+v", in.PathOverrides)
	}
	if _, err := normalizeModelInput(ProviderModelInput{ModelID: ""}); err == nil {
		t.Fatal("expected empty model_id error")
	}
}

func TestNormalizeProviderAPITypesKeepsEmpty(t *testing.T) {
	if got := normalizeAPITypesOrEmpty(nil); len(got) != 0 {
		t.Fatalf("empty provider types should stay unset, got %v", got)
	}
	got := normalizeAPITypesOrEmpty([]string{"messages", "messages", "bogus"})
	if len(got) != 2 || got[0] != APIMessages || got[1] != APIChatCompletions {
		t.Fatalf("bad normalization: %v", got)
	}
}
