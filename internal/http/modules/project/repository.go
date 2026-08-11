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
