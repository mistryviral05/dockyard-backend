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
	ErrAgentNotFound     = errors.New("agent not found")
)

type Repository interface {
	Create(ctx context.Context, dto *CreateAgentDto) (*Agent, error)
	GetByClerkId(ctx context.Context, clerkID string) ([]*Agent, error)
	GetByID(ctx context.Context, agentID, clerkID string) (*Agent, error)
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

	return &a, nil
}

// GetByID ek agent lautata hai, sirf tab jab wo isi clerk user ka ho.
// temp_token yahan kabhi select nahi hota.
func (r *repository) GetByID(ctx context.Context, agentID, clerkID string) (*Agent, error) {
	var a Agent

	err := r.db.QueryRowContext(ctx, `
		SELECT a.id, a.agent_name, a.status, a.subdomain, a.port,
		       a.last_heartbeat_at, a.created_at,
		       (SELECT COUNT(*) FROM project_agent pa WHERE pa.agent_id = a.id)
		FROM agents a
		JOIN users u ON u.id = a.user_id
		WHERE a.id = $1 AND u.clerk_id = $2
	`, agentID, clerkID).Scan(
		&a.Id, &a.AgentName, &a.Status, &a.Subdomain, &a.Port,
		&a.LastHeartbeatAt, &a.CreatedAt, &a.ProjectCount,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	return &a, nil
}

func (r *repository) GetByClerkId(ctx context.Context, clerkID string) ([]*Agent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id, a.agent_name, a.status, a.subdomain, a.port
		FROM agents a
		JOIN users u ON u.id = a.user_id
		WHERE u.clerk_id = $1
		ORDER BY a.created_at DESC
	`, clerkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := []*Agent{}
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.Id, &a.AgentName, &a.Status, &a.Subdomain, &a.Port); err != nil {
			return nil, err
		}
		agents = append(agents, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return agents, nil
}
