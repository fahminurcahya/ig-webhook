package types

import (
	"encoding/json"
	"testing"
)

func TestNodeTypeUnmarshal(t *testing.T) {
	data := []byte(`{"id":"1","type":"TRIGGER","data":{}}`)
	var n Node
	if err := json.Unmarshal(data, &n); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if n.Type != "TRIGGER" {
		t.Fatalf("expected type TRIGGER, got %q", n.Type)
	}
}
