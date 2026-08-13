package deployment

import (
	"database/sql"
	"log/slog"
)

func New(db *sql.DB, logger *slog.Logger) *Handler {
	repo := NewRepository(db)
	service := NewService(db, logger, repo)
	return NewHandler(service, logger)
}
