package accrual

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/storage"
)

type fakeStore struct {
	mu       sync.Mutex
	pending  []string
	statuses map[string]string
	accruals map[string]*int64
	credited map[string]int64
	calls    map[string]int
}

func newFakeStore(pending ...string) *fakeStore {
	return &fakeStore{
		pending:  pending,
		statuses: map[string]string{},
		accruals: map[string]*int64{},
		credited: map[string]int64{},
		calls:    map[string]int{},
	}
}

func (f *fakeStore) ListPending(ctx context.Context, after string, limit int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, n := range f.pending {
		s := f.statuses[n]
		if n > after && s != storage.StatusProcessed && s != storage.StatusInvalid {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) ApplyAccrual(ctx context.Context, number, status string, accrual *int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[number]++
	if s := f.statuses[number]; s == storage.StatusProcessed || s == storage.StatusInvalid {
		return nil
	}
	f.statuses[number] = status
	f.accruals[number] = accrual
	if status == storage.StatusProcessed && accrual != nil {
		f.credited[number] += *accrual
	}
	return nil
}

func (f *fakeStore) status(n string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statuses[n]
}

type fakeSource struct {
	mu    sync.Mutex
	fn    func(number string, call int) (Result, error)
	calls map[string]int
	times []time.Time
}

func (s *fakeSource) GetOrder(ctx context.Context, number string) (Result, error) {
	s.mu.Lock()
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[number]++
	call := s.calls[number]
	s.times = append(s.times, time.Now())
	s.mu.Unlock()
	return s.fn(number, call)
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func startWorker(t *testing.T, src Source, st Store) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	w := NewWorker(src, st, quiet(), 5*time.Millisecond, 3)
	go func() {
		defer close(done)
		w.Run(ctx)
	}()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("worker did not stop")
			}
		})
	}
	t.Cleanup(stop)
	return stop
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func ptr(v float64) *float64 { return &v }

func TestWorkerLifecycle(t *testing.T) {
	st := newFakeStore("1", "2", "3", "4", "5")
	src := &fakeSource{fn: func(n string, call int) (Result, error) {
		switch n {
		case "1":
			if call == 1 {
				return Result{Status: "REGISTERED"}, nil
			}
			if call == 2 {
				return Result{Status: "PROCESSING"}, nil
			}
			return Result{Status: "PROCESSED", Accrual: ptr(500.55)}, nil
		case "2":
			return Result{Status: "INVALID"}, nil
		case "3":
			return Result{}, ErrNotRegistered
		case "4":
			return Result{}, errors.New("boom")
		default:
			return Result{Status: "WEIRD"}, nil
		}
	}}
	stop := startWorker(t, src, st)

	waitFor(t, func() bool {
		return st.status("1") == storage.StatusProcessed && st.status("2") == storage.StatusInvalid
	})
	time.Sleep(50 * time.Millisecond)
	stop()

	st.mu.Lock()
	defer st.mu.Unlock()
	if st.credited["1"] != 50055 || st.calls["1"] != 3 {
		t.Fatalf("order 1: credited=%d calls=%d", st.credited["1"], st.calls["1"])
	}
	if st.credited["2"] != 0 || st.accruals["2"] != nil {
		t.Fatalf("invalid order got points: %d", st.credited["2"])
	}
	if st.calls["3"] != 0 || st.calls["4"] != 0 || st.calls["5"] != 0 {
		t.Fatalf("unexpected applies: %v", st.calls)
	}
}

func TestWorkerRateLimitPausesAll(t *testing.T) {
	st := newFakeStore("1", "2", "3", "4")
	var pausedAt time.Time
	var pausedMu sync.Mutex
	var once sync.Once
	src := &fakeSource{}
	src.fn = func(n string, call int) (Result, error) {
		var first bool
		once.Do(func() { first = true })
		if first {
			time.Sleep(30 * time.Millisecond)
			pausedMu.Lock()
			pausedAt = time.Now()
			pausedMu.Unlock()
			return Result{}, &RateLimitError{RetryAfter: 200 * time.Millisecond}
		}
		return Result{Status: "INVALID"}, nil
	}
	startWorker(t, src, st)

	waitFor(t, func() bool {
		for _, n := range []string{"1", "2", "3", "4"} {
			if st.status(n) != storage.StatusInvalid {
				return false
			}
		}
		return true
	})

	src.mu.Lock()
	defer src.mu.Unlock()
	pausedMu.Lock()
	defer pausedMu.Unlock()
	after := 0
	for _, at := range src.times {
		if !at.After(pausedAt) {
			continue
		}
		after++
		if at.Sub(pausedAt) < 190*time.Millisecond {
			t.Fatalf("request %s after the 429, want a pause of 200ms", at.Sub(pausedAt))
		}
	}
	if after == 0 {
		t.Fatal("no requests after the pause")
	}
}

func TestWorkerStopsOnCancel(t *testing.T) {
	st := newFakeStore("1", "2", "3")
	src := &fakeSource{fn: func(string, int) (Result, error) {
		return Result{}, &RateLimitError{RetryAfter: time.Hour}
	}}
	stop := startWorker(t, src, st)
	waitFor(t, func() bool {
		src.mu.Lock()
		defer src.mu.Unlock()
		return len(src.times) > 0
	})
	stop()

	src.mu.Lock()
	calls := len(src.times)
	src.mu.Unlock()
	time.Sleep(50 * time.Millisecond)
	src.mu.Lock()
	defer src.mu.Unlock()
	if len(src.times) != calls {
		t.Fatalf("requests after stop: %d -> %d", calls, len(src.times))
	}
}

func TestWorkerCoversMoreThanOneBatch(t *testing.T) {
	var numbers []string
	for i := 0; i < batchSize*2+30; i++ {
		numbers = append(numbers, fmt.Sprintf("%06d", i))
	}
	st := newFakeStore(numbers...)
	src := &fakeSource{fn: func(n string, call int) (Result, error) {
		if n < "000100" {
			return Result{}, ErrNotRegistered
		}
		return Result{Status: "INVALID"}, nil
	}}
	startWorker(t, src, st)

	waitFor(t, func() bool { return st.status(numbers[len(numbers)-1]) == storage.StatusInvalid })
}

func TestWorkerProcessedWithoutAccrual(t *testing.T) {
	st := newFakeStore("1")
	src := &fakeSource{fn: func(string, int) (Result, error) { return Result{Status: "PROCESSED"}, nil }}
	startWorker(t, src, st)

	waitFor(t, func() bool { return st.status("1") == storage.StatusProcessed })
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.accruals["1"] != nil || st.credited["1"] != 0 {
		t.Fatalf("accrual = %v credited = %d", st.accruals["1"], st.credited["1"])
	}
}

func TestMapResult(t *testing.T) {
	if _, a, ok := mapResult(Result{Status: "PROCESSED"}); !ok || a != nil {
		t.Fatal("processed without accrual")
	}
	if s, a, ok := mapResult(Result{Status: "PROCESSED", Accrual: ptr(0.29)}); !ok || s != storage.StatusProcessed || *a != 29 {
		t.Fatalf("rounding: %v %v", s, a)
	}
	for _, bad := range []float64{-1, math.NaN(), math.Inf(1)} {
		if _, _, ok := mapResult(Result{Status: "PROCESSED", Accrual: ptr(bad)}); ok {
			t.Fatalf("accrual %v accepted", bad)
		}
	}
	if _, _, ok := mapResult(Result{Status: "?"}); ok {
		t.Fatal("unknown status accepted")
	}
}

func TestNewWorkerMinimumWorkers(t *testing.T) {
	if w := NewWorker(nil, nil, quiet(), time.Second, 0); w.workers != 1 {
		t.Fatalf("workers = %d", w.workers)
	}
}
