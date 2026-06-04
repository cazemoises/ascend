package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/caze/ascend/api/internal/store"
)

type ChallengesHandler struct {
	store *store.Store
}

func NewChallengesHandler(s *store.Store) *ChallengesHandler {
	return &ChallengesHandler{store: s}
}

func (h *ChallengesHandler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Delete("/{id}", h.delete)
	r.Post("/{id}/test-cases", h.createTestCase)
	r.Get("/{id}/test-cases", h.listTestCases)
	r.Post("/{id}/submissions", h.createSubmission)
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

func (h *ChallengesHandler) list(w http.ResponseWriter, r *http.Request) {
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

	challenges, err := h.store.ListChallenges(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, challenges)
}

type createChallengeBody struct {
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Difficulty    string `json:"difficulty"`
	TimeLimitMs   int    `json:"time_limit_ms"`
	MemoryLimitMb int    `json:"memory_limit_mb"`
}

func (h *ChallengesHandler) create(w http.ResponseWriter, r *http.Request) {
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

func (h *ChallengesHandler) delete(w http.ResponseWriter, r *http.Request) {
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

func (h *ChallengesHandler) createTestCase(w http.ResponseWriter, r *http.Request) {
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

func (h *ChallengesHandler) listTestCases(w http.ResponseWriter, r *http.Request) {
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
