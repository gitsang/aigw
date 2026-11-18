package health

import (
	"encoding/json"
	"testing"

	"github.com/gitsang/aigw/internal/store"
)

func TestProbeBodyInteractions(t *testing.T) {
	body := probeBody(store.HealthProbeTarget{
		UpstreamModel: "gemini-4-hydrogen",
		APIType:       store.APIInteractions,
	})

	if got := body["model"]; got != "gemini-4-hydrogen" {
		t.Fatalf("model = %v, want gemini-4-hydrogen", got)
	}
	if got := body["input"]; got != "ping" {
		t.Fatalf("input = %v, want ping", got)
	}
	// The Interactions API takes stream as a top-level field, like the others.
	if stream, ok := body["stream"].(bool); !ok || stream {
		t.Fatalf("stream = %v, want false", body["stream"])
	}
	// Generation knobs must live inside generation_config, never at the top
	// level, otherwise the upstream rejects the probe.
	if _, ok := body["max_output_tokens"]; ok {
		t.Fatal("max_output_tokens must not be a top-level field")
	}
	gen, ok := body["generation_config"].(map[string]any)
	if !ok {
		t.Fatalf("generation_config = %#v, want an object", body["generation_config"])
	}
	if got := gen["max_output_tokens"]; got != 1 {
		t.Fatalf("generation_config.max_output_tokens = %v, want 1", got)
	}
	// No chat/completions or messages leftovers.
	if _, ok := body["messages"]; ok {
		t.Fatal("interactions probe body must not contain messages")
	}

	if _, err := json.Marshal(body); err != nil {
		t.Fatalf("probe body is not JSON serializable: %v", err)
	}
}

func TestProbeBodyExistingProtocolsUnchanged(t *testing.T) {
	chat := probeBody(store.HealthProbeTarget{UpstreamModel: "m", APIType: store.APIChatCompletions})
	if chat["max_tokens"] != 1 || chat["messages"] == nil {
		t.Fatalf("chat probe body changed: %#v", chat)
	}
	responses := probeBody(store.HealthProbeTarget{UpstreamModel: "m", APIType: store.APIResponses})
	if responses["max_output_tokens"] != 1 {
		t.Fatalf("responses probe body changed: %#v", responses)
	}
}
