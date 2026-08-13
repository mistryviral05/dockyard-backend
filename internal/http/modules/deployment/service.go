package deployment

import (
	"database/sql"
	"log/slog"
)

type Service struct {
	db     *sql.DB
	logger *slog.Logger
	repo   Repository
}

func NewService(db *sql.DB, logger *slog.Logger, repo Repository) *Service {
	return &Service{
		db:     db,
		logger: logger,
		repo:   repo,
	}
}
