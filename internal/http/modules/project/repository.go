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
)

type Repository interface {
	Create(ctx context.Context, dto *CreateProjectDto) (string, error)
	CreateRepository(ctx context.Context, dto *CreateRepositoryDto) error
	GetByClerkId(ctx context.Context, clerkID, workspaceID string) ([]*Project, error)
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
		INSERT INTO projects (id, wsp_id, project_name, project_stack)
		SELECT $1, w.id, $4, $5::tech_stack
		FROM workspaces w
		JOIN users u ON u.id = w.user_id
		WHERE w.id = $2 AND u.clerk_id = $3
		RETURNING id
	`, dto.ProjectId, dto.WorkspaceId, dto.ClerkId, dto.Name, dto.Stack).Scan(&projectId)

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

// CreateRepository links the github repo to the project, resolving the
// installation row from the repo owner owned by this clerk user.
func (r *repository) CreateRepository(ctx context.Context, dto *CreateRepositoryDto) error {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO repositories (id, installation_id, project_id, github_repo_id, repo_name, repo_full_name, repo_url, is_private)
		SELECT $1, gi.id, $2, $5, $6, $7, $8, $9
		FROM github_installations gi
		JOIN users u ON u.id = gi.user_id
		WHERE u.clerk_id = $3 AND gi.account_login = $4
		ORDER BY gi.created_at DESC
		LIMIT 1
	`, dto.RepositoryId, dto.ProjectId, dto.ClerkId, dto.RepoOwner,
		dto.GithubRepoId, dto.RepoName, dto.RepoFullName, dto.RepoUrl, dto.IsPrivate)

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
