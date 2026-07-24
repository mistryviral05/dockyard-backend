package user

import (
	"context"
	"database/sql"

	"github.com/mistryviral05/dockyard/internal/http/modules/workspace"
)

type Service struct {
	db     *sql.DB
	repo   Repository
	wsRepo workspace.Repository
}

func NewService(db *sql.DB, repo Repository, wsRepo workspace.Repository) *Service {
	return &Service{
		db:     db,
		repo:   repo,
		wsRepo: wsRepo,
	}

}

// func (s *Service) createUser(ctx context.Context, dto *CreateUserDto) error {
// 	return s.repo.Create(ctx, dto)
// }

func (s *Service) CreateUserWithWorkspace(ctx context.Context, dto *CreateUserDto) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op after Commit; also fires on panic

	// rebind BOTH repos onto the same transaction
	userRepo := s.repo.WithTx(tx)
	wsRepo := s.wsRepo.WithTx(tx)

	userID, created, err := userRepo.Create(ctx, dto)
	if err != nil {
		return err
	}
	if created {
		if _, err := wsRepo.CreateDefault(ctx, userID); err != nil {
			return err // rolls back the user insert too
		}
	}
	return tx.Commit()
}
