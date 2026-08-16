package deployment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
	postgres "github.com/mistryviral05/dockyard/internal/http/database"
)

var updatableColumns = []string{
	"agent_id",
	"image_tag",
	"container_id",
	"error_message",
}

var (
	ErrProjectNotFound    = errors.New("project not found")
	ErrDeploymentNotFound = errors.New("deployment not found")
)

const (
	ErrInvalidTransition = "invalid transition"
)

type Repository interface {
	Create(ctx context.Context, dto *CreateDeploymentDto) error
	Transition(ctx context.Context, id, to string, extra map[string]interface{}) error
	ResolveProjectForUser(ctx context.Context, projectID, clerkID string) (*ProjectRef, error)
	HasActiveDeployment(ctx context.Context, projectID string) (bool, error)
	GetByID(ctx context.Context, id, clerkID string) (*Deployment, error)
	ListByProject(ctx context.Context, projectID, clerkID, cursor string, limit int) ([]*Deployment, error)
	WithTx(db postgres.DBTX) Repository
}

type repository struct {
	db postgres.DBTX // *sql.DB in normal use, *sql.Tx inside a transaction
}

func (r *repository) WithTx(db postgres.DBTX) Repository {
	return &repository{db: db}
}

func NewRepository(db postgres.DBTX) Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) Transition(ctx context.Context, id, to string, extra map[string]interface{}) error {

	from, ok := PredecessorsOf(to)

	if !ok {
		return errors.New(ErrInvalidTransition)
	}

	sets := []string{"status = $1", "updated_at = NOW()"}
	args := []interface{}{to}

	switch {
	case to == StatusBuilding:
		sets = append(sets, "started_at = COALESCE(started_at, NOW())")
	case IsTerminal(to):
		sets = append(sets, "completed_at = NOW()")
	}

	for _, col := range updatableColumns {
		val, ok := extra[col]
		if !ok {
			continue
		}
		args = append(args, val)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}

	args = append(args, id)
	idPos := len(args)

	args = append(args, pq.Array(from))
	fromPos := len(args)

	q := fmt.Sprintf(`UPDATE deployments
		  SET %s
		  WHERE id = $%d AND status = ANY($%d)
		  RETURNING id`, strings.Join(sets, ", "), idPos, fromPos)

	var returned string
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&returned)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("failed to transition deployment: %w", err)
	}

	return nil
}

const selectColumns = `
	d.id, d.project_id, d.agent_id, d.triggered_by, d.trigger_source,
	d.commit_sha, d.commit_message, d.branch, d.status, d.image_tag,
	d.container_id, d.error_message, d.started_at, d.completed_at,
	d.created_at, d.updated_at`

const ownershipJoin = `
	FROM deployments d
	JOIN projects p   ON p.id = d.project_id
	JOIN workspaces w ON w.id = p.wsp_id
	JOIN users u      ON u.id = w.user_id`

func (r *repository) Create(ctx context.Context, dto *CreateDeploymentDto) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deployments (
			id, project_id, agent_id, triggered_by, trigger_source,
			branch, commit_sha, commit_message, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')
	`, dto.DeploymentId, dto.ProjectId, dto.AgentId, dto.TriggeredBy,
		dto.TriggerSource, dto.Branch, dto.CommitSha, dto.CommitMessage)

	return err
}

func (r *repository) ResolveProjectForUser(ctx context.Context, projectID, clerkID string) (*ProjectRef, error) {
	var ref ProjectRef

	err := r.db.QueryRowContext(ctx, `
		SELECT p.id, w.user_id, p.default_branch, pa.agent_id
		FROM projects p
		JOIN workspaces w ON w.id = p.wsp_id
		JOIN users u      ON u.id = w.user_id
		LEFT JOIN project_agent pa ON pa.project_id = p.id
		WHERE p.id = $1 AND u.clerk_id = $2
		ORDER BY pa.created_at DESC
		LIMIT 1
	`, projectID, clerkID).Scan(&ref.ProjectId, &ref.UserId, &ref.DefaultBranch, &ref.AgentId)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}

	return &ref, nil
}

// HasActiveDeployment batata hai ki is project pe koi deployment abhi chal rahi hai.
func (r *repository) HasActiveDeployment(ctx context.Context, projectID string) (bool, error) {
	var exists bool

	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM deployments
			WHERE project_id = $1 AND status = ANY($2)
		)
	`, projectID, pq.Array(ActiveStatuses())).Scan(&exists)

	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetByID ek deployment lautata hai, sirf tab jab wo is clerk user ka ho.
func (r *repository) GetByID(ctx context.Context, id, clerkID string) (*Deployment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectColumns+ownershipJoin+`
		WHERE d.id = $1 AND u.clerk_id = $2
	`, id, clerkID)

	d, err := scanDeployment(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDeploymentNotFound
		}
		return nil, err
	}

	return d, nil
}

func (r *repository) ListByProject(ctx context.Context, projectID, clerkID, cursor string, limit int) ([]*Deployment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+selectColumns+ownershipJoin+`
		WHERE d.project_id = $1
		  AND u.clerk_id = $2
		  AND ($3 = '' OR (d.created_at, d.id) < (
				SELECT c.created_at, c.id FROM deployments c WHERE c.id = $3
		  ))
		ORDER BY d.created_at DESC, d.id DESC
		LIMIT $4
	`, projectID, clerkID, cursor, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deployments := []*Deployment{}
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, d)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return deployments, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanDeployment(s scanner) (*Deployment, error) {
	var d Deployment

	err := s.Scan(
		&d.Id, &d.ProjectId, &d.AgentId, &d.TriggeredBy, &d.TriggerSource,
		&d.CommitSha, &d.CommitMessage, &d.Branch, &d.Status, &d.ImageTag,
		&d.ContainerId, &d.ErrorMessage, &d.StartedAt, &d.CompletedAt,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &d, nil
}
