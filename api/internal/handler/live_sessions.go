package handler

import (
	"encoding/json"
	"errors"
	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type LiveSessionsHandler struct{ store *store.Store }

func NewLiveSessionsHandler(s *store.Store) *LiveSessionsHandler {
	return &LiveSessionsHandler{store: s}
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
	}
	if json.NewDecoder(r.Body).Decode(&b) != nil || b.ProblemListID == "" || b.MinParticipants < 1 {
		writeError(w, 422, "problem_list_id and min_participants are required")
		return
	}
	x, err := h.store.CreateLiveSession(r.Context(), b.ProblemListID, c.UserID, b.MinParticipants)
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
func (h *LiveSessionsHandler) Join(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
		return
	}
	if c.Role == "teacher" {
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
	err := h.store.FinishLiveSession(r.Context(), chi.URLParam(r, "id"), c.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "session not found or already finished")
		return
	}
	if err != nil {
		writeError(w, 500, "internal server error")
		return
	}
	w.WriteHeader(204)
}
func (h *LiveSessionsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	c, ok := liveClaims(w, r)
	if !ok {
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
	if c.Role != "teacher" || d.Session.CreatedBy != c.UserID {
		writeError(w, 403, "insufficient permissions")
		return
	}
	writeJSON(w, 200, d)
}
