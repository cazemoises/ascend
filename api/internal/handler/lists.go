package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

type ListsHandler struct {
	store *store.Store
}

func NewListsHandler(s *store.Store) *ListsHandler {
	return &ListsHandler{store: s}
}

var validListDifficulties = map[string]bool{
	"easy":   true,
	"medium": true,
	"hard":   true,
}

type listBody struct {
	Title       string  `json:"title"`
	WeekLabel   *string `json:"week_label"`
	WeekStart   *string `json:"week_start"`
	WeekEnd     *string `json:"week_end"`
	Description *string `json:"description"`
	Published   bool    `json:"published"`
}

const dateOnlyLayout = "2006-01-02"

// parseOptionalDate parses an optional "YYYY-MM-DD" field. A nil or empty
// pointer is a valid "no date set" and returns (nil, true); a non-empty
// string that fails to parse returns (nil, false) so the caller can 422.
func parseOptionalDate(s *string) (*time.Time, bool) {
	if s == nil || *s == "" {
		return nil, true
	}
	t, err := time.Parse(dateOnlyLayout, *s)
	if err != nil {
		return nil, false
	}
	return &t, true
}

// isCurrentWeek reports whether now's calendar date falls within [start,
// end] inclusive. DATE columns round-trip through lib/pq as UTC midnight,
// so now is normalized the same way — comparing full timestamps would flip
// "today" out of range depending on time of day.
func isCurrentWeek(start, end *time.Time, now time.Time) bool {
	if start == nil || end == nil {
		return false
	}
	today := now.UTC()
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	return !todayDate.Before(start.UTC()) && !todayDate.After(end.UTC())
}

// problemListResponse adds the is_current flag to a list payload — derived
// from week_start/week_end and the current time, never persisted.
type problemListResponse struct {
	store.ProblemList
	IsCurrent bool `json:"is_current"`
}

// Create handles POST /api/v1/lists (teacher only). New lists always start
// as drafts — published is never client-settable here, only via Update.
func (h *ListsHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body listBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Title == "" {
		writeError(w, http.StatusUnprocessableEntity, "title is required")
		return
	}
	weekStart, ok := parseOptionalDate(body.WeekStart)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "week_start must be YYYY-MM-DD")
		return
	}
	weekEnd, ok := parseOptionalDate(body.WeekEnd)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "week_end must be YYYY-MM-DD")
		return
	}

	list, err := h.store.CreateProblemList(r.Context(), store.CreateProblemListRequest{
		TeacherID:   claims.UserID,
		Title:       body.Title,
		WeekLabel:   body.WeekLabel,
		WeekStart:   weekStart,
		WeekEnd:     weekEnd,
		Description: body.Description,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, list)
}

// List handles GET /api/v1/lists — unauthenticated, always public: every
// requester sees the same thing, published lists platform-wide.
func (h *ListsHandler) List(w http.ResponseWriter, r *http.Request) {
	lists, err := h.store.ListProblemListsForViewer(r.Context(), "", "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	now := time.Now()
	resp := make([]problemListResponse, 0, len(lists))
	for _, l := range lists {
		resp = append(resp, problemListResponse{
			ProblemList: l,
			IsCurrent:   isCurrentWeek(l.WeekStart, l.WeekEnd, now),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// Get handles GET /api/v1/lists/:id, returning the list plus its items in
// ordinal order. Unauthenticated, always public: a draft (published=false)
// 404s for everyone, since there's no owner identity to grant it to.
func (h *ListsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := h.store.GetProblemListDetail(r.Context(), id, "", "")
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "list not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// Update handles PATCH /api/v1/lists/:id (teacher only, owner only — a
// non-owner's request 404s, same as a nonexistent list).
func (h *ListsHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body listBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Title == "" {
		writeError(w, http.StatusUnprocessableEntity, "title is required")
		return
	}
	weekStart, ok := parseOptionalDate(body.WeekStart)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "week_start must be YYYY-MM-DD")
		return
	}
	weekEnd, ok := parseOptionalDate(body.WeekEnd)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "week_end must be YYYY-MM-DD")
		return
	}

	id := chi.URLParam(r, "id")
	list, err := h.store.UpdateProblemList(r.Context(), id, claims.UserID, store.UpdateProblemListRequest{
		Title:       body.Title,
		WeekLabel:   body.WeekLabel,
		WeekStart:   weekStart,
		WeekEnd:     weekEnd,
		Description: body.Description,
		Published:   body.Published,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "list not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// Delete handles DELETE /api/v1/lists/:id (teacher only, owner only).
func (h *ListsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteProblemList(r.Context(), id, claims.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "list not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type listItemBody struct {
	Title      string `json:"title"`
	Difficulty string `json:"difficulty"`
	IsBonus    bool   `json:"is_bonus"`
	Body       string `json:"body"`
}

func (b listItemBody) validate() (string, bool) {
	if b.Title == "" || b.Body == "" {
		return "title and body are required", false
	}
	if !validListDifficulties[b.Difficulty] {
		return "difficulty must be easy, medium, or hard", false
	}
	return "", true
}

// CreateItem handles POST /api/v1/lists/:id/items (teacher only, owner of
// the parent list only).
func (h *ListsHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body listItemBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if msg, ok := body.validate(); !ok {
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}

	listID := chi.URLParam(r, "id")
	item, err := h.store.CreateListItem(r.Context(), listID, claims.UserID, store.CreateListItemRequest{
		Title:      body.Title,
		Difficulty: body.Difficulty,
		IsBonus:    body.IsBonus,
		Body:       body.Body,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "list not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// UpdateItem handles PATCH /api/v1/list-items/:id (teacher only, owner of
// the parent list only).
func (h *ListsHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body listItemBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if msg, ok := body.validate(); !ok {
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}

	itemID := chi.URLParam(r, "id")
	item, err := h.store.UpdateListItem(r.Context(), itemID, claims.UserID, store.UpdateListItemRequest{
		Title:      body.Title,
		Difficulty: body.Difficulty,
		IsBonus:    body.IsBonus,
		Body:       body.Body,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// DeleteItem handles DELETE /api/v1/list-items/:id (teacher only, owner of
// the parent list only).
func (h *ListsHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	itemID := chi.URLParam(r, "id")
	if err := h.store.DeleteListItem(r.Context(), itemID, claims.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type reorderBody struct {
	Items []struct {
		ItemID  string `json:"item_id"`
		Ordinal int    `json:"ordinal"`
	} `json:"items"`
}

// Reorder handles PATCH /api/v1/lists/:id/reorder (teacher only, owner
// only) — the whole batch is applied atomically by the store.
func (h *ListsHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body reorderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	listID := chi.URLParam(r, "id")
	items := make([]store.ReorderItem, 0, len(body.Items))
	for _, it := range body.Items {
		items = append(items, store.ReorderItem{ItemID: it.ItemID, Ordinal: it.Ordinal})
	}

	if err := h.store.ReorderListItems(r.Context(), listID, claims.UserID, items); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "list or item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CompleteItem handles POST /api/v1/list-items/:id/complete (student only).
// Idempotent: completing an already-completed item is a no-op, not an error.
func (h *ListsHandler) CompleteItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	itemID := chi.URLParam(r, "id")
	if err := h.store.CompleteListItem(r.Context(), itemID, claims.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UncompleteItem handles DELETE /api/v1/list-items/:id/complete (student
// only). Idempotent: unmarking an item that was never completed is a no-op.
func (h *ListsHandler) UncompleteItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	itemID := chi.URLParam(r, "id")
	if err := h.store.UncompleteListItem(r.Context(), itemID, claims.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
