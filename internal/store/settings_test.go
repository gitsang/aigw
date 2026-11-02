package store

import (
	"encoding/json"
	"testing"
)

func TestDefaultSettings(t *testing.T) {
	settings := DefaultSettings()
	if settings.AutoRouteMatch {
		t.Fatal("auto route matching must default to off")
	}
}

// The console sends {"auto_route_match": bool}; keep the wire format stable.
func TestSettingsJSONContract(t *testing.T) {
	raw, err := json.Marshal(Settings{AutoRouteMatch: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"auto_route_match":true}` {
		t.Fatalf("unexpected payload: %s", raw)
	}
	var decoded Settings
	if err := json.Unmarshal([]byte(`{"auto_route_match":true}`), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !decoded.AutoRouteMatch {
		t.Fatal("auto_route_match was not decoded")
	}
}
