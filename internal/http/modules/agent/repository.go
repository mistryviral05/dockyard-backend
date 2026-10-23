package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	postgres "github.com/mistryviral05/dockyard/internal/http/database"
)

const (
	StatusPending         = "pending"
	StatusActive          = "active"
	TokenTypeRegistration = "registration"
	TokenTypeJWT          = "jwt"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrAgentNameConflict = errors.New("an agent with this name already exists")
	ErrAgentNotFound     = errors.New("agent not found")
	ErrAgentTokenInvalid = errors.New("agent token is invalid, revoked or expired")

	ErrRegistrationTokenInvalid = errors.New("registration token is invalid, used or expired")
	ErrSubdomainConflict        = errors.New("subdomain is already taken")
	ErrPublicIPInvalid          = errors.New("public ip is missing or not a valid address")
)

type Repository interface {
	Create(ctx context.Context, dto *CreateAgentDto) (*Agent, error)
	GetByClerkId(ctx context.Context, clerkID string) ([]*Agent, error)
	GetByID(ctx context.Context, agentID, clerkID string) (*Agent, error)
	GetLatestMetrics(ctx context.Context, agentID, clerkID string) (*AgentMetrics, error)
	GetMetricsSeries(ctx context.Context, agentID string, since time.Time, bucketSeconds, limit int) ([]*MetricPoint, error)
	FindAgentIDByTokenHash(ctx context.Context, tokenHash string) (string, error)
	FindPendingByTempToken(ctx context.Context, tokenHash string) (*Agent, error)
	SubdomainTaken(ctx context.Context, subdomain string) (bool, error)
	CompleteRegistration(ctx context.Context, dto *RegisterDto) error
	InsertToken(ctx context.Context, tokenID, agentID, tokenHash, tokenType string) error
	InsertMetrics(ctx context.Context, metricID string, dto *HeartbeatDto) error
	UpdateHeartbeat(ctx context.Context, dto *HeartbeatDto) error
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

func (r *repository) GetLatestMetrics(ctx context.Context, agentID, clerkID string) (*AgentMetrics, error) {
	owner := "JOIN users u ON u.id = a.user_id"
	where := "WHERE a.id = $1 AND u.clerk_id = $2"
	args := []any{agentID, clerkID}

	if clerkID == "" {
		owner = ""
		where = "WHERE a.id = $1"
		args = []any{agentID}
	}

	query := fmt.Sprintf(`
		SELECT a.id, a.agent_name, a.status, a.last_heartbeat_at, a.public_ip, a.os_info,
		       m.cpu_usage, m.ram_usage, m.disk_usage,
		       m.load_1, m.load_5, m.load_15,
		       m.cpu_cores, m.uptime_seconds, m.recorded_at
		FROM agents a
		%s
		LEFT JOIN LATERAL (
			SELECT am.cpu_usage, am.ram_usage, am.disk_usage,
			       am.load_1, am.load_5, am.load_15,
			       am.cpu_cores, am.uptime_seconds, am.recorded_at
			FROM agent_metrics am
			WHERE am.agent_id = a.id
			ORDER BY am.recorded_at DESC
			LIMIT 1
		) m ON true
		%s
	`, owner, where)

	var m AgentMetrics

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&m.AgentID, &m.AgentName, &m.Status, &m.LastHeartbeatAt, &m.PublicIP, &m.OSInfo,
		&m.CPUUsage, &m.MemoryUsage, &m.DiskUsage,
		&m.Load1, &m.Load5, &m.Load15,
		&m.CPUCores, &m.UptimeSec, &m.RecordedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	return &m, nil
}

func (r *repository) GetMetricsSeries(ctx context.Context, agentID string, since time.Time, bucketSeconds, limit int) ([]*MetricPoint, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT to_timestamp(floor(extract(epoch FROM recorded_at) / $3) * $3) AS bucket,
		       avg(cpu_usage), avg(ram_usage), avg(disk_usage), avg(load_1)
		FROM agent_metrics
		WHERE agent_id = $1 AND recorded_at > $2
		GROUP BY bucket
		ORDER BY bucket
		LIMIT $4
	`, agentID, since, bucketSeconds, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []*MetricPoint{}
	for rows.Next() {
		var p MetricPoint
		if err := rows.Scan(&p.At, &p.CPU, &p.RAM, &p.Disk, &p.Load); err != nil {
			return nil, err
		}
		points = append(points, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return points, nil
}

func (r *repository) FindPendingByTempToken(ctx context.Context, tokenHash string) (*Agent, error) {
	var a Agent

	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, agent_name, port, status, token_exp, created_at
		FROM agents
		WHERE temp_token = $1
		  AND status = 'pending'
		  AND (token_exp IS NULL OR token_exp > now())
	`, tokenHash).Scan(
		&a.Id, &a.UserId, &a.AgentName, &a.Port, &a.Status, &a.TokenExp, &a.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRegistrationTokenInvalid
		}
		return nil, err
	}

	return &a, nil
}

func (r *repository) SubdomainTaken(ctx context.Context, subdomain string) (bool, error) {
	var exists bool

	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM agents WHERE subdomain = $1)
	`, subdomain).Scan(&exists)

	return exists, err
}

func (r *repository) CompleteRegistration(ctx context.Context, dto *RegisterDto) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE agents
		SET subdomain      = $2,
		    public_ip      = $3,
		    os_info        = COALESCE(NULLIF($4, ''), os_info),
		    signing_secret = $5,
		    status         = 'active',
		    registered_at  = now(),
		    temp_token     = NULL,
		    token_exp      = NULL,
		    updated_at     = now()
		WHERE id = $1 AND status = 'pending'
	`, dto.AgentID, dto.Subdomain, dto.PublicIP, dto.OSInfo, dto.SigningSecret)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return ErrSubdomainConflict
		}
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrRegistrationTokenInvalid
	}

	return nil
}

func (r *repository) InsertToken(ctx context.Context, tokenID, agentID, tokenHash, tokenType string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_tokens (id, agent_id, token, type, expires_at)
		VALUES ($1, $2, $3, $4, NULL)
	`, tokenID, agentID, tokenHash, tokenType)

	return err
}

func (r *repository) FindAgentIDByTokenHash(ctx context.Context, tokenHash string) (string, error) {
	var agentID string

	err := r.db.QueryRowContext(ctx, `
		SELECT t.agent_id
		FROM agent_tokens t
		JOIN agents a ON a.id = t.agent_id
		WHERE t.token = $1
		  AND t.revoked_at IS NULL
		  AND (t.expires_at IS NULL OR t.expires_at > now())
	`, tokenHash).Scan(&agentID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrAgentTokenInvalid
		}
		return "", err
	}

	return agentID, nil
}

func (r *repository) InsertMetrics(ctx context.Context, metricID string, dto *HeartbeatDto) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_metrics (
			id, agent_id, cpu_usage, ram_usage, disk_usage,
			load_1, load_5, load_15, cpu_cores, uptime_seconds, recorded_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		metricID, dto.AgentID, dto.CPUUsage, dto.MemoryUsage, dto.DiskUsage,
		dto.Load1, dto.Load5, dto.Load15, dto.CPUCores, dto.UptimeSec, dto.RecordedAt,
	)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23503" {
			return ErrAgentNotFound
		}
		return err
	}

	return nil
}

func (r *repository) UpdateHeartbeat(ctx context.Context, dto *HeartbeatDto) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE agents
		SET last_heartbeat_at = $2,
		    status            = 'active',
		    public_ip         = COALESCE(NULLIF($3, ''), public_ip),
		    os_info           = COALESCE(NULLIF($4, ''), os_info),
		    updated_at        = now()
		WHERE id = $1
	`, dto.AgentID, dto.RecordedAt, dto.PublicIP, dto.OSInfo)

	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrAgentNotFound
	}

	return nil
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
