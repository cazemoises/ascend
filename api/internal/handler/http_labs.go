package handler

import (
	"encoding/json"
	"errors"
	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strings"
)

func (h *ChallengesHandler) CreateHTTPLab(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	var in store.HTTPLabInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024)).Decode(&in); err != nil {
		writeError(w, 400, "invalid HTTP lab JSON")
		return
	}
	if err := in.Validate(); err != nil {
		writeError(w, 422, err.Error())
		return
	}
	id, err := h.store.CreateHTTPLab(r.Context(), chi.URLParam(r, "id"), claims.UserID, in)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "list item not found")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		writeError(w, 409, "item already has an automatic assessment")
		return
	}
	if err != nil {
		writeError(w, 500, "could not create HTTP lab")
		return
	}
	writeJSON(w, 201, map[string]string{"challenge_id": id})
}
func (h *ChallengesHandler) ChallengeAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" || id == "import" {
			next.ServeHTTP(w, r)
			return
		}
		claims, _ := auth.FromContext(r.Context())
		write := r.Method != "GET" && !strings.HasSuffix(r.URL.Path, "/submissions")
		if strings.HasSuffix(r.URL.Path, "/test-cases") {
			write = true
		}
		ok, err := h.store.CanAccessChallenge(r.Context(), id, claims.UserID, claims.Role == "teacher", write)
		if err != nil {
			writeError(w, 500, "could not check challenge access")
			return
		}
		if !ok {
			writeError(w, 404, "challenge not found")
			return
		}
		if write && r.Method != "GET" {
			lab, err := h.store.IsHTTPLab(r.Context(), id)
			if err != nil {
				writeError(w, 500, "could not check lab")
				return
			}
			if lab {
				writeError(w, 409, "HTTP lab configuration is immutable; create a new list item for a new version")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
