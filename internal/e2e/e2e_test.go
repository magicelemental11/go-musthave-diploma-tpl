// Package e2e holds end-to-end tests that run against a real PostgreSQL.
package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/accrual"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/storage"
)

func TestFlow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URI")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URI is not set")
	}
	if err := storage.Migrate(dsn); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store := storage.New(db)

	order := "12345678903"
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"order":"`+order+`","status":"PROCESSED","accrual":500}`)
	}))
	t.Cleanup(fake.Close)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	worker := accrual.NewWorker(accrual.NewClient(fake.URL), store, slog.Default(), 50*time.Millisecond, 2)
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	srv := httptest.NewServer(handler.New(store, "secret").Router())
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	login := "user" + time.Now().Format("150405.000000")

	do := func(method, path, body string, want int) []byte {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if strings.HasPrefix(body, "{") {
			req.Header.Set("Content-Type", "application/json")
		} else {
			req.Header.Set("Content-Type", "text/plain")
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("%s %s: status %d, want %d, body %s", method, path, resp.StatusCode, want, b)
		}
		return b
	}

	do(http.MethodPost, "/api/user/register", `{"login":"`+login+`","password":"pw"}`, http.StatusOK)
	do(http.MethodPost, "/api/user/orders", order, http.StatusAccepted)

	deadline := time.Now().Add(5 * time.Second)
	var bal struct{ Current, Withdrawn float64 }
	for time.Now().Before(deadline) {
		json.Unmarshal(do(http.MethodGet, "/api/user/balance", "", http.StatusOK), &bal)
		if bal.Current == 500 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if bal.Current != 500 {
		t.Fatalf("balance not accrued: %+v", bal)
	}

	do(http.MethodPost, "/api/user/balance/withdraw", `{"order":"2377225624","sum":200}`, http.StatusOK)
	json.Unmarshal(do(http.MethodGet, "/api/user/balance", "", http.StatusOK), &bal)
	if bal.Current != 300 || bal.Withdrawn != 200 {
		t.Fatalf("unexpected balance: %+v", bal)
	}

	var ws []struct{ Order string }
	json.Unmarshal(do(http.MethodGet, "/api/user/withdrawals", "", http.StatusOK), &ws)
	if len(ws) != 1 || ws[0].Order != "2377225624" {
		t.Fatalf("unexpected withdrawals: %+v", ws)
	}
}
