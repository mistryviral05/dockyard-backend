package project

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
	postgres "github.com/mistryviral05/dockyard/internal/http/database"
)

var (
	ErrWorkspaceNotFound    = errors.New("workspace not found")
	ErrInstallationNotFound = errors.New("github installation not found")
	ErrRepoAlreadyLinked    = errors.New("this repository is already linked to a project")
	ErrProjectNotFound      = errors.New("project not found")
	ErrAgentNotFound        = errors.New("agent not found")
)

type Repository interface {
	Create(ctx context.Context, dto *CreateProjectDto) (string, error)
	CreateRepository(ctx context.Context, dto *CreateRepositoryDto) error
	GetByClerkId(ctx context.Context, clerkID, workspaceID string) ([]*Project, error)
	GetDetailByID(ctx context.Context, projectID, clerkID string) (*ProjectDetail, error)
	CheckOwnership(ctx context.Context, projectID, agentID, clerkID string) error
	ReplaceAgent(ctx context.Context, projectID, agentID string) error
	WithTx(db postgres.DBTX) Repository
}

type repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) Repository {
	return &repository{db: db}
}

func (r *repository) WithTx(db postgres.DBTX) Repository {
	return &repository{db: db}
}

// Create inserts the project only if the workspace belongs to this clerk user.
func (r *repository) Create(ctx context.Context, dto *CreateProjectDto) (string, error) {
	var projectId string

	err := r.db.QueryRowContext(ctx, `
		INSERT INTO projects (id, wsp_id, project_name, project_stack, default_branch)
		SELECT $1, w.id, $4, $5::tech_stack, $6
		FROM workspaces w
		JOIN users u ON u.id = w.user_id
		WHERE w.id = $2 AND u.clerk_id = $3
		RETURNING id
	`, dto.ProjectId, dto.WorkspaceId, dto.ClerkId, dto.Name, dto.Stack, dto.DefaultBranch).Scan(&projectId)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrWorkspaceNotFound
		}
		return "", err
	}

	return projectId, nil
}

// GetByClerkId lists the projects of one workspace owned by this clerk user,
// with their repo name and the status of their most recent deployment
// (null when never deployed).
func (r *repository) GetByClerkId(ctx context.Context, clerkID, workspaceID string) ([]*Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.project_name, p.project_stack, r.repo_name, d.status
		FROM projects p
		JOIN workspaces w ON w.id = p.wsp_id
		JOIN users u ON u.id = w.user_id
		LEFT JOIN repositories r ON r.project_id = p.id
		LEFT JOIN LATERAL (
			SELECT dp.status
			FROM deployments dp
			WHERE dp.project_id = p.id
			ORDER BY dp.created_at DESC
			LIMIT 1
		) d ON true
		WHERE u.clerk_id = $1 AND w.id = $2
		ORDER BY p.created_at DESC
	`, clerkID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []*Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.Id, &p.ProjectName, &p.ProjectStack, &p.RepoName, &p.DeploymentStatus); err != nil {
			return nil, err
		}
		projects = append(projects, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return projects, nil
}

// GetDetailByID ek project ka poora detail deta hai: repo, uska sabse naya
// agent aur sabse naya deployment. project_agent ka PK (project_id, agent_id)
// hai, isliye ek project pe ek se zyada row ho sakti hai. Yahan sabse naya
// hi liya jaata hai, bilkul waise jaise deploy ka resolver leta hai.
func (r *repository) GetDetailByID(ctx context.Context, projectID, clerkID string) (*ProjectDetail, error) {
	var p ProjectDetail

	err := r.db.QueryRowContext(ctx, `
		SELECT
			p.id, p.project_name, p.project_stack, p.default_branch, p.is_deployed, p.created_at,
			r.repo_name, r.repo_full_name, r.repo_url, r.is_private,
			a.id, a.agent_name, a.status, a.port,
			d.id, d.status, d.branch, d.created_at
		FROM projects p
		JOIN workspaces w ON w.id = p.wsp_id
		JOIN users u ON u.id = w.user_id
		LEFT JOIN repositories r ON r.project_id = p.id
		LEFT JOIN LATERAL (
			SELECT ag.id, ag.agent_name, ag.status, ag.port
			FROM project_agent pa
			JOIN agents ag ON ag.id = pa.agent_id
			WHERE pa.project_id = p.id
			ORDER BY pa.created_at DESC
			LIMIT 1
		) a ON true
		LEFT JOIN LATERAL (
			SELECT dp.id, dp.status, dp.branch, dp.created_at
			FROM deployments dp
			WHERE dp.project_id = p.id
			ORDER BY dp.created_at DESC
			LIMIT 1
		) d ON true
		WHERE p.id = $1 AND u.clerk_id = $2
	`, projectID, clerkID).Scan(
		&p.Id, &p.ProjectName, &p.ProjectStack, &p.DefaultBranch, &p.IsDeployed, &p.CreatedAt,
		&p.RepoName, &p.RepoFullName, &p.RepoUrl, &p.RepoIsPrivate,
		&p.AgentId, &p.AgentName, &p.AgentStatus, &p.AgentPort,
		&p.DeploymentId, &p.DeploymentStatus, &p.DeploymentBranch, &p.DeploymentCreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}

	return &p, nil
}

// CheckOwnership dekhta hai ki project aur agent dono isi clerk user ke
// hain. Dono ko ek hi roundtrip me poochha jaata hai, par error alag alag
// aata hai taki user ko sahi message mile.
func (r *repository) CheckOwnership(ctx context.Context, projectID, agentID, clerkID string) error {
	var projectOk, agentOk bool

	err := r.db.QueryRowContext(ctx, `
		SELECT
			EXISTS (
				SELECT 1 FROM projects p
				JOIN workspaces w ON w.id = p.wsp_id
				JOIN users u ON u.id = w.user_id
				WHERE p.id = $1 AND u.clerk_id = $3
			),
			EXISTS (
				SELECT 1 FROM agents a
				JOIN users u ON u.id = a.user_id
				WHERE a.id = $2 AND u.clerk_id = $3
			)
	`, projectID, agentID, clerkID).Scan(&projectOk, &agentOk)

	if err != nil {
		return err
	}
	if !projectOk {
		return ErrProjectNotFound
	}
	if !agentOk {
		return ErrAgentNotFound
	}

	return nil
}

// ReplaceAgent project pe ek hi agent rehne deta hai. project_agent ka PK
// (project_id, agent_id) hai, isliye sirf insert karne se purani row bachi
// reh jaati aur deploy kisi aur agent pe chala jaata. Pehle delete, phir
// insert. Isko transaction ke andar hi call karna hai.
func (r *repository) ReplaceAgent(ctx context.Context, projectID, agentID string) error {
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM project_agent WHERE project_id = $1
	`, projectID); err != nil {
		return err
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO project_agent (project_id, agent_id) VALUES ($1, $2)
	`, projectID, agentID)

	return err
}

// CreateRepository links the github repo to the project, resolving the
// installation row from the repo owner owned by this clerk user.
func (r *repository) CreateRepository(ctx context.Context, dto *CreateRepositoryDto) error {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO repositories (id, installation_id, project_id, github_repo_id, repo_name, repo_full_name, repo_url, is_private, default_branch)
		SELECT $1, gi.id, $2, $5, $6, $7, $8, $9, $10
		FROM github_installations gi
		JOIN users u ON u.id = gi.user_id
		WHERE u.clerk_id = $3 AND gi.account_login = $4
		ORDER BY gi.created_at DESC
		LIMIT 1
	`, dto.RepositoryId, dto.ProjectId, dto.ClerkId, dto.RepoOwner,
		dto.GithubRepoId, dto.RepoName, dto.RepoFullName, dto.RepoUrl, dto.IsPrivate, dto.DefaultBranch)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return ErrRepoAlreadyLinked
		}
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrInstallationNotFound
	}

	return nil
}
