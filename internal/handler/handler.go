// Package handler implements the HTTP API of the service.
package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/middleware"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/storage"
)

// UserStore is the user persistence used by the handlers.
type UserStore interface {
	CreateUser(ctx context.Context, login, passwordHash string) (int64, error)
	GetUserByLogin(ctx context.Context, login string) (storage.User, error)
}

// OrderStore is the order persistence used by the handlers.
type OrderStore interface {
	AddOrder(ctx context.Context, userID int64, number string) (bool, error)
	ListOrders(ctx context.Context, userID int64) ([]storage.Order, error)
}

// BalanceStore is the balance and withdrawal persistence used by the handlers.
type BalanceStore interface {
	GetBalance(ctx context.Context, userID int64) (current, withdrawn int64, err error)
	Withdraw(ctx context.Context, userID int64, order string, sum int64) error
	ListWithdrawals(ctx context.Context, userID int64) ([]storage.Withdrawal, error)
}

// Store is the persistence used by the handlers.
type Store interface {
	UserStore
	OrderStore
	BalanceStore
}

// Handler holds the dependencies of the HTTP handlers.
type Handler struct {
	store  Store
	secret string
}

// New returns a Handler that signs sessions with secret.
func New(store Store, secret string) *Handler {
	return &Handler{store: store, secret: secret}
}

// Router returns the API routes. Everything except registration and login
// requires a session.
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Post("/api/user/register", h.Register)
	r.Post("/api/user/login", h.Login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(h.secret))
		r.Post("/api/user/orders", h.UploadOrder)
		r.Get("/api/user/orders", h.ListOrders)
		r.Get("/api/user/balance", h.Balance)
		r.Post("/api/user/balance/withdraw", h.Withdraw)
		r.Get("/api/user/withdrawals", h.Withdrawals)
	})
	return r
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		serverError(w, "encode response", err)
	}
}
