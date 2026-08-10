package agent

import (
	"database/sql"
	"log/slog"
)

func New(db *sql.DB, logger *slog.Logger) *Handler {
	repo := NewRepository(db)
	service := NewService(repo, db)
	return NewHandler(service, logger)
}
