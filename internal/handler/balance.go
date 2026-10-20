package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/luhn"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/middleware"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/storage"
)

type balanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type withdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

type withdrawalResponse struct {
	Order       string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

func newWithdrawalResponse(wd storage.Withdrawal) withdrawalResponse {
	return withdrawalResponse{
		Order:       wd.Order,
		Sum:         points(wd.Sum),
		ProcessedAt: wd.ProcessedAt.Format(time.RFC3339),
	}
}

// Balance returns the current balance and the total withdrawn amount.
func (h *Handler) Balance(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	current, withdrawn, err := h.store.GetBalance(r.Context(), userID)
	if err != nil {
		serverError(w, "get balance", err)
		return
	}
	writeJSON(w, balanceResponse{Current: points(current), Withdrawn: points(withdrawn)})
}

// Withdraw spends points from the balance on a new order.
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	var req withdrawRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Order == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sum, ok := toHundredths(req.Sum)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !luhn.Valid(req.Order) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	err := h.store.Withdraw(r.Context(), userID, req.Order, sum)
	if errors.Is(err, storage.ErrInsufficientFunds) {
		w.WriteHeader(http.StatusPaymentRequired)
		return
	}
	if err != nil {
		serverError(w, "withdraw", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Withdrawals returns the current user's withdrawals, newest first.
func (h *Handler) Withdrawals(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	list, err := h.store.ListWithdrawals(r.Context(), userID)
	if err != nil {
		serverError(w, "list withdrawals", err)
		return
	}
	if len(list) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	resp := make([]withdrawalResponse, len(list))
	for i, wd := range list {
		resp[i] = newWithdrawalResponse(wd)
	}
	writeJSON(w, resp)
}
