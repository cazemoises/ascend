package handler

import (
	"net/http"

	"github.com/caze/ascend/api/internal/auth"
)

type MeResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	// RealRole is the caller's role in the database, unaffected by
	// middleware.ViewAs. EffectiveRole is what's actually being enforced
	// for this request (RequireRole checks it) — the frontend renders
	// against EffectiveRole but only shows the "view as" toggle when
	// RealRole is "teacher".
	RealRole      string `json:"real_role"`
	EffectiveRole string `json:"effective_role"`
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
	writeJSON(w, http.StatusOK, MeResponse{
		ID:            claims.UserID,
		Email:         claims.Email,
		RealRole:      claims.RealRole,
		EffectiveRole: claims.Role,
	})
}
