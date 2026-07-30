package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

type ChallengeCollectionsHandler struct {
	store *store.Store
}

func NewChallengeCollectionsHandler(s *store.Store) *ChallengeCollectionsHandler {
	return &ChallengeCollectionsHandler{store: s}
}

type challengeCollectionBody struct {
	Title   string  `json:"title"`
	Ordinal *int    `json:"ordinal"`
	GroupID *string `json:"group_id"`
}

// List handles GET /api/v1/challenge-collections (teacher only): the
// caller's own collections, for the studio's "Vincular a coleção" dropdown.
func (h *ChallengeCollectionsHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	ccs, err := h.store.ListChallengeCollections(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, ccs)
}

// Create handles POST /api/v1/challenge-collections (teacher only).
func (h *ChallengeCollectionsHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body challengeCollectionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Title == "" {
		writeError(w, http.StatusUnprocessableEntity, "title is required")
		return
	}

	cc, err := h.store.CreateChallengeCollection(r.Context(), store.CreateChallengeCollectionRequest{
		TeacherID: claims.UserID,
		Title:     body.Title,
		Ordinal:   body.Ordinal,
		GroupID:   body.GroupID,
	})
	if err != nil {
		if errors.Is(err, store.ErrGroupNotFound) {
			writeError(w, http.StatusUnprocessableEntity, "group_id does not reference a challenge group you own")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, cc)
}

// Update handles PATCH /api/v1/challenge-collections/:id (teacher only,
// owner of the collection only).
func (h *ChallengeCollectionsHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body challengeCollectionBody
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
	cc, err := h.store.UpdateChallengeCollection(r.Context(), id, claims.UserID, store.UpdateChallengeCollectionRequest{
		Title:   body.Title,
		Ordinal: ordinal,
		GroupID: body.GroupID,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge collection not found")
			return
		}
		if errors.Is(err, store.ErrGroupNotFound) {
			writeError(w, http.StatusUnprocessableEntity, "group_id does not reference a challenge group you own")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, cc)
}

type collectionReorderBody struct {
	Items []struct {
		ID      string `json:"id"`
		Ordinal int    `json:"ordinal"`
	} `json:"items"`
}

// Reorder handles PATCH /api/v1/challenge-collections/reorder (teacher
// only, owner of every collection in the batch) — applied atomically by the
// store, mirroring ListsHandler.Reorder.
func (h *ChallengeCollectionsHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body collectionReorderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	items := make([]store.CollectionReorderItem, 0, len(body.Items))
	for _, it := range body.Items {
		items = append(items, store.CollectionReorderItem{ID: it.ID, Ordinal: it.Ordinal})
	}

	if err := h.store.ReorderChallengeCollections(r.Context(), claims.UserID, items); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge collection not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete handles DELETE /api/v1/challenge-collections/:id (teacher only,
// owner of the collection only). Challenges in the collection are un-linked
// (collection_id set to NULL), never deleted — enforced by the FK's ON
// DELETE SET NULL, not application code.
func (h *ChallengeCollectionsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteChallengeCollection(r.Context(), id, claims.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge collection not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
