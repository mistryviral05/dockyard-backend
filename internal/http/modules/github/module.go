package github

import (
	"database/sql"
	"log/slog"

	"github.com/mistryviral05/dockyard/internal/config"
)

func New(db *sql.DB,config *config.Config,logger *slog.Logger) *Handler {
	repo := NewRepository(db)
	service := NewService(db,config,repo)
	handler := NewHandler(service, logger)
	return handler
}
