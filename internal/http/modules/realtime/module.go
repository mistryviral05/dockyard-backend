package realtime

import (
	"database/sql"
	"log/slog"

	"github.com/mistryviral05/dockyard/internal/config"
	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
	"github.com/mistryviral05/dockyard/internal/http/utils/signer"
)

func New(db *sql.DB, cfg *config.Config, logger *slog.Logger) *Handler {
	// secret centrifugo.json ke "token_hmac_secret_key" jaisa hi hona chahiye,
	// warna Centrifugo humara token verify nahi kar payega
	sign := signer.New(cfg.Centrifugo.TokenSecret)
	repo := deployment.NewRepository(db)
	service := NewService(&cfg.Centrifugo, sign, repo, logger)

	return NewHandler(service, logger)
}
