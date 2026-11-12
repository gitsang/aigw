package store

import (
	"encoding/json"
	"testing"
)

func TestDefaultSettings(t *testing.T) {
	settings := DefaultSettings()
	if settings.AutoModelMatch {
		t.Fatal("auto model matching must default to off")
	}
}

// The console sends {"auto_model_match": bool}; keep the wire format stable.
func TestSettingsJSONContract(t *testing.T) {
	raw, err := json.Marshal(Settings{AutoModelMatch: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"auto_model_match":true}` {
		t.Fatalf("unexpected payload: %s", raw)
	}
	var decoded Settings
	if err := json.Unmarshal([]byte(`{"auto_model_match":true}`), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !decoded.AutoModelMatch {
		t.Fatal("auto_model_match was not decoded")
	}
}

// A settings row written before the rename still carries "auto_route_match".
func TestSettingsReadsLegacyKey(t *testing.T) {
	var decoded Settings
	if err := json.Unmarshal([]byte(`{"auto_route_match":true}`), &decoded); err != nil {
		t.Fatalf("unmarshal legacy payload: %v", err)
	}
	if !decoded.AutoModelMatch {
		t.Fatal("legacy auto_route_match was not honoured")
	}
	raw, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"auto_model_match":true}` {
		t.Fatalf("expected the new key on write, got %s", raw)
	}
}
