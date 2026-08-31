package agentd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)


var Version = "dev"


const shutdownGrace = 30 * time.Second

type Agent struct {
	cfg *Config
	log *slog.Logger
	srv *http.Server
	wg sync.WaitGroup
}

func New(cfg *Config, log *slog.Logger) *Agent {
	a := &Agent{cfg: cfg, log: log}

	a.srv = &http.Server{
		Addr:         cfg.Addr,
		Handler:      a.routes(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return a
}

func (a *Agent) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", a.handleHealthz)

	return mux
}

func (a *Agent) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": Version,
	})
}


func (a *Agent) background(fn func()) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() {
			if rec := recover(); rec != nil {
				a.log.Error("background task panicked", slog.Any("panic", rec))
			}
		}()
		fn()
	}()
}


func (a *Agent) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		a.log.Info("agent listening", slog.String("addr", a.cfg.Addr))
		if err := a.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return a.shutdown()
	}
}


func (a *Agent) shutdown() error {
	a.log.Info("agent shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()

	var shutdownErr error
	if err := a.srv.Shutdown(ctx); err != nil {
		a.log.Error("http shutdown failed", slog.String("error", err.Error()))
		shutdownErr = err
	}

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		a.log.Info("agent stopped")
	case <-ctx.Done():

		a.log.Warn("background tasks did not finish in time",
			slog.Duration("grace", shutdownGrace),
		)
	}

	return shutdownErr
}


func StartAgent(cfg *Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return New(cfg, log).Run(ctx)
}
