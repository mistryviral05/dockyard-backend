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
	"github.com/mistryviral05/dockyard/internal/http/modules/agent"
	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
	"github.com/mistryviral05/dockyard/internal/http/modules/github"
	"github.com/mistryviral05/dockyard/internal/http/modules/project"
	"github.com/mistryviral05/dockyard/internal/http/modules/realtime"
	"github.com/mistryviral05/dockyard/internal/http/modules/user"
	"github.com/mistryviral05/dockyard/internal/http/modules/workspace"
	"github.com/mistryviral05/dockyard/internal/http/router"
	"github.com/mistryviral05/dockyard/internal/logger"
	"github.com/mistryviral05/dockyard/internal/queue"
)

func main() {

	// Each operation requires a context.Context as the first argument
	ctx := context.Background()
	//load the config
	cfg := config.MustLoad()
	logger := logger.New(logger.Options{
		Env:    cfg.Env,
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})

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

	queueClient := queue.NewClient(&cfg.Redis, logger)
	defer queueClient.Close()
	logger.Info("Queue connected",
		slog.String("redis", cfg.Redis.Addr),
	)

	wsModule := workspace.New(db, logger)
	userModule := user.New(db, logger, cfg, wsModule.Repo)
	githubModule := github.New(db, cfg, logger)
	agentModule := agent.New(db, logger)
	projectModule := project.New(db, logger)
	deploymentModule := deployment.New(db, logger, queueClient)
	realtimeModule := realtime.New(db, cfg, logger)
	authMW := middleware.RequireAuth(logger)

	modules := router.Modules{
		User:       userModule,
		Workspace:  wsModule.Handler,
		Github:     githubModule,
		Agent:      agentModule,
		Project:    projectModule,
		Deployment: deploymentModule,
		Realtime:   realtimeModule,
		Auth:       authMW,
	}
	r := router.New(modules)

	handler := middleware.CORS(cfg.CORS.AllowedOrigins)(middleware.RequestLogger(logger)(middleware.Recover(logger)(r)))

	server := http.Server{
		Addr:         cfg.HTTPServer.Addr,
		Handler:      handler,
		ReadTimeout:  time.Duration(cfg.HTTPServer.ReadTimeout),
		WriteTimeout: time.Duration(cfg.HTTPServer.WriteTimeout),
		IdleTimeout:  time.Duration(cfg.HTTPServer.IdleTimeout),
	}
	//start server
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		logger.Info("Server started",
			slog.String("address", cfg.HTTPServer.Addr),
		)
		err := server.ListenAndServe()

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server : %s", err.Error())
		}
	}()
	<-done

	logger.Info("Server is gracefull shutting down...")
	ctx, cancle := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancle()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("failed to shutdown server : ", slog.String("error : ", err.Error()))
	}
	logger.Info("Server stopped")

}
