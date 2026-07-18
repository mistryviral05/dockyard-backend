package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/mistryviral05/dockyard/internal/config"
)

func main() {

	// Each operation requires a context.Context as the first argument
	ctx := context.Background()
	//load the config
	cfg := config.MustLoad()

	clerk.SetKey(cfg.Clerk.SecretKey)

	// setup database
	// setup router
	router := http.NewServeMux()

	router.HandleFunc("/", publicRoute)
	protectedHandler := http.HandlerFunc(protectedRoute)
	router.Handle(
		"/protected",
		clerkhttp.WithHeaderAuthorization()(protectedHandler),
	)

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

func publicRoute(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"access": "public"}`))
}

func protectedRoute(w http.ResponseWriter, r *http.Request) {
	claims, ok := clerk.SessionClaimsFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"access": "unauthorized"}`))
		return
	}

	usr, err := user.Get(r.Context(), claims.Subject)
	if err != nil {
		// handle the error
	}
	fmt.Fprintf(w, `{"user_id": "%s", "user_banned": "%t"}`, usr.ID, usr.Banned)
}
