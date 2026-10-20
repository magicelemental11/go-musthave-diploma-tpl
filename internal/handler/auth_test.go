package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/middleware"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/storage"
)

type fakeStore struct {
	users  map[string]storage.User
	owner  map[string]int64
	orders []storage.Order

	balance, withdrawn int64
	withdrawals        []storage.Withdrawal
	err                error
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: map[string]storage.User{}, owner: map[string]int64{}}
}

func (f *fakeStore) CreateUser(_ context.Context, login, hash string) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	if _, ok := f.users[login]; ok {
		return 0, storage.ErrLoginTaken
	}
	id := int64(len(f.users) + 1)
	f.users[login] = storage.User{ID: id, Login: login, PasswordHash: hash}
	return id, nil
}

func (f *fakeStore) GetUserByLogin(_ context.Context, login string) (storage.User, error) {
	if f.err != nil {
		return storage.User{}, f.err
	}
	u, ok := f.users[login]
	if !ok {
		return storage.User{}, storage.ErrNotFound
	}
	return u, nil
}

func do(h http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func session(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == middleware.SessionCookie {
			return c
		}
	}
	return nil
}

func TestRegister(t *testing.T) {
	store := newFakeStore()
	h := New(store, "k").Router()

	rec := do(h, http.MethodPost, "/api/user/register", `{"login":"a","password":"p"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	c := session(rec)
	if c == nil || !c.HttpOnly || c.Path != "/" {
		t.Fatalf("cookie = %+v", c)
	}
	if store.users["a"].PasswordHash == "p" {
		t.Fatal("password stored in plain text")
	}

	if rec := do(h, http.MethodGet, "/api/user/balance", "", c); rec.Code != http.StatusOK {
		t.Fatalf("authorized request status = %d", rec.Code)
	}

	if rec := do(h, http.MethodPost, "/api/user/register", `{"login":"a","password":"p"}`); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d", rec.Code)
	}
}

func TestBadRequests(t *testing.T) {
	h := New(newFakeStore(), "k").Router()
	bodies := map[string]string{
		"broken json":    `{`,
		"empty":          ``,
		"no login":       `{"password":"p"}`,
		"no password":    `{"login":"a"}`,
		"password > 72b": `{"login":"a","password":"` + strings.Repeat("x", 100) + `"}`,
	}
	for _, path := range []string{"/api/user/register", "/api/user/login"} {
		for name, body := range bodies {
			if path == "/api/user/login" && name == "password > 72b" {
				continue
			}
			t.Run(path+" "+name, func(t *testing.T) {
				if rec := do(h, http.MethodPost, path, body); rec.Code != http.StatusBadRequest {
					t.Fatalf("status = %d", rec.Code)
				}
			})
		}
	}
}

func TestLogin(t *testing.T) {
	h := New(newFakeStore(), "k").Router()
	do(h, http.MethodPost, "/api/user/register", `{"login":"a","password":"p"}`)

	rec := do(h, http.MethodPost, "/api/user/login", `{"login":"a","password":"p"}`)
	if rec.Code != http.StatusOK || session(rec) == nil {
		t.Fatalf("status = %d, cookie = %v", rec.Code, session(rec))
	}

	for name, body := range map[string]string{
		"wrong password": `{"login":"a","password":"x"}`,
		"unknown login":  `{"login":"b","password":"p"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := do(h, http.MethodPost, "/api/user/login", body)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d", rec.Code)
			}
			if session(rec) != nil {
				t.Fatal("cookie set on failure")
			}
		})
	}
}

func TestStoreErrors(t *testing.T) {
	store := newFakeStore()
	store.err = errors.New("db down")
	h := New(store, "k").Router()
	for _, path := range []string{"/api/user/register", "/api/user/login"} {
		if rec := do(h, http.MethodPost, path, `{"login":"a","password":"p"}`); rec.Code != http.StatusInternalServerError {
			t.Fatalf("%s status = %d", path, rec.Code)
		}
	}
}

func TestProtectedRoutesRequireAuth(t *testing.T) {
	h := New(newFakeStore(), "k").Router()
	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/user/orders"},
		{http.MethodGet, "/api/user/orders"},
		{http.MethodGet, "/api/user/balance"},
		{http.MethodPost, "/api/user/balance/withdraw"},
		{http.MethodGet, "/api/user/withdrawals"},
	}
	for _, rt := range routes {
		if rec := do(h, rt.method, rt.path, ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d", rt.method, rt.path, rec.Code)
		}
	}
}
