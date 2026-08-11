package project

import (
	"context"
	"database/sql"

	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

type Service struct {
	db   *sql.DB
	repo Repository
}

func NewService(repo Repository, db *sql.DB) *Service {
	return &Service{repo: repo, db: db}
}

// Create writes the project and its repository in one transaction:
// a project without its repo row is not a valid state.
func (s *Service) Create(ctx context.Context, clerkID string, req *CreateProjectRequest, repo *RepositoryRequest) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	txRepo := s.repo.WithTx(tx)

	projectId, err := txRepo.Create(ctx, &CreateProjectDto{
		ClerkId:     clerkID,
		ProjectId:   id.ProjectID(),
		WorkspaceId: req.WorkspaceId,
		Name:        req.Name,
		Stack:       req.Stack,
	})
	if err != nil {
		return err
	}

	err = txRepo.CreateRepository(ctx, &CreateRepositoryDto{
		ClerkId:      clerkID,
		RepositoryId: id.RepositoryID(),
		ProjectId:    projectId,
		RepoOwner:    repo.RepoOwner,
		GithubRepoId: repo.GithubRepoId,
		RepoName:     repo.RepoName,
		RepoFullName: repo.RepoFullName,
		RepoUrl:      repo.RepoUrl,
		IsPrivate:    repo.IsPrivate,
	})
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Service) Get(ctx context.Context, clerkID, workspaceID string) ([]*ProjectDto, error) {
	projects, err := s.repo.GetByClerkId(ctx, clerkID, workspaceID)
	if err != nil {
		return nil, err
	}

	dtos := make([]*ProjectDto, 0, len(projects))
	for _, p := range projects {
		dto := &ProjectDto{
			Id:       p.Id,
			Name:     p.ProjectName.String,
			Stack:    p.ProjectStack.String,
			RepoName: p.RepoName.String,
		}
		if p.DeploymentStatus.Valid {
			dto.DeploymentStatus = &p.DeploymentStatus.String
		}
		dtos = append(dtos, dto)
	}

	return dtos, nil
}
