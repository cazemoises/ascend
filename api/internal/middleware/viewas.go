package middleware

import (
	"net/http"

	"github.com/caze/ascend/api/internal/auth"
)

const viewAsHeader = "X-View-As"

// ViewAs lets a real teacher preview the app as a student would, without
// touching their role in the database. It must run after PangolinAuth,
// which populates the real identity: when the caller's RealRole is
// "teacher" and the X-View-As header is "student", it overrides Role (the
// effective role RequireRole checks) to "student" for this request only.
//
// The header is only ever honored when RealRole is already "teacher" — a
// caller who isn't really a teacher cannot use it to escalate or change
// anything about how they're authorized.
func ViewAs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.FromContext(r.Context())
		if !ok || claims.RealRole != "teacher" || r.Header.Get(viewAsHeader) != "student" {
			next.ServeHTTP(w, r)
			return
		}

		claims.Role = "student"
		next.ServeHTTP(w, r.WithContext(auth.NewContext(r.Context(), claims)))
	})
}
