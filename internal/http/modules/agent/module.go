package agent

import (
	"database/sql"
	"log/slog"
)

func New(db *sql.DB, logger *slog.Logger, dns DNSProvider) *Handler {
	repo := NewRepository(db)
	service := NewService(repo, db, dns)
	return NewHandler(service, logger)
}
