// Package auth hashes passwords and signs session cookie values.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrPasswordTooLong is returned when a password exceeds the bcrypt limit.
var ErrPasswordTooLong = bcrypt.ErrPasswordTooLong

// HashPassword returns the bcrypt hash of password.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// CheckPassword reports whether password matches the bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

var dummyHash = sync.OnceValue(func() string {
	h, _ := HashPassword("dummy")
	return h
})

// SpendCheck performs a password check against a fixed hash so that a login
// attempt for an unknown user takes as long as one for a known user.
func SpendCheck(password string) {
	CheckPassword(dummyHash(), password)
}

// SessionTTL is how long a signed session value stays valid.
const SessionTTL = 24 * time.Hour

// Sign returns a session value of the form
// "<userID>.<issued unix seconds>.<hex hmac-sha256>".
func Sign(secret string, userID int64, issuedAt time.Time) string {
	payload := strconv.FormatInt(userID, 10) + "." + strconv.FormatInt(issuedAt.Unix(), 10)
	return payload + "." + mac(secret, payload)
}

// Parse validates a value produced by Sign and returns the user ID. Values
// older than SessionTTL or issued in the future are rejected.
func Parse(secret, value string) (int64, bool) {
	i := strings.LastIndexByte(value, '.')
	if i < 0 {
		return 0, false
	}
	payload, sig := value[:i], value[i+1:]
	if !hmac.Equal([]byte(sig), []byte(mac(secret, payload))) {
		return 0, false
	}
	id, issued, ok := strings.Cut(payload, ".")
	if !ok {
		return 0, false
	}
	userID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || userID <= 0 {
		return 0, false
	}
	unix, err := strconv.ParseInt(issued, 10, 64)
	if err != nil {
		return 0, false
	}
	age := time.Since(time.Unix(unix, 0))
	if age < 0 || age > SessionTTL {
		return 0, false
	}
	return userID, true
}

func mac(secret, data string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(data))
	return hex.EncodeToString(m.Sum(nil))
}
