package deployment

import (
	"database/sql"
	"log/slog"
)

// Module exposes the service alongside the handler because deployments are
// also started from outside HTTP — the github webhook calls CreateFromPush.
type Module struct {
	Service *Service
	Repo    Repository
	Handler *Handler
}

func New(db *sql.DB, logger *slog.Logger, queue Enqueuer) *Module {
	repo := NewRepository(db)
	service := NewService(db, logger, repo, queue)
	return &Module{
		Service: service,
		Repo:    repo,
		Handler: NewHandler(service, logger),
	}
}
