package server

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
)

// wsHandler upgrades an authenticated GET /api/ws to a WebSocket and pumps Hub
// events to the client until it disconnects. One subscriber per connection;
// disconnect removes it from the Hub so no goroutine or mailbox leaks.
func wsHandler(h *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// LAN tool: accept connections from any origin (browsers across the
		// network share a token, not a same-origin guarantee).
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		defer c.CloseNow()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		s := h.subscribe()
		defer h.unsubscribe(s)

		// Reader goroutine: detects client disconnect/close by failing to read.
		// The client never sends meaningful messages; we only care about EOF.
		go func() {
			defer cancel()
			for {
				if _, _, err := c.Read(ctx); err != nil {
					return
				}
			}
		}()

		for {
			select {
			case msg := <-s.out:
				if err := c.Write(ctx, websocket.MessageText, msg); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}
