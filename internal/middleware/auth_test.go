package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/auth"
)

func TestRequireAuth(t *testing.T) {
	var gotID int64
	var gotOK bool
	h := RequireAuth("k")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID, gotOK = UserID(r.Context())
	}))

	tests := []struct {
		name   string
		cookie *http.Cookie
		status int
	}{
		{"no cookie", nil, http.StatusUnauthorized},
		{"garbage", &http.Cookie{Name: SessionCookie, Value: "garbage"}, http.StatusUnauthorized},
		{"forged", &http.Cookie{Name: SessionCookie, Value: auth.Sign("other", 7)}, http.StatusUnauthorized},
		{"valid", &http.Cookie{Name: SessionCookie, Value: auth.Sign("k", 7)}, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotOK = 0, false
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if tt.status == http.StatusOK && (!gotOK || gotID != 7) {
				t.Fatalf("UserID = %d, %v", gotID, gotOK)
			}
			if tt.status != http.StatusOK && gotOK {
				t.Fatal("handler reached")
			}
		})
	}
}

func TestUserIDMissing(t *testing.T) {
	if _, ok := UserID(httptest.NewRequest(http.MethodGet, "/", nil).Context()); ok {
		t.Fatal("unexpected user ID")
	}
}
