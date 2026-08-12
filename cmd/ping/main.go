package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/mistryviral05/dockyard/internal/config"
	"github.com/mistryviral05/dockyard/internal/logger"
	"github.com/mistryviral05/dockyard/internal/queue"
)

func main() {
	cfg := config.MustLoad()
	log := logger.New()

	c := queue.NewClient(&cfg.Redis, log)
	defer c.Close()

	if err := c.EnqueueDebugPing(context.Background(), "hello from cli"); err != nil {
		log.Error("enqueue failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
