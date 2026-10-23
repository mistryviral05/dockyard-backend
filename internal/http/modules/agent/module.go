package agent

import (
	"database/sql"
	"log/slog"

	"github.com/mistryviral05/dockyard/internal/http/utils/centrifugo"
)

func New(
	db *sql.DB,
	logger *slog.Logger,
	dns DNSProvider,
	install InstallConfig,
	deployments DeploymentUpdater,
	realtime *centrifugo.Client,
) *Handler {
	repo := NewRepository(db)
	service := NewService(repo, db, dns, deployments)
	SetServerURL(install.ServerURL)
	return NewHandler(service, logger, install, realtime)
}
