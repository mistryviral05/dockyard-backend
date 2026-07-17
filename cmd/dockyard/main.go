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

	"github.com/mistryviral05/dockyard/internal/config"
)

func main() {

	//load the config
	cfg := config.MustLoad()
	// setup database
	// setup router
	router := http.NewServeMux()

	server := http.Server{
		Addr:    cfg.Addr,
		Handler: router,
	}
	//start server
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		slog.Info("server is started", slog.String("on", cfg.Addr))
		err := server.ListenAndServe()

		if err != nil {
			log.Fatalf("Faild to start server : %s", err.Error())
		}
	}()
	<-done

	slog.Info("server is shutting down...")

	ctx, cancle := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancle()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("failed to shutdown server : ", slog.String("error : ", err.Error()))
	}

}
