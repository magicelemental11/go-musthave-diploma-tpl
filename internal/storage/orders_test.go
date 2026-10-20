package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func newUser(t *testing.T, s *Storage) int64 {
	t.Helper()
	id, err := s.CreateUser(context.Background(), fmt.Sprintf("u-%d", time.Now().UnixNano()), "h")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestOrders(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	u1, u2 := newUser(t, s), newUser(t, s)
	n2 := fmt.Sprintf("%d", time.Now().UnixNano())
	n1 := n2 + "0"

	if created, err := s.AddOrder(ctx, u1, n1); err != nil || !created {
		t.Fatalf("first add: created=%v err=%v", created, err)
	}
	if created, err := s.AddOrder(ctx, u1, n1); err != nil || created {
		t.Fatalf("repeat add: created=%v err=%v", created, err)
	}
	if _, err := s.AddOrder(ctx, u2, n1); !errors.Is(err, ErrOrderOwnedByAnother) {
		t.Fatalf("foreign add: err=%v", err)
	}
	if _, err := s.AddOrder(ctx, u1, n2); err != nil {
		t.Fatal(err)
	}

	orders, err := s.ListOrders(ctx, u1)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 2 || orders[0].Number != n2 || orders[1].Number != n1 {
		t.Fatalf("orders = %+v", orders)
	}
	if orders[0].Status != StatusNew || orders[0].Accrual != nil {
		t.Fatalf("order = %+v", orders[0])
	}

	if orders, err := s.ListOrders(ctx, u2); err != nil || len(orders) != 0 {
		t.Fatalf("u2 orders = %+v err=%v", orders, err)
	}
}

func TestAddOrderConcurrent(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	u1, u2 := newUser(t, s), newUser(t, s)

	for round := 0; round < 20; round++ {
		number := fmt.Sprintf("%d%02d", time.Now().UnixNano(), round)
		start := make(chan struct{})
		var wg sync.WaitGroup
		errs := make([]error, 2)
		created := make([]bool, 2)
		for i, u := range []int64{u1, u2} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				created[i], errs[i] = s.AddOrder(ctx, u, number)
			}()
		}
		close(start)
		wg.Wait()

		won, lost := 0, 0
		for i := range errs {
			switch {
			case errs[i] == nil && created[i]:
				won++
			case errors.Is(errs[i], ErrOrderOwnedByAnother):
				lost++
			default:
				t.Fatalf("round %d: unexpected result %d: created=%v err=%v", round, i, created[i], errs[i])
			}
		}
		if won != 1 || lost != 1 {
			t.Fatalf("round %d: won=%d lost=%d", round, won, lost)
		}
	}
}
