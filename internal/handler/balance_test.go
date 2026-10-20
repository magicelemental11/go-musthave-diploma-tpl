package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/storage"
)

func (f *fakeStore) GetBalance(context.Context, int64) (int64, int64, error) {
	return f.balance, f.withdrawn, f.err
}

func (f *fakeStore) Withdraw(_ context.Context, _ int64, order string, sum int64) error {
	if f.err != nil {
		return f.err
	}
	if sum > f.balance {
		return storage.ErrInsufficientFunds
	}
	f.balance -= sum
	f.withdrawn += sum
	f.withdrawals = append(f.withdrawals, storage.Withdrawal{Order: order, Sum: sum})
	return nil
}

func (f *fakeStore) ListWithdrawals(context.Context, int64) ([]storage.Withdrawal, error) {
	return f.withdrawals, f.err
}

func TestBalance(t *testing.T) {
	const path = "/api/user/balance"
	store := newFakeStore()
	store.balance, store.withdrawn = 50050, 4200
	h := New(store, "k").Router()

	if rec := do(h, http.MethodGet, path, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized: status = %d", rec.Code)
	}
	rec := do(h, http.MethodGet, path, "", sessionFor(1))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type = %q", ct)
	}
	var got map[string]float64
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["current"] != 500.5 || got["withdrawn"] != 42 {
		t.Fatalf("got = %v", got)
	}

	store.err = errors.New("db down")
	if rec := do(h, http.MethodGet, path, "", sessionFor(1)); rec.Code != http.StatusInternalServerError {
		t.Fatalf("store error: status = %d", rec.Code)
	}
}

func TestWithdraw(t *testing.T) {
	const path = "/api/user/balance/withdraw"
	tests := []struct {
		name    string
		body    string
		balance int64
		cookie  *http.Cookie
		want    int
	}{
		{"unauthorized", `{"order":"2377225624","sum":1}`, 50050, nil, http.StatusUnauthorized},
		{"broken json", `{`, 50050, sessionFor(1), http.StatusBadRequest},
		{"null body", `null`, 50050, sessionFor(1), http.StatusBadRequest},
		{"sum as string", `{"order":"2377225624","sum":"1"}`, 50050, sessionFor(1), http.StatusBadRequest},
		{"no order", `{"sum":1}`, 50050, sessionFor(1), http.StatusBadRequest},
		{"no sum", `{"order":"2377225624"}`, 50050, sessionFor(1), http.StatusBadRequest},
		{"zero sum", `{"order":"2377225624","sum":0}`, 50050, sessionFor(1), http.StatusBadRequest},
		{"negative sum", `{"order":"2377225624","sum":-5}`, 50050, sessionFor(1), http.StatusBadRequest},
		{"sum rounds to zero", `{"order":"2377225624","sum":0.001}`, 50050, sessionFor(1), http.StatusBadRequest},
		{"bad luhn", `{"order":"2377225625","sum":1}`, 50050, sessionFor(1), http.StatusUnprocessableEntity},
		{"insufficient", `{"order":"2377225624","sum":500.51}`, 50050, sessionFor(1), http.StatusPaymentRequired},
		{"huge sum", `{"order":"2377225624","sum":1e300}`, 50050, sessionFor(1), http.StatusPaymentRequired},
		{"empty balance", `{"order":"2377225624","sum":0.01}`, 0, sessionFor(1), http.StatusPaymentRequired},
		{"whole balance", `{"order":"2377225624","sum":500.5}`, 50050, sessionFor(1), http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			store.balance = tt.balance
			var cookies []*http.Cookie
			if tt.cookie != nil {
				cookies = append(cookies, tt.cookie)
			}
			rec := do(New(store, "k").Router(), http.MethodPost, path, tt.body, cookies...)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if (tt.want == http.StatusOK) != (len(store.withdrawals) == 1) {
				t.Fatalf("withdrawals = %+v", store.withdrawals)
			}
		})
	}
}

func TestWithdrawRounding(t *testing.T) {
	store := newFakeStore()
	store.balance = 10000
	h := New(store, "k").Router()
	rec := do(h, http.MethodPost, "/api/user/balance/withdraw", `{"order":"2377225624","sum":19.99}`, sessionFor(1))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := store.withdrawals[0].Sum; got != 1999 {
		t.Fatalf("sum = %d, want 1999", got)
	}
}

func TestWithdrawStoreError(t *testing.T) {
	store := newFakeStore()
	store.err = errors.New("db down")
	rec := do(New(store, "k").Router(), http.MethodPost, "/api/user/balance/withdraw", `{"order":"2377225624","sum":1}`, sessionFor(1))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestWithdrawals(t *testing.T) {
	const path = "/api/user/withdrawals"
	processed := time.Date(2020, 12, 9, 16, 9, 57, 0, time.UTC)
	store := newFakeStore()
	h := New(store, "k").Router()

	if rec := do(h, http.MethodGet, path, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized: status = %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, path, "", sessionFor(1)); rec.Code != http.StatusNoContent {
		t.Fatalf("empty: status = %d", rec.Code)
	}

	store.withdrawals = []storage.Withdrawal{
		{Order: "2377225624", Sum: 50000, ProcessedAt: processed},
		{Order: "12345678903", Sum: 150, ProcessedAt: processed.Add(-time.Hour)},
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
	if len(got) != 2 || got[0]["order"] != "2377225624" || got[0]["sum"] != 500.0 ||
		got[0]["processed_at"] != "2020-12-09T16:09:57Z" || got[1]["sum"] != 1.5 {
		t.Fatalf("got = %v", got)
	}

	store.err = errors.New("db down")
	if rec := do(h, http.MethodGet, path, "", sessionFor(1)); rec.Code != http.StatusInternalServerError {
		t.Fatalf("store error: status = %d", rec.Code)
	}
}
