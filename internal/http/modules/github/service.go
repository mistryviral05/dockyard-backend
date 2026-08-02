package github

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mistryviral05/dockyard/internal/config"
	"github.com/mistryviral05/dockyard/internal/http/utils/signer"
)

type Service struct {
	db     *sql.DB
	cfg    *config.Config
	signer *signer.Signer
	repo   Repository
}

func NewService(db *sql.DB, cfg *config.Config, repo Repository) *Service {
	return &Service{
		db:   db,
		cfg:  cfg,
		repo: repo,
	}
}

type StateClaims struct {
	ClerkId string `json:"clerk_id"`
}

func (s *Service) CreateInstallation(clerkId string) (string, error) {
	claims := StateClaims{
		ClerkId: clerkId,
	}
	token, err := s.signer.Sign(claims, time.Now().Add(10*time.Minute))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://github.com/apps/%s/installations/new?state=%s", s.cfg.Github.AppSlug, token), nil
}
