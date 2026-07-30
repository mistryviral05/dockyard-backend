package workspace

import (
	"context"
	"database/sql"
)

type Service struct {
	db   *sql.DB
	repo Repository
}

func NewService(db *sql.DB, repo Repository) *Service {
	return &Service{
		db:   db,
		repo: repo,
	}
}

// Standalone (no-tx) use case.
func (s *Service) Create(ctx context.Context, dto *CreateWorkspaceDto) (string, error) {
	return s.repo.Create(ctx, dto)
}

func (s *Service) Get(ctx context.Context, userId string) ([]*Workspace, error) {
	return s.repo.GetByUserId(ctx, userId)
}
