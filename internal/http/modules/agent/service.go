package agent

import (
	"context"
	"database/sql"
	"time"

	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

const registrationTokenTTL = 15 * time.Minute

type Service struct {
	db   *sql.DB
	repo Repository
}

func NewService(repo Repository, db *sql.DB) *Service {

	return &Service{repo: repo, db: db}
}

func (s *Service) Create(ctx context.Context, clerkID, name, port string) (*CreateAgentResponse, error) {
	plainToken, tokenHash, err := GenerateToken()
	if err != nil {
		return nil, err
	}

	dto := &CreateAgentDto{
		ClerkId:   clerkID,
		AgentId:   id.AgentID(),
		TokenId:   id.AgentID(),
		Name:      name,
		Port:      normalizePort(port),
		TokenHash: tokenHash,
		TokenExp:  time.Now().Add(registrationTokenTTL),
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	created, err := s.repo.WithTx(tx).Create(ctx, dto)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &CreateAgentResponse{
		Id:             created.Id,
		Name:           created.AgentName.String,
		Status:         created.Status,
		Port:           created.Port,
		CreatedAt:      created.CreatedAt,
		Token:          plainToken, // last time this value exists server-side
		TokenExpiresAt: created.TokenExp.Time,
		TokenExpiresIn: int64(time.Until(created.TokenExp.Time).Seconds()),
		RunCommand:     BuildRunCommand(plainToken, created.Port),
	}, nil
}
