package handler

import (
	"net/http"

	"github.com/caze/ascend/api/internal/store"
)

type TeacherHandler struct {
	store *store.Store
}

func NewTeacherHandler(s *store.Store) *TeacherHandler {
	return &TeacherHandler{store: s}
}

// StudentsOverview handles GET /api/v1/teacher/students-overview
// (teacher-only route, enforced by middleware): the flat roster of every
// student who has a real submission or a list item completion, each with
// their best verdict per challenge attempted and every list item completed.
func (h *TeacherHandler) StudentsOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.store.TeacherStudentsOverview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, overview)
}
