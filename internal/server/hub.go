package server

import (
	"encoding/json"
	"sync"
)

// subscriberBufferSize bounds per-client outbound buffering. Broadcast drops a
// event for a slow client rather than block the sender (upload/delete handlers).
const subscriberBufferSize = 16

// subscriber is one connected WebSocket client's outbound mailbox. The Hub
// pushes serialized events here; a per-connection writer goroutine drains it.
type subscriber struct {
	out chan []byte
}

// Hub is a goroutine-safe fan-out of file-system events to all connected
// WebSocket clients. It owns no goroutines itself; broadcast is synchronous and
// non-blocking per subscriber.
//
// shares holds the in-memory share-link store. It lives on the Hub so the
// handlers that already take a *Hub can reach it without a new parameter, and
// because — like the subscriber set — it is process-lifetime server state.
type Hub struct {
	mu   sync.Mutex
	subs map[*subscriber]struct{}

	shares *ShareManager
}

func newHub() *Hub {
	return &Hub{subs: make(map[*subscriber]struct{}), shares: newShareManager()}
}

// subscribe registers a new client and returns its outbound channel.
func (h *Hub) subscribe() *subscriber {
	s := &subscriber{out: make(chan []byte, subscriberBufferSize)}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

// unsubscribe removes a client; safe to call more than once.
func (h *Hub) unsubscribe(s *subscriber) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
}

// subscribers returns the current subscriber count (test/observability).
func (h *Hub) subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// broadcast JSON-encodes the payload and delivers it to every subscriber. A
// full client mailbox is skipped (the client will resync on reconnect) rather
// than blocking the HTTP handler that triggered the event.
func (h *Hub) broadcast(v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		select {
		case s.out <- raw:
		default:
		}
	}
}
