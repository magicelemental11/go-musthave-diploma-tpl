package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func fund(t *testing.T, s *Storage, userID, amount int64) {
	t.Helper()
	if _, err := s.db.Exec(`UPDATE users SET balance = $1 WHERE id = $2`, amount, userID); err != nil {
		t.Fatal(err)
	}
}

func TestWithdraw(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	u := newUser(t, s)
	fund(t, s, u, 50050)

	if err := s.Withdraw(ctx, u, "2377225624", 10000); err != nil {
		t.Fatal(err)
	}
	if err := s.Withdraw(ctx, u, "12345678903", 2000); err != nil {
		t.Fatal(err)
	}
	if err := s.Withdraw(ctx, u, "9278923470", 38051); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("overdraft: err = %v", err)
	}

	current, withdrawn, err := s.GetBalance(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	if current != 38050 || withdrawn != 12000 {
		t.Fatalf("balance = %d, withdrawn = %d", current, withdrawn)
	}

	list, err := s.ListWithdrawals(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Order != "12345678903" || list[0].Sum != 2000 || list[1].Order != "2377225624" {
		t.Fatalf("list = %+v", list)
	}

	if list, err := s.ListWithdrawals(ctx, newUser(t, s)); err != nil || len(list) != 0 {
		t.Fatalf("other user: list = %+v err = %v", list, err)
	}
}

func TestBalanceUnknownUser(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	const missing = int64(1) << 60

	if _, _, err := s.GetBalance(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetBalance: err = %v", err)
	}
	if err := s.Withdraw(ctx, missing, "2377225624", 1); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("Withdraw: err = %v", err)
	}
}

func TestWithdrawConcurrent(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	u := newUser(t, s)
	fund(t, s, u, 1000)

	const workers = 10
	start := make(chan struct{})
	var wg sync.WaitGroup
	var ok, denied atomic.Int32
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := s.Withdraw(ctx, u, fmt.Sprintf("order-%d", i), 300)
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, ErrInsufficientFunds):
				denied.Add(1)
			default:
				t.Error(err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if ok.Load() != 3 || denied.Load() != workers-3 {
		t.Fatalf("ok = %d, denied = %d", ok.Load(), denied.Load())
	}
	current, withdrawn, err := s.GetBalance(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	if current != 100 || withdrawn != 900 {
		t.Fatalf("balance = %d, withdrawn = %d", current, withdrawn)
	}
	if list, _ := s.ListWithdrawals(ctx, u); len(list) != 3 {
		t.Fatalf("withdrawals = %d", len(list))
	}
}
