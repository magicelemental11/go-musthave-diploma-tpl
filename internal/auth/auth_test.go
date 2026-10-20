package auth

import (
	"strings"
	"testing"
)

func TestPassword(t *testing.T) {
	h, err := HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "secret") {
		t.Fatal("correct password rejected")
	}
	if CheckPassword(h, "other") {
		t.Fatal("wrong password accepted")
	}
}

func TestHashPasswordTooLong(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("a", 100)); err != ErrPasswordTooLong {
		t.Fatalf("err = %v", err)
	}
}

func TestSignParse(t *testing.T) {
	id, ok := Parse("k", Sign("k", 42))
	if !ok || id != 42 {
		t.Fatalf("got %d, %v", id, ok)
	}
}

func TestParseRejects(t *testing.T) {
	valid := Sign("k", 42)
	tests := map[string]string{
		"empty":        "",
		"no dot":       "42",
		"wrong secret": Sign("other", 42),
		"swapped id":   "43" + valid[strings.Index(valid, "."):],
		"bad sig":      "42.deadbeef",
		"non numeric":  "x." + mac("k", "x"),
		"zero id":      Sign("k", 0),
		"negative id":  Sign("k", -1),
	}
	for name, v := range tests {
		t.Run(name, func(t *testing.T) {
			if _, ok := Parse("k", v); ok {
				t.Fatalf("%q accepted", v)
			}
		})
	}
}

func TestSpendCheck(t *testing.T) {
	if !CheckPassword(dummyHash(), "dummy") {
		t.Fatal("dummy hash does not match its password")
	}
	SpendCheck("anything")
}
