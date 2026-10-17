package handler

import (
	"errors"
	"net/http"

	"github.com/caze/ascend/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func (h *LiveSessionsHandler) Results(w http.ResponseWriter, r *http.Request) {
	claims, ok := liveClaims(w, r)
	if !ok {
		return
	}
	result, err := h.store.LiveSessionResults(r.Context(), chi.URLParam(r, "id"), claims.UserID, claims.Role == "teacher")
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, "results require a finished session")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
