package storage

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestApplyAccrual(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	u := newUser(t, s)
	base := time.Now().UnixNano()
	processed, invalid := fmt.Sprintf("%d1", base), fmt.Sprintf("%d2", base)
	for _, n := range []string{processed, invalid} {
		if _, err := s.AddOrder(ctx, u, n); err != nil {
			t.Fatal(err)
		}
	}

	pending, err := s.ListPending(ctx, "", 100000)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, n := range pending {
		if n == processed || n == invalid {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("pending = %v", pending)
	}

	v := int64(50050)
	if err := s.ApplyAccrual(ctx, processed, StatusProcessing, nil); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := s.ApplyAccrual(ctx, processed, StatusProcessed, &v); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ApplyAccrual(ctx, invalid, StatusInvalid, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyAccrual(ctx, invalid, StatusProcessed, &v); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyAccrual(ctx, "missing", StatusProcessed, &v); err != nil {
		t.Fatal(err)
	}

	current, _, err := s.GetBalance(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	if current != 50050 {
		t.Fatalf("balance = %d, want 50050", current)
	}

	pending, err = s.ListPending(ctx, "", 100000)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range pending {
		if n == processed || n == invalid {
			t.Fatalf("final order %s still pending", n)
		}
	}
}
