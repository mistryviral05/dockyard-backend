package agent

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
	postgres "github.com/mistryviral05/dockyard/internal/http/database"
)

const (
	StatusPending         = "pending"
	TokenTypeRegistration = "registration"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrAgentNameConflict = errors.New("an agent with this name already exists")
)

type Repository interface {
	Create(ctx context.Context, dto *CreateAgentDto) (*Agent, error)
	WithTx(db postgres.DBTX) Repository
}

type repository struct {
	db postgres.DBTX
}

func (r *repository) WithTx(db postgres.DBTX) Repository {
	return &repository{db: db}
}

func NewRepository(db postgres.DBTX) Repository {
	return &repository{db: db}
}


func (r *repository) Create(ctx context.Context, dto *CreateAgentDto) (*Agent, error) {
	var a Agent

	err := r.db.QueryRowContext(ctx, `
		INSERT INTO agents (id, user_id, agent_name, port, status, temp_token, token_exp)
		SELECT $1, u.id, $3, $4, 'pending', $5, $6
		FROM users u
		WHERE u.clerk_id = $2
		RETURNING id, user_id, agent_name, port, status, token_exp, created_at
	`, dto.AgentId, dto.ClerkId, dto.Name, dto.Port, dto.TokenHash, dto.TokenExp).
		Scan(&a.Id, &a.UserId, &a.AgentName, &a.Port, &a.Status, &a.TokenExp, &a.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, ErrAgentNameConflict
		}
		return nil, err
	}

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_tokens (id, agent_id, token, type, expires_at)
		VALUES ($1, $2, $3, 'registration', $4)
	`, dto.TokenId, a.Id, dto.TokenHash, dto.TokenExp); err != nil {
		return nil, err
	}

	return &a, nil
}
