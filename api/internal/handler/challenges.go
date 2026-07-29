package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

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

// Routes registers only the public read endpoints. The list endpoint (List),
// teacher-only management, and the authenticated submission route are wired
// by the router with the appropriate middleware.
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

// List serves every challenge, newest first, annotated with whether the
// requesting viewer has already solved it. Identity (if any) comes from
// middleware.PangolinAuth, which runs globally and never rejects a request
// by itself.
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

	viewerID := ""
	if claims, ok := auth.FromContext(r.Context()); ok {
		viewerID = claims.UserID
	}

	challenges, err := h.store.ListChallengesForViewer(r.Context(), viewerID, limit, offset)
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

type createChallengeBody struct {
	Slug          string  `json:"slug"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Difficulty    string  `json:"difficulty"`
	TimeLimitMs   int     `json:"time_limit_ms"`
	MemoryLimitMb int     `json:"memory_limit_mb"`
	Notes         *string `json:"notes"`
	StarterCode   *string `json:"starter_code"`
	Language      *string `json:"language"`
	SQLSchema     *string `json:"sql_schema"`
	CollectionID  *string `json:"collection_id"`
}

// validateSQLFields enforces the scope of the SQL challenge modality: the
// only accepted explicit language is "sql" (nil/omitted keeps today's
// multi-language behavior), and a SQL-only challenge must carry the shared
// DDL/seed schema its test cases run against. Returns "" when valid.
func validateSQLFields(language, sqlSchema *string) string {
	if language == nil || *language == "" {
		return ""
	}
	if *language != "sql" {
		return `language must be omitted or "sql"`
	}
	if sqlSchema == nil || strings.TrimSpace(*sqlSchema) == "" {
		return `sql_schema is required when language is "sql"`
	}
	return ""
}

// validate is shared by Create, Update, and Import so all three enforce the
// exact same rules. Returns "" when valid.
func (b createChallengeBody) validate() string {
	if b.Slug == "" || b.Title == "" || b.Difficulty == "" {
		return "slug, title, and difficulty are required"
	}
	if !validDifficulties[b.Difficulty] {
		return "difficulty must be easy, medium, or hard"
	}
	if msg := validateSQLFields(b.Language, b.SQLSchema); msg != "" {
		return msg
	}
	return ""
}

func (h *ChallengesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body createChallengeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if msg := body.validate(); msg != "" {
		writeError(w, http.StatusUnprocessableEntity, msg)
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
		Language:      body.Language,
		SQLSchema:     body.SQLSchema,
		CollectionID:  body.CollectionID,
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "slug already exists")
			return
		}
		if errors.Is(err, store.ErrCollectionNotFound) {
			writeError(w, http.StatusUnprocessableEntity, "collection_id does not reference an existing challenge collection")
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
	if msg := body.validate(); msg != "" {
		writeError(w, http.StatusUnprocessableEntity, msg)
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
		Language:      body.Language,
		SQLSchema:     body.SQLSchema,
		CollectionID:  body.CollectionID,
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
		if errors.Is(err, store.ErrCollectionNotFound) {
			writeError(w, http.StatusUnprocessableEntity, "collection_id does not reference an existing challenge collection")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *ChallengesHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	viewerID := ""
	if claims, ok := auth.FromContext(r.Context()); ok {
		viewerID = claims.UserID
	}
	detail, err := h.store.GetChallengeDetail(r.Context(), id, viewerID)
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
	OrderMatters   bool   `json:"order_matters"`
}

// validate is shared by CreateTestCase, ReplaceTestCases, and Import so all
// three enforce the exact same rule. Returns "" when valid.
func (b createTestCaseBody) validate() string {
	if b.ExpectedOutput == "" {
		return "expected_output is required"
	}
	return ""
}

func (h *ChallengesHandler) CreateTestCase(w http.ResponseWriter, r *http.Request) {
	var body createTestCaseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if msg := body.validate(); msg != "" {
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}

	id := chi.URLParam(r, "id")
	tc, err := h.store.CreateTestCase(r.Context(), id, store.CreateTestCaseRequest{
		Input:          body.Input,
		ExpectedOutput: body.ExpectedOutput,
		IsSample:       body.IsSample,
		OrderMatters:   body.OrderMatters,
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
		if msg := tc.validate(); msg != "" {
			writeError(w, http.StatusUnprocessableEntity,
				"test case "+strconv.Itoa(i+1)+": "+msg)
			return
		}
		reqs = append(reqs, store.CreateTestCaseRequest{
			Input:          tc.Input,
			ExpectedOutput: tc.ExpectedOutput,
			IsSample:       tc.IsSample,
			OrderMatters:   tc.OrderMatters,
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

// importTestCaseBody adds an optional explicit Ordinal to createTestCaseBody
// (validated identically via the embedded validate()) — omitted means "use
// this test case's position in the array", same as ReplaceTestCases.
type importTestCaseBody struct {
	createTestCaseBody
	Ordinal *int `json:"ordinal"`
}

// importChallengeBody adds the full test suite to createChallengeBody,
// validated identically via the embedded validate().
type importChallengeBody struct {
	createChallengeBody
	TestCases []importTestCaseBody `json:"test_cases"`
}

type importChallengesBody struct {
	Challenges []importChallengeBody `json:"challenges"`
	// CollectionTitle, when present, links every challenge in the batch to
	// that collection (matched case-insensitively against the caller's own
	// collections, created if none matches) — a whole import normally
	// belongs to one collection, so this lives at the payload root rather
	// than per challenge.
	CollectionTitle *string `json:"collection_title"`
}

// Import handles POST /api/v1/challenges/import (teacher only). Creates
// every challenge and its full test suite from the JSON payload in one
// transaction — any challenge or test case failing validation, or any slug
// collision (within the batch or against an existing challenge), rolls back
// the whole request, so nothing is persisted half-imported. Validation
// reuses createChallengeBody.validate() and createTestCaseBody.validate(),
// the exact same rules Create/CreateTestCase enforce.
func (h *ChallengesHandler) Import(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body importChallengesBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	reqs := make([]store.ImportChallengeRequest, 0, len(body.Challenges))
	for i, ch := range body.Challenges {
		if msg := ch.validate(); msg != "" {
			writeError(w, http.StatusUnprocessableEntity, "challenge "+strconv.Itoa(i)+": "+msg)
			return
		}

		tcReqs := make([]store.ImportTestCaseRequest, 0, len(ch.TestCases))
		for j, tc := range ch.TestCases {
			if msg := tc.validate(); msg != "" {
				writeError(w, http.StatusUnprocessableEntity,
					"challenge "+strconv.Itoa(i)+", test case "+strconv.Itoa(j)+": "+msg)
				return
			}
			tcReqs = append(tcReqs, store.ImportTestCaseRequest{
				Input:          tc.Input,
				ExpectedOutput: tc.ExpectedOutput,
				IsSample:       tc.IsSample,
				Ordinal:        tc.Ordinal,
				OrderMatters:   tc.OrderMatters,
			})
		}

		reqs = append(reqs, store.ImportChallengeRequest{
			CreateChallengeRequest: store.CreateChallengeRequest{
				Slug:          ch.Slug,
				Title:         ch.Title,
				Description:   ch.Description,
				Difficulty:    ch.Difficulty,
				TimeLimitMs:   ch.TimeLimitMs,
				MemoryLimitMb: ch.MemoryLimitMb,
				Notes:         ch.Notes,
				StarterCode:   ch.StarterCode,
				Language:      ch.Language,
				SQLSchema:     ch.SQLSchema,
				CollectionID:  ch.CollectionID,
			},
			TestCases: tcReqs,
		})
	}

	results, err := h.store.ImportChallenges(r.Context(), store.ImportChallengesRequest{
		TeacherID:       claims.UserID,
		CollectionTitle: body.CollectionTitle,
		Challenges:      reqs,
	})
	if err != nil {
		if errors.Is(err, store.ErrCollectionNotFound) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		var conflictErr *store.ImportConflictError
		if errors.As(err, &conflictErr) {
			writeError(w, http.StatusUnprocessableEntity, conflictErr.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, results)
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
