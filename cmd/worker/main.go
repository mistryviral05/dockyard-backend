package main

import (
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mistryviral05/dockyard/internal/config"
	postgres "github.com/mistryviral05/dockyard/internal/http/database"
	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
	"github.com/mistryviral05/dockyard/internal/logger"
	"github.com/mistryviral05/dockyard/internal/queue"
)

func main() {

	cfg := config.MustLoad()
	logger := logger.New()

	// TODO : Connect the db
	db, err := postgres.New(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	logger.Info("Database connected",
		slog.String("host", cfg.Database.Host),
		slog.String("db", cfg.Database.Name),
	)
	deploymentRepository := deployment.NewRepository(db)
	service := queue.NewService(deploymentRepository, logger)
	handlers := queue.NewHandlers(logger, service)
	srv := queue.NewServer(&cfg.Redis, logger, handlers)

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
