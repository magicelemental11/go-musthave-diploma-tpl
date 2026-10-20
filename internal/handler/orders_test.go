package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/auth"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/middleware"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/storage"
)

func (f *fakeStore) AddOrder(_ context.Context, userID int64, number string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if owner, ok := f.owner[number]; ok {
		if owner != userID {
			return false, storage.ErrOrderOwnedByAnother
		}
		return false, nil
	}
	f.owner[number] = userID
	return true, nil
}

func (f *fakeStore) ListOrders(context.Context, int64) ([]storage.Order, error) {
	return f.orders, f.err
}

func sessionFor(id int64) *http.Cookie {
	return &http.Cookie{Name: middleware.SessionCookie, Value: auth.Sign("k", id)}
}

func upload(h http.Handler, contentType, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUploadOrder(t *testing.T) {
	h := New(newFakeStore(), "k").Router()

	tests := []struct {
		name        string
		contentType string
		body        string
		cookie      *http.Cookie
		want        int
	}{
		{"unauthorized", "text/plain", "12345678903", nil, http.StatusUnauthorized},
		{"empty", "text/plain", "", sessionFor(1), http.StatusBadRequest},
		{"no content type", "", "12345678903", sessionFor(1), http.StatusBadRequest},
		{"json content type", "application/json", "12345678903", sessionFor(1), http.StatusBadRequest},
		{"bad luhn", "text/plain", "12345678904", sessionFor(1), http.StatusUnprocessableEntity},
		{"letters", "text/plain", "1234a678903", sessionFor(1), http.StatusUnprocessableEntity},
		{"whitespace", "text/plain", " 12345678903\n", sessionFor(1), http.StatusUnprocessableEntity},
		{"new", "text/plain", "12345678903", sessionFor(1), http.StatusAccepted},
		{"charset parameter", "text/plain; charset=utf-8", "12345678903", sessionFor(1), http.StatusOK},
		{"same user again", "text/plain", "12345678903", sessionFor(1), http.StatusOK},
		{"another user", "text/plain", "12345678903", sessionFor(2), http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := upload(h, tt.contentType, tt.body, tt.cookie)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestUploadOrderStoreError(t *testing.T) {
	store := newFakeStore()
	store.err = errors.New("db down")
	rec := upload(New(store, "k").Router(), "text/plain", "12345678903", sessionFor(1))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestListOrders(t *testing.T) {
	const path = "/api/user/orders"
	accrual := int64(50050)
	uploaded := time.Date(2020, 12, 10, 15, 15, 45, 0, time.UTC)
	store := newFakeStore()
	h := New(store, "k").Router()

	if rec := do(h, http.MethodGet, path, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized: status = %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, path, "", sessionFor(1)); rec.Code != http.StatusNoContent {
		t.Fatalf("empty: status = %d", rec.Code)
	}

	store.orders = []storage.Order{
		{Number: "9278923470", Status: storage.StatusProcessed, Accrual: &accrual, UploadedAt: uploaded},
		{Number: "12345678903", Status: storage.StatusNew, UploadedAt: uploaded.Add(-time.Hour)},
	}
	rec := do(h, http.MethodGet, path, "", sessionFor(1))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type = %q", ct)
	}
	var got []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0]["number"] != "9278923470" || got[1]["number"] != "12345678903" {
		t.Fatalf("got = %v", got)
	}
	if got[0]["accrual"] != 500.5 {
		t.Fatalf("accrual = %v", got[0]["accrual"])
	}
	if got[0]["uploaded_at"] != "2020-12-10T15:15:45Z" {
		t.Fatalf("uploaded_at = %v", got[0]["uploaded_at"])
	}
	if _, ok := got[1]["accrual"]; ok {
		t.Fatalf("accrual present for unprocessed order: %v", got[1])
	}
}

func TestListOrdersStoreError(t *testing.T) {
	store := newFakeStore()
	store.err = errors.New("db down")
	rec := do(New(store, "k").Router(), http.MethodGet, "/api/user/orders", "", sessionFor(1))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}
