package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

type ChallengeGroupsHandler struct {
	store *store.Store
}

func NewChallengeGroupsHandler(s *store.Store) *ChallengeGroupsHandler {
	return &ChallengeGroupsHandler{store: s}
}

type challengeGroupBody struct {
	Title   string `json:"title"`
	Ordinal *int   `json:"ordinal"`
}

// List handles GET /api/v1/challenge-groups (teacher only): the caller's
// own groups, for the studio's "Grupo" dropdown on a collection.
func (h *ChallengeGroupsHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	cgs, err := h.store.ListChallengeGroups(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, cgs)
}

// Create handles POST /api/v1/challenge-groups (teacher only).
func (h *ChallengeGroupsHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body challengeGroupBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Title == "" {
		writeError(w, http.StatusUnprocessableEntity, "title is required")
		return
	}

	cg, err := h.store.CreateChallengeGroup(r.Context(), store.CreateChallengeGroupRequest{
		TeacherID: claims.UserID,
		Title:     body.Title,
		Ordinal:   body.Ordinal,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, cg)
}

// Update handles PATCH /api/v1/challenge-groups/:id (teacher only, owner
// of the group only).
func (h *ChallengeGroupsHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body challengeGroupBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Title == "" {
		writeError(w, http.StatusUnprocessableEntity, "title is required")
		return
	}
	ordinal := 0
	if body.Ordinal != nil {
		ordinal = *body.Ordinal
	}

	id := chi.URLParam(r, "id")
	cg, err := h.store.UpdateChallengeGroup(r.Context(), id, claims.UserID, store.UpdateChallengeGroupRequest{
		Title:   body.Title,
		Ordinal: ordinal,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge group not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, cg)
}

type groupReorderBody struct {
	Items []struct {
		ID      string `json:"id"`
		Ordinal int    `json:"ordinal"`
	} `json:"items"`
}

// Reorder handles PATCH /api/v1/challenge-groups/reorder (teacher only,
// owner of every group in the batch) — applied atomically by the store,
// mirroring ChallengeCollectionsHandler.Reorder.
func (h *ChallengeGroupsHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body groupReorderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	items := make([]store.GroupReorderItem, 0, len(body.Items))
	for _, it := range body.Items {
		items = append(items, store.GroupReorderItem{ID: it.ID, Ordinal: it.Ordinal})
	}

	if err := h.store.ReorderChallengeGroups(r.Context(), claims.UserID, items); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge group not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete handles DELETE /api/v1/challenge-groups/:id (teacher only, owner
// of the group only). Collections in the group are un-linked (group_id
// set to NULL), never deleted — enforced by the FK's ON DELETE SET NULL.
func (h *ChallengeGroupsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteChallengeGroup(r.Context(), id, claims.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge group not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
