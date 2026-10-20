package accrual

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/orders/1":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"order":"1","status":"PROCESSED","accrual":500.5}`))
		case "/api/orders/2":
			w.Write([]byte(`{"order":"2","status":"PROCESSING"}`))
		case "/api/orders/3":
			w.WriteHeader(http.StatusNoContent)
		case "/api/orders/4":
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
		case "/api/orders/5":
			w.WriteHeader(http.StatusTooManyRequests)
		case "/api/orders/6":
			w.Write([]byte(`not json`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL + "/")
	ctx := context.Background()

	res, err := c.GetOrder(ctx, "1")
	if err != nil || res.Status != "PROCESSED" || res.Accrual == nil || *res.Accrual != 500.5 {
		t.Fatalf("200: %+v, %v", res, err)
	}
	res, err = c.GetOrder(ctx, "2")
	if err != nil || res.Status != "PROCESSING" || res.Accrual != nil {
		t.Fatalf("200 without accrual: %+v, %v", res, err)
	}
	if _, err := c.GetOrder(ctx, "3"); !errors.Is(err, ErrNotRegistered) {
		t.Fatalf("204: %v", err)
	}
	var rl *RateLimitError
	if _, err := c.GetOrder(ctx, "4"); !errors.As(err, &rl) || rl.RetryAfter != 7*time.Second {
		t.Fatalf("429: %v", err)
	}
	if _, err := c.GetOrder(ctx, "5"); !errors.As(err, &rl) || rl.RetryAfter != 60*time.Second {
		t.Fatalf("429 default: %v", err)
	}
	if _, err := c.GetOrder(ctx, "6"); err == nil {
		t.Fatal("bad json: expected error")
	}
	if _, err := c.GetOrder(ctx, "7"); err == nil || errors.Is(err, ErrNotRegistered) {
		t.Fatalf("500: %v", err)
	}
}

func TestGetOrderNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	if _, err := NewClient(url).GetOrder(context.Background(), "1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRateLimitErrorMessage(t *testing.T) {
	if (&RateLimitError{RetryAfter: time.Second}).Error() == "" {
		t.Fatal("empty message")
	}
}
