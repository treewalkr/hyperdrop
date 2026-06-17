package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHub_Broadcast_DeliversToSubscriber(t *testing.T) {
	h := newHub()
	s := h.subscribe()

	h.broadcast(map[string]any{
		"type": "file_uploaded",
		"file": map[string]any{"name": "photo.jpg", "size": int64(42), "category": "img"},
	})

	select {
	case raw := <-s.out:
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got["type"] != "file_uploaded" {
			t.Errorf("type: got %v, want file_uploaded", got["type"])
		}
		file, ok := got["file"].(map[string]any)
		if !ok || file["name"] != "photo.jpg" {
			t.Errorf("file payload: got %v", got["file"])
		}
	default:
		t.Fatal("no event received on subscriber channel")
	}
}

func TestHub_Unsubscribe_StopsDelivery(t *testing.T) {
	h := newHub()
	s := h.subscribe()
	h.unsubscribe(s)

	h.broadcast(map[string]any{"type": "file_deleted", "name": "x"})

	if len(s.out) != 0 {
		t.Errorf("after unsubscribe, channel should be empty; got %d", len(s.out))
	}
}

func TestHub_Broadcast_JSONShape_FileDeleted(t *testing.T) {
	h := newHub()
	s := h.subscribe()
	h.broadcast(map[string]any{"type": "file_deleted", "name": "photo.jpg"})

	raw := <-s.out
	if !strings.Contains(string(raw), `"type":"file_deleted"`) {
		t.Errorf("payload missing type marker: %s", raw)
	}
	if !strings.Contains(string(raw), `"name":"photo.jpg"`) {
		t.Errorf("payload missing name: %s", raw)
	}
}
