package handler

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const liveEventsChannel = "live_session_events"

// LiveHub is deliberately a small broadcast-only hub. Redis is the source of
// cross-process events; this type owns only the sockets connected to one API.
type LiveHub struct {
	mu      sync.RWMutex
	clients map[string]map[*websocket.Conn]struct{}
}

func NewLiveHub() *LiveHub { return &LiveHub{clients: map[string]map[*websocket.Conn]struct{}{}} }
func (h *LiveHub) add(session string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[session] == nil {
		h.clients[session] = map[*websocket.Conn]struct{}{}
	}
	h.clients[session][c] = struct{}{}
}
func (h *LiveHub) remove(session string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients[session], c)
	if len(h.clients[session]) == 0 {
		delete(h.clients, session)
	}
}
func (h *LiveHub) Broadcast(session string, payload []byte) {
	h.mu.RLock()
	cs := make([]*websocket.Conn, 0, len(h.clients[session]))
	for c := range h.clients[session] {
		cs = append(cs, c)
	}
	h.mu.RUnlock()
	for _, c := range cs {
		if err := c.WriteMessage(websocket.TextMessage, payload); err != nil {
			h.remove(session, c)
			_ = c.Close()
		}
	}
}
func (h *LiveHub) Run(ctx context.Context, rdb *redis.Client) {
	if rdb == nil {
		return
	}
	sub := rdb.Subscribe(ctx, liveEventsChannel)
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var envelope struct {
				SessionID string `json:"session_id"`
			}
			if json.Unmarshal([]byte(msg.Payload), &envelope) == nil && envelope.SessionID != "" {
				h.Broadcast(envelope.SessionID, []byte(msg.Payload))
			}
		}
	}
}
