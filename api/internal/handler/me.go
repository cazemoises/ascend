package handler

import (
	"net/http"

	"github.com/caze/ascend/api/internal/auth"
)

type MeResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Me returns the caller's identity as resolved by middleware.PangolinAuth
// from the Remote-Email header. 401 when no identity was resolved (neither
// the header nor DEV_FAKE_EMAIL was present upstream).
func Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, MeResponse{ID: claims.UserID, Email: claims.Email, Role: claims.Role})
}
