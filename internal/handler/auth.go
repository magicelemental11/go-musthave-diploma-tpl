package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/auth"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/middleware"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/storage"
)

const maxBodyBytes = 1 << 20

type credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func readCredentials(w http.ResponseWriter, r *http.Request) (credentials, bool) {
	var c credentials
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil || c.Login == "" || c.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		return credentials{}, false
	}
	return c, true
}

func serverError(w http.ResponseWriter, msg string, err error) {
	slog.Error(msg, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}

func (h *Handler) startSession(w http.ResponseWriter, userID int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookie,
		Value:    auth.Sign(h.secret, userID),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusOK)
}

// Register creates a user and starts a session.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	c, ok := readCredentials(w, r)
	if !ok {
		return
	}
	hash, err := auth.HashPassword(c.Password)
	if errors.Is(err, auth.ErrPasswordTooLong) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err != nil {
		serverError(w, "hash password", err)
		return
	}
	id, err := h.store.CreateUser(r.Context(), c.Login, hash)
	if errors.Is(err, storage.ErrLoginTaken) {
		w.WriteHeader(http.StatusConflict)
		return
	}
	if err != nil {
		serverError(w, "create user", err)
		return
	}
	h.startSession(w, id)
}

// Login checks the credentials and starts a session.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	c, ok := readCredentials(w, r)
	if !ok {
		return
	}
	u, err := h.store.GetUserByLogin(r.Context(), c.Login)
	if errors.Is(err, storage.ErrNotFound) {
		auth.SpendCheck(c.Password)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if err != nil {
		serverError(w, "get user", err)
		return
	}
	if !auth.CheckPassword(u.PasswordHash, c.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	h.startSession(w, u.ID)
}
