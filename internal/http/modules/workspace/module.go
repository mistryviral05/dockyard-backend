package workspace

import (
	"database/sql"
	"log/slog"
)

type Module struct {
	Service *Service
	Repo    Repository
	Handler *Handler
}

func New(db *sql.DB, logger *slog.Logger) *Module {
	repo := NewRepository(db)
	service := NewService(db, repo)
	handler := NewHandler(service, logger)
	return &Module{
		Service: service,
		Repo:    repo,
		Handler: handler,
	}
}
