package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

type LiveSessionsHandler struct {
	store   *store.Store
	hub     *LiveHub
	roomHub *RoomHub
}

func NewLiveSessionsHandler(s *store.Store, hub *LiveHub, roomHub *RoomHub) *LiveSessionsHandler {
	if hub == nil {
		hub = NewLiveHub()
	}
	if roomHub == nil {
		roomHub = NewRoomHub()
	}
	return &LiveSessionsHandler{store: s, hub: hub, roomHub: roomHub}
}
func liveClaims(w http.ResponseWriter, r *http.Request) (auth.Claims, bool) {
	c, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
	}
	return c, ok
}
func (h *LiveSessionsHandler) List(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	xs, err := h.store.ListLiveSessions(r.Context(), c.UserID, c.Role == "teacher")
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 200, xs)
}
func (h *LiveSessionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	x, ps, err := h.store.GetLiveSession(r.Context(), chi.URLParam(r, "id"), c.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "session not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	if !x.Joined && x.CreatedBy != c.UserID {
		x.Items = []store.ListItem{}
		ps = []store.LiveParticipant{}
	}
	writeJSON(w, 200, map[string]any{"session": x, "participants": ps})
}
func (h *LiveSessionsHandler) Create(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	if c.Role != "teacher" {
		writeError(w, 403, "insufficient permissions")
		return
	}
	var b struct {
		ProblemListID   string `json:"problem_list_id"`
		MinParticipants int    `json:"min_participants"`
		Mode            string `json:"mode"`
	}
	if json.NewDecoder(r.Body).Decode(&b) != nil || b.ProblemListID == "" || b.MinParticipants < 1 {
		writeError(w, 422, "problem_list_id and min_participants are required")
		return
	}
	if b.Mode == "" {
		b.Mode = "individual"
	}
	x, err := h.store.CreateLiveSessionWithMode(r.Context(), b.ProblemListID, c.UserID, b.MinParticipants, b.Mode)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "problem list not found")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		writeError(w, 422, "problem list needs at least one linked challenge")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 201, x)
}

func (h *LiveSessionsHandler) teacherClaims(w http.ResponseWriter, r *http.Request) (auth.Claims, bool) {
	c, ok := liveClaims(w, r)
	if !ok {
		return c, false
	}
	if c.RealRole != "teacher" {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return c, false
	}
	return c, true
}
func (h *LiveSessionsHandler) CreateRounds(w http.ResponseWriter, r *http.Request) {
	c, ok := h.teacherClaims(w, r)
	if !ok {
		return
	}
	rounds, err := h.store.CreateLiveSessionRounds(r.Context(), chi.URLParam(r, "id"), c.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "session not found")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		writeError(w, 409, "rounds require a timed session with linked challenges")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 201, rounds)
}
func (h *LiveSessionsHandler) StartRound(w http.ResponseWriter, r *http.Request) {
	c, ok := h.teacherClaims(w, r)
	if !ok {
		return
	}
	var b struct {
		DurationSeconds int `json:"duration_seconds"`
	}
	if json.NewDecoder(r.Body).Decode(&b) != nil || b.DurationSeconds <= 0 {
		writeError(w, 422, "duration_seconds must be positive")
		return
	}
	round, err := h.store.StartLiveSessionRound(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "round_id"), c.UserID, b.DurationSeconds)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "round or session not found")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		writeError(w, 409, "another round is active or duration is invalid")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	_ = h.store.PublishLiveEvent(r.Context(), round.SessionID, "round_started", round)
	go func(round store.LiveSessionRound, teacher string) {
		timer := time.NewTimer(time.Duration(round.DurationSeconds) * time.Second)
		defer timer.Stop()
		<-timer.C
		ended, err := h.store.EndLiveSessionRound(context.Background(), round.SessionID, round.ID, teacher)
		if err == nil {
			_ = h.store.PublishLiveEvent(context.Background(), ended.SessionID, "round_ended", map[string]string{"round_id": ended.ID})
		}
	}(round, c.UserID)
	writeJSON(w, 200, round)
}
func (h *LiveSessionsHandler) EndRound(w http.ResponseWriter, r *http.Request) {
	c, ok := h.teacherClaims(w, r)
	if !ok {
		return
	}
	round, err := h.store.EndLiveSessionRound(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "round_id"), c.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "active round not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	_ = h.store.PublishLiveEvent(r.Context(), round.SessionID, "round_ended", map[string]string{"round_id": round.ID})
	writeJSON(w, 200, round)
}

var liveUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || origin == "http://localhost:5173" || origin == "http://localhost:5174" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}}

func (h *LiveSessionsHandler) Events(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	session, _, err := h.store.GetLiveSession(r.Context(), chi.URLParam(r, "id"), c.UserID)
	if errors.Is(err, store.ErrNotFound) || (!session.Joined && session.CreatedBy != c.UserID) {
		writeError(w, 404, "session not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	conn, err := liveUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.hub.add(session.ID, conn)
	defer func() { h.hub.remove(session.ID, conn); _ = conn.Close() }()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
func (h *LiveSessionsHandler) CurrentRound(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	if c.RealRole == "teacher" {
		writeError(w, 403, "insufficient permissions")
		return
	}
	round, err := h.store.CurrentLiveSessionRound(r.Context(), chi.URLParam(r, "id"), c.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "session not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 200, round)
}
func (h *LiveSessionsHandler) LiveRoundStatus(w http.ResponseWriter, r *http.Request) {
	c, ok := h.teacherClaims(w, r)
	if !ok {
		return
	}
	round, students, err := h.store.LiveRoundStatus(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "round_id"), c.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "round or session not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 200, map[string]any{"round": round, "students": students, "server_time": time.Now().UTC()})
}
func (h *LiveSessionsHandler) Join(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	if c.RealRole == "teacher" {
		writeError(w, 403, "teachers cannot join live sessions")
		return
	}
	x, err := h.store.JoinLiveSession(r.Context(), chi.URLParam(r, "id"), c.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "session not found")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		writeError(w, 409, "session finished")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 200, x)
}
func (h *LiveSessionsHandler) Finish(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	if c.Role != "teacher" {
		writeError(w, 403, "insufficient permissions")
		return
	}
	id := chi.URLParam(r, "id")
	err := h.store.FinishLiveSession(r.Context(), id, c.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "session not found or already finished")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	rooms, err := h.store.FreezeOpenLiveRoomsForSession(r.Context(), id, "time_limit")
	if err != nil {
		slog.Error("freeze live rooms on session finish", "session_id", id, "err", err)
	}
	for _, room := range rooms {
		h.finishFrozenRoom(r.Context(), room, id)
	}
	w.WriteHeader(204)
}
func (h *LiveSessionsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	if c.RealRole != "teacher" {
		writeError(w, 403, "insufficient permissions")
		return
	}
	d, err := h.store.LiveDashboard(r.Context(), chi.URLParam(r, "id"), c.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "session not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	if d.Session.CreatedBy != c.UserID {
		writeError(w, 403, "insufficient permissions")
		return
	}
	writeJSON(w, 200, d)
}
