package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

type ChallengesHandler struct {
	store *store.Store
}

func NewChallengesHandler(s *store.Store) *ChallengesHandler {
	return &ChallengesHandler{store: s}
}

// Routes registers only the public read endpoints. The list endpoint (List)
// is wired by the router behind optional auth so the feed can be filtered by
// class enrollment; teacher-only management and the authenticated submission
// route are wired by the router with the appropriate middleware.
func (h *ChallengesHandler) Routes(r chi.Router) {
	r.Get("/{id}", h.get)
	r.Get("/{id}/submissions", h.listSubmissions)
}

var validDifficulties = map[string]bool{
	"easy":   true,
	"medium": true,
	"hard":   true,
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// List serves the challenge feed filtered by the viewer's visibility:
// anonymous → public only; student → public + enrolled classes; teacher →
// everything. Identity (if any) comes from middleware.PangolinAuth, which
// runs globally and never rejects a request by itself.
func (h *ChallengesHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	viewerID, role := "", ""
	if claims, ok := auth.FromContext(r.Context()); ok {
		viewerID, role = claims.UserID, claims.Role
	}

	challenges, err := h.store.ListChallengesForViewer(r.Context(), viewerID, role, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, challenges)
}

// Stats serves the accepted-run timing distribution for a challenge plus the
// authenticated user's percentile standing (JWT-protected).
func (h *ChallengesHandler) Stats(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	stats, err := h.store.GetChallengeStats(r.Context(), id, claims.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// TeacherScoreboard serves the per-class completion matrix for every class
// owned by the authenticated teacher (teacher-only route).
func (h *ChallengesHandler) TeacherScoreboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	scores, err := h.store.TeacherScoreboard(r.Context(), claims.UserID)
	if err != nil {
		log.Printf("teacher scoreboard for user %s: %v", claims.UserID, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if scores == nil {
		scores = []store.ClassScore{}
	}
	writeJSON(w, http.StatusOK, scores)
}

type createChallengeBody struct {
	Slug          string  `json:"slug"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Difficulty    string  `json:"difficulty"`
	TimeLimitMs   int     `json:"time_limit_ms"`
	MemoryLimitMb int     `json:"memory_limit_mb"`
	Notes         *string `json:"notes"`
	StarterCode   *string `json:"starter_code"`
	ClassID       *string `json:"class_id"`
}

func (h *ChallengesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body createChallengeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Slug == "" || body.Title == "" || body.Difficulty == "" {
		writeError(w, http.StatusUnprocessableEntity, "slug, title, and difficulty are required")
		return
	}
	if !validDifficulties[body.Difficulty] {
		writeError(w, http.StatusUnprocessableEntity, "difficulty must be easy, medium, or hard")
		return
	}

	c, err := h.store.CreateChallenge(r.Context(), store.CreateChallengeRequest{
		Slug:          body.Slug,
		Title:         body.Title,
		Description:   body.Description,
		Difficulty:    body.Difficulty,
		TimeLimitMs:   body.TimeLimitMs,
		MemoryLimitMb: body.MemoryLimitMb,
		Notes:         body.Notes,
		StarterCode:   body.StarterCode,
		ClassID:       body.ClassID,
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "slug already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *ChallengesHandler) Update(w http.ResponseWriter, r *http.Request) {
	var body createChallengeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Slug == "" || body.Title == "" || body.Difficulty == "" {
		writeError(w, http.StatusUnprocessableEntity, "slug, title, and difficulty are required")
		return
	}
	if !validDifficulties[body.Difficulty] {
		writeError(w, http.StatusUnprocessableEntity, "difficulty must be easy, medium, or hard")
		return
	}

	id := chi.URLParam(r, "id")
	c, err := h.store.UpdateChallenge(r.Context(), id, store.CreateChallengeRequest{
		Slug:          body.Slug,
		Title:         body.Title,
		Description:   body.Description,
		Difficulty:    body.Difficulty,
		TimeLimitMs:   body.TimeLimitMs,
		MemoryLimitMb: body.MemoryLimitMb,
		Notes:         body.Notes,
		StarterCode:   body.StarterCode,
		ClassID:       body.ClassID,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "slug already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *ChallengesHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := h.store.GetChallengeDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *ChallengesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.store.DeleteChallenge(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "challenge is referenced by submissions")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type createTestCaseBody struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	IsSample       bool   `json:"is_sample"`
}

func (h *ChallengesHandler) CreateTestCase(w http.ResponseWriter, r *http.Request) {
	var body createTestCaseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.ExpectedOutput == "" {
		writeError(w, http.StatusUnprocessableEntity, "expected_output is required")
		return
	}

	id := chi.URLParam(r, "id")
	tc, err := h.store.CreateTestCase(r.Context(), id, store.CreateTestCaseRequest{
		Input:          body.Input,
		ExpectedOutput: body.ExpectedOutput,
		IsSample:       body.IsSample,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, tc)
}

type replaceTestCasesBody struct {
	TestCases []createTestCaseBody `json:"test_cases"`
}

func (h *ChallengesHandler) ReplaceTestCases(w http.ResponseWriter, r *http.Request) {
	var body replaceTestCasesBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	reqs := make([]store.CreateTestCaseRequest, 0, len(body.TestCases))
	for i, tc := range body.TestCases {
		if tc.ExpectedOutput == "" {
			writeError(w, http.StatusUnprocessableEntity,
				"expected_output is required (test case "+strconv.Itoa(i+1)+")")
			return
		}
		reqs = append(reqs, store.CreateTestCaseRequest{
			Input:          tc.Input,
			ExpectedOutput: tc.ExpectedOutput,
			IsSample:       tc.IsSample,
		})
	}

	id := chi.URLParam(r, "id")
	tcs, err := h.store.ReplaceTestCases(r.Context(), id, reqs)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, tcs)
}

func (h *ChallengesHandler) ListTestCases(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tcs, err := h.store.ListTestCases(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, tcs)
}

func (h *ChallengesHandler) listSubmissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	subs, err := h.store.ListRecentSubmissions(r.Context(), id, 10)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, subs)
}
