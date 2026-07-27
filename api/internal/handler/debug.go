package handler

import "net/http"

// DebugHeaders is a TEMPORARY diagnostic endpoint — added to check whether
// the Pangolin proxy in front of the VM forwards identity headers
// (Remote-User, Remote-Email, Remote-Name, Remote-Role) ahead of switching
// auth to trust Platform SSO instead of email/password. No auth, echoes
// back every header on the incoming request.
//
// DELETE THIS (and its route in router.go) once the investigation is done —
// it must never ship to production.
func DebugHeaders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, r.Header)
}
