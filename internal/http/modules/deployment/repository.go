package deployment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
	postgres "github.com/mistryviral05/dockyard/internal/http/database"
	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

var updatableColumns = []string{
	"agent_id",
	"image_tag",
	"container_id",
	"error_message",
}

const (
	ErrInvalidTransition = "invalid transition"
)

type Repository interface {
	Create(ctx context.Context, dto *CreateDeploymentDto) error
	Transition(ctx context.Context, id, to string, extra map[string]interface{}) (bool, error)
	ResolveProjectForUser(ctx context.Context, projectID, clerkID string) (*ProjectRef, error)
	ResolveProject(ctx context.Context, projectID string) (*ProjectRef, error)
	HasActiveDeployment(ctx context.Context, projectID string) (bool, error)
	GetByID(ctx context.Context, id, clerkID string) (*Deployment, error)
	ListByProject(ctx context.Context, projectID, clerkID, cursor string, limit int) ([]*Deployment, error)
	GetBuildContext(ctx context.Context, deploymentID string) (*BuildContext, error)
	SetCommit(ctx context.Context, deploymentID, sha, message string) error
	InsertLogs(ctx context.Context, deploymentID string, entries []LogEntry) error
	ListLogs(ctx context.Context, deploymentID, clerkID, cursor string, limit int) ([]*DeploymentLog, error)

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

func (r *repository) GetBuildContext(ctx context.Context, deploymentID string) (*BuildContext, error) {
	var b BuildContext

	err := r.db.QueryRowContext(ctx, `
		SELECT d.id, d.project_id, d.branch,
		       COALESCE(rp.repo_full_name, ''), gi.installation_id
		FROM deployments d
		JOIN projects p              ON p.id = d.project_id
		JOIN repositories rp         ON rp.project_id = p.id
		JOIN github_installations gi ON gi.id = rp.installation_id
		WHERE d.id = $1
	`, deploymentID).Scan(&b.DeploymentId, &b.ProjectId, &b.Branch, &b.RepoFullName, &b.InstallationId)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRepoNotLinked
		}
		return nil, err
	}

	return &b, nil
}

func (r *repository) SetCommit(ctx context.Context, deploymentID, sha, message string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE deployments
		SET commit_sha = $2, commit_message = $3, updated_at = NOW()
		WHERE id = $1
	`, deploymentID, sha, message)

	return err
}

func (r *repository) Transition(ctx context.Context, id, to string, extra map[string]interface{}) (bool, error) {

	from, ok := PredecessorsOf(to)

	if !ok {
		return false, errors.New(ErrInvalidTransition)
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
		// 0 rows: status match nahi hui. Galti nahi, par hua bhi nahi.
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to transition deployment: %w", err)
	}

	return true, nil
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

// ResolveProject is the ownerless twin of ResolveProjectForUser. Automatic
// deploys (a GitHub push) have no signed-in user to check against: the caller
// has already proved the request is genuine, so the project is looked up by id
// alone. Never expose this through an HTTP handler without an ownership check
// of its own.
func (r *repository) ResolveProject(ctx context.Context, projectID string) (*ProjectRef, error) {
	var ref ProjectRef

	err := r.db.QueryRowContext(ctx, `
		SELECT p.id, w.user_id, p.default_branch, pa.agent_id
		FROM projects p
		JOIN workspaces w ON w.id = p.wsp_id
		LEFT JOIN project_agent pa ON pa.project_id = p.id
		WHERE p.id = $1
		ORDER BY pa.created_at DESC
		LIMIT 1
	`, projectID).Scan(&ref.ProjectId, &ref.UserId, &ref.DefaultBranch, &ref.AgentId)

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

// InsertLogs ek batch ko ek hi INSERT me daalta hai. Har line pe alag query
// bhejna build ke output ki raftaar pe DB ko bottleneck bana deta.
//
// logged_at caller deta hai, now() nahi, kyunki lines batch me thodi der baad
// aati hain. Caller har line ka waqt strictly badhta hua rakhta hai, isliye
// (logged_at, id) se ordering wahi rehti hai jo build me thi.
func (r *repository) InsertLogs(ctx context.Context, deploymentID string, entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	var (
		placeholders = make([]string, 0, len(entries))
		args         = make([]interface{}, 0, len(entries)*5)
	)

	for i, e := range entries {
		n := i * 5
		placeholders = append(placeholders,
			fmt.Sprintf("($%d, $%d, $%d::log_type, $%d, $%d)", n+1, n+2, n+3, n+4, n+5))
		args = append(args, id.LogID(), deploymentID, e.Type, e.Message, e.At)
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deployment_logs (id, deployment_id, log_type, message, logged_at)
		VALUES `+strings.Join(placeholders, ", "), args...)

	return err
}

// ListLogs purane se naye ki taraf padhta hai, kyunki log viewer upar se
// neeche chalta hai. Cursor pichhle page ki aakhri log id hai.
func (r *repository) ListLogs(ctx context.Context, deploymentID, clerkID, cursor string, limit int) ([]*DeploymentLog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT l.id, l.log_type, l.message, l.logged_at
		FROM deployment_logs l
		JOIN deployments d   ON d.id = l.deployment_id
		JOIN projects p      ON p.id = d.project_id
		JOIN workspaces w    ON w.id = p.wsp_id
		JOIN users u         ON u.id = w.user_id
		WHERE l.deployment_id = $1
		  AND u.clerk_id = $2
		  AND ($3 = '' OR (l.logged_at, l.id) > (
				SELECT c.logged_at, c.id FROM deployment_logs c WHERE c.id = $3
		  ))
		ORDER BY l.logged_at ASC, l.id ASC
		LIMIT $4
	`, deploymentID, clerkID, cursor, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []*DeploymentLog{}
	for rows.Next() {
		var l DeploymentLog
		if err := rows.Scan(&l.Id, &l.LogType, &l.Message, &l.LoggedAt); err != nil {
			return nil, err
		}
		logs = append(logs, &l)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return logs, nil
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
