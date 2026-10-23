package agent

import (
	"database/sql"
	"log/slog"
)

func New(db *sql.DB, logger *slog.Logger, dns DNSProvider, install InstallConfig) *Handler {
	repo := NewRepository(db)
	service := NewService(repo, db, dns)
	SetServerURL(install.ServerURL)
	return NewHandler(service, logger, install)
}
