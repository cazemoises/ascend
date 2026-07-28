package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caze/ascend/api/internal/auth"
)

func viewAsNext(got *auth.Claims, ok *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got, *ok = auth.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

func TestViewAs_Teacher_WithViewAsStudentHeader_OverridesEffectiveRole(t *testing.T) {
	var got auth.Claims
	var ok bool
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-View-As", "student")
	req = req.WithContext(auth.NewContext(req.Context(),
		auth.Claims{UserID: "t-1", Email: "teach@example.com", Role: "teacher", RealRole: "teacher"}))

	w := httptest.NewRecorder()
	ViewAs(viewAsNext(&got, &ok)).ServeHTTP(w, req)

	if !ok {
		t.Fatal("expected claims present")
	}
	if got.Role != "student" {
		t.Errorf("Role = %q, want %q (effective role overridden)", got.Role, "student")
	}
	if got.RealRole != "teacher" {
		t.Errorf("RealRole = %q, want %q (real role preserved)", got.RealRole, "teacher")
	}
}

func TestViewAs_Teacher_NoHeader_EffectiveRoleUnchanged(t *testing.T) {
	var got auth.Claims
	var ok bool
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.NewContext(req.Context(),
		auth.Claims{UserID: "t-1", Email: "teach@example.com", Role: "teacher", RealRole: "teacher"}))

	w := httptest.NewRecorder()
	ViewAs(viewAsNext(&got, &ok)).ServeHTTP(w, req)

	if !ok || got.Role != "teacher" || got.RealRole != "teacher" {
		t.Errorf("claims = %+v, want unchanged teacher", got)
	}
}

func TestViewAs_Student_HeaderIgnored_NoEscalation(t *testing.T) {
	var got auth.Claims
	var ok bool
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-View-As", "student")
	req = req.WithContext(auth.NewContext(req.Context(),
		auth.Claims{UserID: "s-1", Email: "student@example.com", Role: "student", RealRole: "student"}))

	w := httptest.NewRecorder()
	ViewAs(viewAsNext(&got, &ok)).ServeHTTP(w, req)

	if !ok || got.Role != "student" || got.RealRole != "student" {
		t.Errorf("claims = %+v, want unchanged student", got)
	}
}

func TestViewAs_NoClaims_PassesThroughAnonymous(t *testing.T) {
	var got auth.Claims
	var ok bool
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-View-As", "student")

	w := httptest.NewRecorder()
	ViewAs(viewAsNext(&got, &ok)).ServeHTTP(w, req)

	if ok {
		t.Errorf("expected no claims for an anonymous request, got %+v", got)
	}
}
