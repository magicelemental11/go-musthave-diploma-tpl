package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/accrual"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/config"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/middleware"
	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/storage"
)

const (
	pollInterval = time.Second
	pollWorkers  = 5
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(log)
	if err := run(log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", cfg.DatabaseURI)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := db.PingContext(ctx); err != nil {
		return err
	}
	if err := storage.Migrate(cfg.DatabaseURI); err != nil {
		return err
	}

	store := storage.New(db)
	h := handler.New(store, cfg.SecretKey)

	worker := accrual.NewWorker(accrual.NewClient(cfg.AccrualAddress), store, log, pollInterval, pollWorkers)
	workerCtx, stopWorker := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		worker.Run(workerCtx)
	}()
	defer wg.Wait()
	defer stopWorker()

	srv := &http.Server{
		Addr:              cfg.RunAddress,
		Handler:           middleware.Logging(log)(middleware.Gzip(h.Router())),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "address", cfg.RunAddress)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
