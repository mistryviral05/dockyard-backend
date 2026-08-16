package deployment

import (
	"database/sql"
	"log/slog"
)

func New(db *sql.DB, logger *slog.Logger, queue Enqueuer) *Handler {
	repo := NewRepository(db)
	service := NewService(db, logger, repo, queue)
	return NewHandler(service, logger)
}
