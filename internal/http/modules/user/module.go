package user

import (
	"database/sql"
	"log/slog"

	"github.com/mistryviral05/dockyard/internal/config"
)

func New(db *sql.DB, logger *slog.Logger, cfg *config.Config) *Handler {
	repo := newRepository(db)
	service := newService(repo)
	handler := newHandler(service, logger, cfg)
	return handler
}
