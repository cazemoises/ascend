package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

var validLanguages = map[string]bool{
	"go":         true,
	"python":     true,
	"javascript": true,
	"sql":        true,
	"java":       true,
}

type createSubmissionBody struct {
	Language   string `json:"language"`
	SourceCode string `json:"source_code"`
}

// CreateSubmission is registered by the router behind JWT auth, unlike the
// public routes wired in Routes.
func (h *ChallengesHandler) CreateSubmission(w http.ResponseWriter, r *http.Request) {
	var body createSubmissionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Language == "" {
		writeError(w, http.StatusUnprocessableEntity, "language is required")
		return
	}
	if !validLanguages[body.Language] {
		writeError(w, http.StatusUnprocessableEntity, "language must be go, python, javascript, java, or sql")
		return
	}
	if body.SourceCode == "" {
		writeError(w, http.StatusUnprocessableEntity, "source_code is required")
		return
	}

	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	challengeID := chi.URLParam(r, "id")
	sub, err := h.store.CreateSubmission(r.Context(), store.CreateSubmissionRequest{
		ChallengeID: challengeID,
		UserID:      claims.UserID,
		Language:    body.Language,
		SourceCode:  body.SourceCode,
		// Derived server-side from the verified JWT role — never trust this
		// from the request body, since a teacher testing their own challenge
		// shouldn't contaminate student-facing aggregate stats.
		IsTestRun: claims.Role == "teacher",
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		if errors.Is(err, store.ErrLanguageMismatch) {
			writeError(w, http.StatusUnprocessableEntity, "language does not match this challenge's language")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"submission_id": sub.ID})
}

type submissionHistoryResponse struct {
	Items      []store.UserSubmission `json:"items"`
	NextCursor *string                `json:"next_cursor"`
}

// encodeCursor packs the keyset position (created_at, id) of the last row
// into an opaque base64 token; decodeCursor is its inverse.
func encodeCursor(createdAt time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(strconv.FormatInt(createdAt.UnixNano(), 10) + "." + id))
}

func decodeCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("decode cursor: %w", err)
	}
	nanosStr, id, ok := strings.Cut(string(raw), ".")
	if !ok || id == "" {
		return time.Time{}, "", errors.New("malformed cursor")
	}
	nanos, err := strconv.ParseInt(nanosStr, 10, 64)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("parse cursor timestamp: %w", err)
	}
	return time.Unix(0, nanos), id, nil
}

// ListMySubmissions handles GET /api/v1/submissions (JWT-protected):
// the authenticated user's history, newest first, keyset-paginated via
// ?cursor= and ?limit= (default 20, max 100).
func (h *ChallengesHandler) ListMySubmissions(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	var beforeCreatedAt *time.Time
	var beforeID *string
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		createdAt, id, err := decodeCursor(cursor)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid cursor")
			return
		}
		beforeCreatedAt, beforeID = &createdAt, &id
	}

	// Fetch one extra row to detect whether another page exists.
	subs, err := h.store.ListUserSubmissions(r.Context(), claims.UserID, limit+1, beforeCreatedAt, beforeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := submissionHistoryResponse{Items: subs}
	if len(subs) > limit {
		resp.Items = subs[:limit]
		last := resp.Items[len(resp.Items)-1]
		cursor := encodeCursor(last.CreatedAt, last.ID)
		resp.NextCursor = &cursor
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *ChallengesHandler) GetSubmission(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sub, err := h.store.GetSubmission(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "submission not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, sub)
}
