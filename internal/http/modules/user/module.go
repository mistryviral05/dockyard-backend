package user

import (
	"database/sql"
	"log/slog"
)

func New(db *sql.DB, logger *slog.Logger) *Handler {
	repo := newRepository(db)
	service := newService(repo)
	handler := newHandler(service, logger)
	return handler
}
