package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/caze/ascend/api/internal/store"
)

// RoomHub is a pure byte relay for collaborative-room websocket connections,
// one group per room_id. It never inspects the Yjs payload it carries —
// decoding the CRDT protocol is entirely the frontend's job.
type RoomHub struct {
	mu      sync.RWMutex
	clients map[string]map[*websocket.Conn]struct{}
}

func NewRoomHub() *RoomHub { return &RoomHub{clients: map[string]map[*websocket.Conn]struct{}{}} }

func (h *RoomHub) add(room string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[room] == nil {
		h.clients[room] = map[*websocket.Conn]struct{}{}
	}
	h.clients[room][c] = struct{}{}
}

func (h *RoomHub) remove(room string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients[room], c)
	if len(h.clients[room]) == 0 {
		delete(h.clients, room)
	}
}

// Broadcast sends payload to every locally-connected client of room. Yjs
// updates are commutative, so delivering a frame back to its own sender
// (once the origin instance's Run loop echoes its own publish) is harmless.
func (h *RoomHub) Broadcast(room string, payload []byte) {
	h.mu.RLock()
	cs := make([]*websocket.Conn, 0, len(h.clients[room]))
	for c := range h.clients[room] {
		cs = append(cs, c)
	}
	h.mu.RUnlock()
	for _, c := range cs {
		if err := c.WriteMessage(websocket.BinaryMessage, payload); err != nil {
			h.remove(room, c)
			_ = c.Close()
		}
	}
}

// Close force-disconnects every locally-connected client of room. Called
// directly (for immediate local effect) whenever this instance freezes a
// room, and again from Run when another instance's freeze is relayed here.
func (h *RoomHub) Close(room string) {
	h.mu.Lock()
	cs := make([]*websocket.Conn, 0, len(h.clients[room]))
	for c := range h.clients[room] {
		cs = append(cs, c)
	}
	delete(h.clients, room)
	h.mu.Unlock()
	for _, c := range cs {
		_ = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4000, "room frozen"), time.Now().Add(2*time.Second))
		_ = c.Close()
	}
}

func (h *RoomHub) Run(ctx context.Context, rdb *redis.Client) {
	if rdb == nil {
		return
	}
	sub := rdb.Subscribe(ctx, store.LiveRoomEventsChannel)
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
				RoomID string `json:"room_id"`
				Data   string `json:"data"`
				Close  string `json:"close"`
			}
			if json.Unmarshal([]byte(msg.Payload), &envelope) != nil || envelope.RoomID == "" {
				continue
			}
			if envelope.Close != "" {
				h.Close(envelope.RoomID)
				continue
			}
			payload, err := base64.StdEncoding.DecodeString(envelope.Data)
			if err != nil {
				continue
			}
			h.Broadcast(envelope.RoomID, payload)
		}
	}
}
