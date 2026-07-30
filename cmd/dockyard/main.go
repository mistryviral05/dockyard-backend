package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/mistryviral05/dockyard/internal/config"
	postgres "github.com/mistryviral05/dockyard/internal/http/database"
	"github.com/mistryviral05/dockyard/internal/http/middleware"
	"github.com/mistryviral05/dockyard/internal/http/modules/user"
	"github.com/mistryviral05/dockyard/internal/http/modules/workspace"
	"github.com/mistryviral05/dockyard/internal/http/router"
	"github.com/mistryviral05/dockyard/internal/logger"
)

func main() {

	// Each operation requires a context.Context as the first argument
	ctx := context.Background()
	//load the config
	cfg := config.MustLoad()
	logger := logger.New()

	clerk.SetKey(cfg.Clerk.SecretKey)

	// setup database
	db, err := postgres.New(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	logger.Info("Database connected",
		slog.String("host", cfg.Database.Host),
		slog.String("db", cfg.Database.Name),
	)
	// add imports:
	//   "github.com/mistryviral05/dockyard/internal/http/middleware"
	//   "github.com/mistryviral05/dockyard/internal/http/modules/workspace"

	wsModule := workspace.New(db, logger)
	userModule := user.New(db, logger, cfg, wsModule.Repo)
	authMW := middleware.RequireAuth()

	modules := router.Modules{
		User:      userModule,
		Workspace: wsModule.Handler,
		Auth:      authMW,
	}
	r := router.New(modules)

	server := http.Server{
		Addr:    cfg.Addr,
		Handler: middleware.Recover(logger)(r),
	}
	//start server
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		logger.Info("Server started",
			slog.String("address", cfg.Addr),
		)
		err := server.ListenAndServe()

		if err != nil {
			log.Fatalf("Failed to start server : %s", err.Error())
		}
	}()
	<-done

	logger.Info("Server is shutting down...")
	ctx, cancle := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancle()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("failed to shutdown server : ", slog.String("error : ", err.Error()))
	}
	logger.Info("Server stopped")

}
