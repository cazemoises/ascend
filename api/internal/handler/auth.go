package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

type AuthHandler struct {
	store *store.Store
	jwt   *auth.JWT
}

func NewAuthHandler(s *store.Store, j *auth.JWT) *AuthHandler {
	return &AuthHandler{store: s, jwt: j}
}

func (h *AuthHandler) Routes(r chi.Router) {
	r.Post("/register", h.register)
	r.Post("/login", h.login)
}

type credentialsBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string     `json:"token"`
	User  store.User `json:"user"`
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	var body credentialsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if addr, err := mail.ParseAddress(body.Email); err != nil || addr.Address != body.Email {
		writeError(w, http.StatusUnprocessableEntity, "invalid email address")
		return
	}
	if len(body.Password) < 8 || len(body.Password) > 72 {
		writeError(w, http.StatusUnprocessableEntity, "password must be between 8 and 72 characters")
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	u, err := h.store.CreateUser(r.Context(), body.Email, hash)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	token, err := h.jwt.Sign(u.ID, u.Email, u.Role, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{Token: token, User: u})
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var body credentialsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Email == "" || body.Password == "" {
		writeError(w, http.StatusUnprocessableEntity, "email and password are required")
		return
	}

	u, passwordHash, err := h.store.GetUserByEmail(r.Context(), body.Email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			auth.EqualizeTiming(body.Password)
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if err := auth.CheckPassword(passwordHash, body.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.jwt.Sign(u.ID, u.Email, u.Role, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token, User: u})
}
