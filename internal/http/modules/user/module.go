package user

import (
	"database/sql"
	"log/slog"

	"github.com/mistryviral05/dockyard/internal/config"
	"github.com/mistryviral05/dockyard/internal/http/modules/workspace"
)

func New(db *sql.DB, logger *slog.Logger, cfg *config.Config, wsRepo workspace.Repository) *Handler {
	repo := NewRepository(db)
	service := NewService(db, repo, wsRepo)
	handler := NewHandler(service, logger, cfg)
	return handler
}
