package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func testStorage(t *testing.T) *Storage {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URI")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URI is not set")
	}
	if err := Migrate(dsn); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db)
}

func TestUsers(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	login := fmt.Sprintf("user-%d", time.Now().UnixNano())

	id, err := s.CreateUser(ctx, login, "hash")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.CreateUser(ctx, login, "hash"); !errors.Is(err, ErrLoginTaken) {
		t.Fatalf("duplicate: err = %v", err)
	}

	u, err := s.GetUserByLogin(ctx, login)
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != id || u.Login != login || u.PasswordHash != "hash" {
		t.Fatalf("got %+v", u)
	}

	if _, err := s.GetUserByLogin(ctx, login+"-missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: err = %v", err)
	}
}
