package accrual

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/storage"
)

const batchSize = 100

// Source is the accrual system as seen by the Worker.
type Source interface {
	GetOrder(ctx context.Context, number string) (Result, error)
}

// Store is the order storage as seen by the Worker.
type Store interface {
	ListPending(ctx context.Context, after string, limit int) ([]string, error)
	ApplyAccrual(ctx context.Context, number, status string, accrual *int64) error
}

// Worker polls the accrual system for pending orders and credits points.
type Worker struct {
	source   Source
	store    Store
	log      *slog.Logger
	interval time.Duration
	workers  int

	cursor string

	mu    sync.Mutex
	until time.Time
}

// NewWorker returns a Worker that every interval polls pending orders with
// the given number of concurrent requests.
func NewWorker(source Source, store Store, log *slog.Logger, interval time.Duration, workers int) *Worker {
	if workers < 1 {
		workers = 1
	}
	return &Worker{source: source, store: store, log: log, interval: interval, workers: workers}
}

// Run polls until ctx is cancelled and returns after all in-flight requests
// have finished.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Worker) poll(ctx context.Context) {
	numbers, err := w.store.ListPending(ctx, w.cursor, batchSize)
	if err != nil {
		if ctx.Err() == nil {
			w.log.Error("list pending orders", "error", err)
		}
		return
	}
	if len(numbers) < batchSize {
		w.cursor = ""
	} else {
		w.cursor = numbers[len(numbers)-1]
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	for range w.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for number := range jobs {
				w.process(ctx, number)
			}
		}()
	}
	for _, number := range numbers {
		select {
		case jobs <- number:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	wg.Wait()
}

func (w *Worker) process(ctx context.Context, number string) {
	if !w.waitPause(ctx) {
		return
	}
	res, err := w.source.GetOrder(ctx, number)
	if err != nil {
		var rl *RateLimitError
		switch {
		case errors.As(err, &rl):
			w.pauseFor(rl.RetryAfter)
			w.log.Warn("accrual system rate limit", "retry_after", rl.RetryAfter)
		case errors.Is(err, ErrNotRegistered), ctx.Err() != nil:
		default:
			w.log.Error("get accrual", "order", number, "error", err)
		}
		return
	}

	status, accrual, ok := mapResult(res)
	if !ok {
		w.log.Error("unknown accrual status", "order", number, "status", res.Status)
		return
	}
	if err := w.store.ApplyAccrual(ctx, number, status, accrual); err != nil && ctx.Err() == nil {
		w.log.Error("apply accrual", "order", number, "error", err)
	}
}

func mapResult(res Result) (status string, accrual *int64, ok bool) {
	switch res.Status {
	case "REGISTERED", "PROCESSING":
		return storage.StatusProcessing, nil, true
	case "INVALID":
		return storage.StatusInvalid, nil, true
	case "PROCESSED":
		if res.Accrual == nil {
			return storage.StatusProcessed, nil, true
		}
		if f := *res.Accrual; f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
			return "", nil, false
		}
		v := int64(math.Round(*res.Accrual * 100))
		return storage.StatusProcessed, &v, true
	default:
		return "", nil, false
	}
}

func (w *Worker) pauseFor(d time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if until := time.Now().Add(d); until.After(w.until) {
		w.until = until
	}
}

func (w *Worker) waitPause(ctx context.Context) bool {
	for {
		w.mu.Lock()
		d := time.Until(w.until)
		w.mu.Unlock()
		if d <= 0 {
			return ctx.Err() == nil
		}
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return false
		case <-t.C:
		}
	}
}
