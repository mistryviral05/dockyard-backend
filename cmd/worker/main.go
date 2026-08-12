package main

import (
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mistryviral05/dockyard/internal/config"
	"github.com/mistryviral05/dockyard/internal/logger"
	"github.com/mistryviral05/dockyard/internal/queue"
)

func main() {

	cfg := config.MustLoad()
	logger := logger.New()

	// TODO : Connect the db

	handlers := queue.NewHandlers(logger)
	srv := queue.NewServer(cfg.Redis, logger, handlers)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("worker started", slog.String("redis", cfg.Redis.Addr))
		if err := srv.Run(); err != nil {
			log.Fatal("Failed to run worker: ", err)
			os.Exit(1)
		}
	}()

	<-done

	logger.Info("worker shutting down...")
	srv.Shutdown()
	logger.Info("worker stopped")

}
