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
	clients map[string]map[*websocket.Conn]*roomWriter
}

// Every data writer, including hydration and journal catch-up, shares this lock.
type roomWriter struct {
	conn       *websocket.Conn
	mu         sync.Mutex
	frozen     chan struct{}
	freezeOnce sync.Once
}

func (w *roomWriter) freeze() { w.freezeOnce.Do(func() { close(w.frozen) }) }

func (w *roomWriter) write(payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return w.conn.WriteMessage(websocket.BinaryMessage, payload)
}

func NewRoomHub() *RoomHub { return &RoomHub{clients: map[string]map[*websocket.Conn]*roomWriter{}} }

func (h *RoomHub) add(room string, c *websocket.Conn) *roomWriter {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[room] == nil {
		h.clients[room] = map[*websocket.Conn]*roomWriter{}
	}
	writer := &roomWriter{conn: c, frozen: make(chan struct{})}
	h.clients[room][c] = writer
	return writer
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
	cs := make([]*roomWriter, 0, len(h.clients[room]))
	for _, writer := range h.clients[room] {
		cs = append(cs, writer)
	}
	h.mu.RUnlock()
	for _, c := range cs {
		if err := c.write(payload); err != nil {
			h.remove(room, c.conn)
			_ = c.conn.Close()
		}
	}
}

// Close requests final journal replay before sending the frozen close code.
// A close notification can overtake a committed update on another instance.
func (h *RoomHub) Close(room string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, writer := range h.clients[room] {
		writer.freeze()
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
