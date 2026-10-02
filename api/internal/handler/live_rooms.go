package handler

import (
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/caze/ascend/api/internal/store"
)

// Binary frame tags for the collaborative-room websocket protocol. The
// server treats every payload as opaque past this leading byte — it never
// parses the Yjs encoding.
const (
	roomFrameFullState byte = 2 // client->server versioned binary + text snapshot cache
	roomFrameUpdate    byte = 3 // client->server Yjs update; server->client uint64 version + update
	roomFrameReady     byte = 5 // journal replay frontier; uint64 big endian
)

func roomVersionFrame(tag byte, version uint64, payload []byte) []byte {
	frame := make([]byte, 9+len(payload))
	frame[0] = tag
	binary.BigEndian.PutUint64(frame[1:9], version)
	copy(frame[9:], payload)
	return frame
}

func (h *LiveSessionsHandler) ListRooms(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	session, _, err := h.store.GetLiveSession(r.Context(), id, c.UserID)
	if errors.Is(err, store.ErrNotFound) || (!session.Joined && session.CreatedBy != c.UserID) {
		writeError(w, 404, "session not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	rooms, err := h.store.ListLiveRoomsForSession(r.Context(), id)
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	if c.RealRole == "teacher" {
		for i := range rooms {
			if rooms[i].RoomID == nil {
				continue
			}
			ps, err := h.store.LiveRoomParticipants(r.Context(), *rooms[i].RoomID)
			if err != nil {
				writeError(w, 500, "internal server error")
				return
			}
			rooms[i].Participants = ps
		}
	}
	writeJSON(w, 200, rooms)
}

// RoomWS is the Yjs relay socket for one collaborative room. A real teacher
// (RealRole=="teacher", ViewAs or not) can edit the shared document but does
// not count toward room presence, mirroring the RealRole guard already used
// for Join/CurrentRound.
func (h *LiveSessionsHandler) RoomWS(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	sessionID := chi.URLParam(r, "id")
	listItemID := chi.URLParam(r, "list_item_id")

	session, _, err := h.store.GetLiveSession(r.Context(), sessionID, c.UserID)
	if errors.Is(err, store.ErrNotFound) || (!session.Joined && session.CreatedBy != c.UserID) {
		writeError(w, 404, "session not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	if err := h.store.ValidateLiveRoomItem(r.Context(), sessionID, listItemID); err != nil {
		writeError(w, 404, "room not found")
		return
	}
	room, err := h.store.GetOrCreateLiveRoom(r.Context(), sessionID, listItemID)
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}

	conn, err := liveUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	if c.RealRole != "teacher" && room.Status == "open" {
		participantID, err := h.store.JoinLiveRoom(context.Background(), room.ID, c.UserID)
		if err != nil {
			return
		}
		defer func() { _ = h.store.LeaveLiveRoom(context.Background(), participantID) }()
	}

	writer := h.roomHub.add(room.ID, conn)
	defer h.roomHub.remove(room.ID, conn)
	conn.SetReadLimit(16 << 20)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	// Register before reading: concurrent broadcasts may be duplicated or arrive
	// before their dependencies, which Yjs handles. Replay supplies every update.
	replay := func(after int64) (int64, error) {
		updates, err := h.store.ReadLiveRoomUpdates(ctx, room.ID, after)
		if err != nil {
			return after, err
		}
		for _, update := range updates {
			if len(update.Payload) > 0 {
				if err := writer.write(roomVersionFrame(roomFrameUpdate, update.Version, update.Payload)); err != nil {
					return after, err
				}
			}
			after = int64(update.Version)
		}
		if after < 0 {
			after = 0
		}
		return after, writer.write(roomVersionFrame(roomFrameReady, uint64(after), nil))
	}
	after, err := replay(-1)
	if err != nil {
		slog.Error("hydrate live room", "room_id", room.ID, "err", err)
		return
	}
	closeIfFrozen := func() (bool, error) {
		current, err := h.store.GetLiveRoom(ctx, room.ID)
		if err != nil {
			return false, err
		}
		if current.Status == "open" {
			return false, nil
		}
		// Includes a final replay before close, even when Redis close was missed.
		_, err = replay(after)
		if err != nil {
			return false, err
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4000, "room frozen"), time.Now().Add(2*time.Second))
		return true, nil
	}
	if frozen, err := closeIfFrozen(); err != nil || frozen {
		return
	}
	// Redis pubsub is a latency optimization, not a durable delivery guarantee.
	// Catch up from the journal even after dropped/reordered Redis events.
	replayDone := make(chan struct{})
	go func() {
		defer close(replayDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-writer.frozen:
				_, err := closeIfFrozen()
				if err != nil {
					slog.Error("final live room replay", "room_id", room.ID, "err", err)
				}
				_ = conn.Close()
				return
			case <-ticker.C:
				next, err := replay(after)
				if err != nil {
					slog.Error("replay live room", "room_id", room.ID, "err", err)
					_ = conn.Close()
					return
				}
				after = next
				if frozen, err := closeIfFrozen(); err != nil || frozen {
					_ = conn.Close()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-replayDone }()

	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.BinaryMessage || len(data) < 1 {
			continue
		}
		switch data[0] {
		case roomFrameUpdate:
			if len(data) < 2 {
				continue
			}
			version, err := h.store.AppendLiveRoomUpdate(ctx, room.ID, data[1:])
			if err != nil {
				if errors.Is(err, store.ErrConflict) {
					writer.freeze()
					<-replayDone
					return
				}
				slog.Error("persist live room update", "room_id", room.ID, "err", err)
				return
			}
			payload := roomVersionFrame(roomFrameUpdate, version, data[1:])
			h.roomHub.Broadcast(room.ID, payload)
			if err := h.store.PublishLiveRoomFrame(ctx, room.ID, payload); err != nil {
				slog.Error("publish live room update", "room_id", room.ID, "err", err)
			}
		case roomFrameFullState:
			// One versioned frame saves binary and text snapshots atomically. The
			// legacy unversioned full/text frames cannot overwrite this cache.
			if len(data) < 13 {
				continue
			}
			version := binary.BigEndian.Uint64(data[1:9])
			docLen := uint64(binary.BigEndian.Uint32(data[9:13]))
			if docLen > uint64(len(data)-13) {
				continue
			}
			if _, err := h.store.SaveLiveRoomSnapshot(ctx, room.ID, version, data[13:13+docLen], string(data[13+docLen:])); err != nil {
				slog.Error("save live room snapshot", "room_id", room.ID, "err", err)
				return
			}
		}
	}
}

func (h *LiveSessionsHandler) MarkRoomDone(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	if c.RealRole == "teacher" {
		writeError(w, 403, "insufficient permissions")
		return
	}
	sessionID := chi.URLParam(r, "id")
	listItemID := chi.URLParam(r, "list_item_id")
	if err := h.store.ValidateLiveRoomItem(r.Context(), sessionID, listItemID); err != nil {
		writeError(w, 404, "room not found")
		return
	}
	room, err := h.store.GetOrCreateLiveRoom(r.Context(), sessionID, listItemID)
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	allDone, err := h.store.MarkLiveRoomDone(r.Context(), room.ID, c.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 409, "not an active participant of this room")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	if allDone {
		h.freezeRoomByID(r.Context(), room.ID, sessionID, "all_done")
	}
	w.WriteHeader(204)
}

func (h *LiveSessionsHandler) ForceEndRoom(w http.ResponseWriter, r *http.Request) {
	c, ok := h.teacherClaims(w, r)
	if !ok {
		return
	}
	sessionID := chi.URLParam(r, "id")
	listItemID := chi.URLParam(r, "list_item_id")
	session, _, err := h.store.GetLiveSession(r.Context(), sessionID, c.UserID)
	if errors.Is(err, store.ErrNotFound) || session.CreatedBy != c.UserID {
		writeError(w, 404, "session not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	if err := h.store.ValidateLiveRoomItem(r.Context(), sessionID, listItemID); err != nil {
		writeError(w, 404, "room not found")
		return
	}
	room, err := h.store.GetOrCreateLiveRoom(r.Context(), sessionID, listItemID)
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	h.freezeRoomByID(r.Context(), room.ID, sessionID, "teacher_forced")
	w.WriteHeader(204)
}

func (h *LiveSessionsHandler) RoomResult(w http.ResponseWriter, r *http.Request) {
	c, ok := h.teacherClaims(w, r)
	if !ok {
		return
	}
	sessionID := chi.URLParam(r, "id")
	listItemID := chi.URLParam(r, "list_item_id")
	session, _, err := h.store.GetLiveSession(r.Context(), sessionID, c.UserID)
	if errors.Is(err, store.ErrNotFound) || session.CreatedBy != c.UserID {
		writeError(w, 404, "session not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	room, err := h.store.GetOrCreateLiveRoom(r.Context(), sessionID, listItemID)
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	sub, err := h.store.LiveRoomResultSubmission(r.Context(), room.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, 200, map[string]any{"room": room, "submission": nil})
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 200, map[string]any{"room": room, "submission": sub})
}

// freezeRoomByID attempts to freeze roomID; a room already frozen by a
// concurrent trigger is a silent no-op, since every trigger races the same
// state transition by design.
func (h *LiveSessionsHandler) freezeRoomByID(ctx context.Context, roomID, sessionID, reason string) {
	frozen, err := h.store.FreezeLiveRoom(ctx, roomID, reason)
	if err != nil {
		if !errors.Is(err, store.ErrConflict) {
			slog.Error("freeze live room", "room_id", roomID, "err", err)
		}
		return
	}
	h.finishFrozenRoom(ctx, frozen, sessionID)
}

// finishFrozenRoom closes the room's sockets and, if the item has a linked
// challenge, runs the judge on the last known plain-text snapshot.
func (h *LiveSessionsHandler) finishFrozenRoom(ctx context.Context, frozen store.LiveSessionRoom, sessionID string) {
	h.roomHub.Close(frozen.ID)
	_ = h.store.PublishLiveRoomClose(ctx, frozen.ID)

	challengeID, language, err := h.store.LiveRoomChallenge(ctx, frozen.ListItemID)
	if err != nil {
		slog.Error("resolve live room challenge", "room_id", frozen.ID, "err", err)
		return
	}
	if challengeID == "" || strings.TrimSpace(frozen.TextSnapshot) == "" {
		return
	}
	roomID := frozen.ID
	if _, err := h.store.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID:   challengeID,
		Language:      language,
		SourceCode:    frozen.TextSnapshot,
		LiveSessionID: &sessionID,
		LiveRoomID:    &roomID,
	}); err != nil {
		slog.Error("create live room submission", "room_id", frozen.ID, "err", err)
	}
}
