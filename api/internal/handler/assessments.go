package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
	"github.com/go-chi/chi/v5"
)

type AssessmentsHandler struct{ store *store.Store }

func NewAssessmentsHandler(s *store.Store) *AssessmentsHandler { return &AssessmentsHandler{store: s} }

func (h *AssessmentsHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	a, err := h.store.GetAssessment(r.Context(), chi.URLParam(r, "id"), claims.UserID, claims.Role == "teacher", false)
	if h.writeFailure(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, a.Assessment)
}
func (h *AssessmentsHandler) Authoring(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	a, err := h.store.GetAssessment(r.Context(), chi.URLParam(r, "id"), claims.UserID, claims.Role == "teacher", true)
	if h.writeFailure(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, a)
}
func (h *AssessmentsHandler) Save(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	var input store.AssessmentInput
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid assessment JSON")
		return
	}
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a, err := h.store.SaveAssessment(r.Context(), chi.URLParam(r, "id"), claims.UserID, input)
	if h.writeFailure(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, a)
}
func realAssessmentStudent(w http.ResponseWriter, r *http.Request) (auth.Claims, bool) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return claims, false
	}
	if claims.Role != "student" || claims.RealRole != "student" {
		writeError(w, http.StatusForbidden, "only students can answer assessments")
		return claims, false
	}
	return claims, true
}
func (h *AssessmentsHandler) Answer(w http.ResponseWriter, r *http.Request) {
	claims, ok := realAssessmentStudent(w, r)
	if !ok {
		return
	}
	var input struct {
		RevisionID string `json:"revision_id"`
		Answer     string `json:"answer"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid answer JSON")
		return
	}
	if input.RevisionID == "" || strings.TrimSpace(input.Answer) == "" || len(input.Answer) > 2000 {
		writeError(w, http.StatusUnprocessableEntity, "revision_id and answer (1 to 2000 bytes) are required")
		return
	}
	attempt, err := h.store.AnswerAssessment(r.Context(), chi.URLParam(r, "id"), claims.UserID, input.RevisionID, input.Answer)
	if h.writeFailure(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, attempt)
}
func (h *AssessmentsHandler) Attempts(w http.ResponseWriter, r *http.Request) {
	claims, ok := realAssessmentStudent(w, r)
	if !ok {
		return
	}
	attempts, err := h.store.AssessmentAttempts(r.Context(), chi.URLParam(r, "id"), claims.UserID)
	if h.writeFailure(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, attempts)
}
func (h *AssessmentsHandler) writeFailure(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "assessment not found")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "assessment changed; reload the current question")
	case errors.Is(err, store.ErrLinkedItem):
		writeError(w, http.StatusConflict, "linked challenges use the code judge")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
	return true
}
