package github

import (
	"database/sql"
	"log/slog"

	"github.com/mistryviral05/dockyard/internal/config"
	"github.com/mistryviral05/dockyard/internal/http/utils/signer"
)

func New(db *sql.DB, cfg *config.Config, logger *slog.Logger) *Handler {
	sign := signer.New(cfg.Github.StateSecret)
	repo := NewRepository(db)
	service := NewService(db, cfg, repo, sign)
	handler := NewHandler(service, logger)
	return handler
}
