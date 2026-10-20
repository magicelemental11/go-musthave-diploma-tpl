package handler

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/luhn"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/middleware"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/storage"
)

type orderResponse struct {
	Number     string   `json:"number"`
	Status     string   `json:"status"`
	Accrual    *float64 `json:"accrual,omitempty"`
	UploadedAt string   `json:"uploaded_at"`
}

func newOrderResponse(o storage.Order) orderResponse {
	resp := orderResponse{
		Number:     o.Number,
		Status:     o.Status,
		UploadedAt: o.UploadedAt.Format(time.RFC3339),
	}
	if o.Accrual != nil {
		v := points(*o.Accrual)
		resp.Accrual = &v
	}
	return resp
}

// UploadOrder registers an order number for the current user.
func (h *Handler) UploadOrder(w http.ResponseWriter, r *http.Request) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "text/plain" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	userID, _ := middleware.UserID(r.Context())
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	number := strings.TrimSpace(string(body))
	if number == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !luhn.Valid(number) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	created, err := h.store.AddOrder(r.Context(), userID, number)
	if errors.Is(err, storage.ErrOrderOwnedByAnother) {
		w.WriteHeader(http.StatusConflict)
		return
	}
	if err != nil {
		serverError(w, "add order", err)
		return
	}
	if created {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ListOrders returns the current user's orders, newest first.
func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	orders, err := h.store.ListOrders(r.Context(), userID)
	if err != nil {
		serverError(w, "list orders", err)
		return
	}
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	resp := make([]orderResponse, len(orders))
	for i, o := range orders {
		resp[i] = newOrderResponse(o)
	}
	writeJSON(w, resp)
}
