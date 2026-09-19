package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/caze/ascend/api/internal/store"
)

// Binary frame tags for the collaborative-room websocket protocol. The
// server treats every payload as opaque past this leading byte — it never
// parses the Yjs encoding.
const (
	roomFrameFullState byte = 2 // hydration: server->client on connect, client->server debounced persistence
	roomFrameUpdate    byte = 3 // incremental Yjs update, relayed to every other client in the room
	roomFrameText      byte = 4 // debounced plain-text snapshot, stored for judge input / fallback review
)

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
// (RealRole=="teacher", ViewAs or not) connects read-only: it neither counts
// toward presence nor is allowed to write updates, mirroring the RealRole
// guard already used for Join/CurrentRound.
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
	if room.Status != "open" {
		writeError(w, 409, "room is frozen")
		return
	}

	conn, err := liveUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	readOnly := c.RealRole == "teacher"
	if !readOnly {
		participantID, err := h.store.JoinLiveRoom(context.Background(), room.ID, c.UserID)
		if err != nil {
			return
		}
		defer func() { _ = h.store.LeaveLiveRoom(context.Background(), participantID) }()
	}

	h.roomHub.add(room.ID, conn)
	defer h.roomHub.remove(room.ID, conn)

	if doc, err := h.store.GetLiveRoomDocSnapshot(r.Context(), room.ID); err == nil && len(doc) > 0 {
		_ = conn.WriteMessage(websocket.BinaryMessage, append([]byte{roomFrameFullState}, doc...))
	}

	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.BinaryMessage || len(data) < 1 || readOnly {
			continue
		}
		switch data[0] {
		case roomFrameUpdate:
			_ = h.store.PublishLiveRoomFrame(context.Background(), room.ID, data)
		case roomFrameFullState:
			_ = h.store.SaveLiveRoomDocSnapshot(context.Background(), room.ID, data[1:])
		case roomFrameText:
			_ = h.store.SaveLiveRoomTextSnapshot(context.Background(), room.ID, string(data[1:]))
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
