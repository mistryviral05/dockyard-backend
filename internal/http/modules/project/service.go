package project

import (
	"context"
	"database/sql"
	"strings"

	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

// Deploy p.default_branch padhta hai, isliye ye kabhi khali nahi jaana chahiye.
const defaultBranch = "main"

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

	branch := strings.TrimSpace(repo.DefaultBranch)
	if branch == "" {
		branch = defaultBranch
	}

	projectId, err := txRepo.Create(ctx, &CreateProjectDto{
		ClerkId:       clerkID,
		ProjectId:     id.ProjectID(),
		WorkspaceId:   req.WorkspaceId,
		Name:          req.Name,
		Stack:         req.Stack,
		DefaultBranch: branch,
	})
	if err != nil {
		return err
	}

	err = txRepo.CreateRepository(ctx, &CreateRepositoryDto{
		ClerkId:       clerkID,
		RepositoryId:  id.RepositoryID(),
		ProjectId:     projectId,
		RepoOwner:     repo.RepoOwner,
		GithubRepoId:  repo.GithubRepoId,
		RepoName:      repo.RepoName,
		RepoFullName:  repo.RepoFullName,
		RepoUrl:       repo.RepoUrl,
		IsPrivate:     repo.IsPrivate,
		DefaultBranch: branch,
	})
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetByID project detail page ka data banata hai. Jo bhi block DB me nahi
// mila wo JSON me null jaata hai, khali object nahi, taki frontend seedha
// null check kar sake.
func (s *Service) GetByID(ctx context.Context, clerkID, projectID string) (*ProjectDetailDto, error) {
	p, err := s.repo.GetDetailByID(ctx, projectID, clerkID)
	if err != nil {
		return nil, err
	}

	dto := &ProjectDetailDto{
		Id:            p.Id,
		Name:          p.ProjectName.String,
		Stack:         p.ProjectStack.String,
		DefaultBranch: p.DefaultBranch,
		IsDeployed:    p.IsDeployed,
		CreatedAt:     p.CreatedAt,
	}

	if p.RepoFullName.Valid {
		dto.Repository = &RepositoryDto{
			RepoName:     p.RepoName.String,
			RepoFullName: p.RepoFullName.String,
			RepoUrl:      p.RepoUrl.String,
			IsPrivate:    p.RepoIsPrivate.Bool,
		}
	}

	if p.AgentId.Valid {
		dto.Agent = &ProjectAgentDto{
			Id:     p.AgentId.String,
			Name:   p.AgentName.String,
			Status: p.AgentStatus.String,
			Port:   p.AgentPort.String,
		}
	}

	if p.DeploymentId.Valid {
		dto.LastDeployment = &LastDeploymentDto{
			Id:        p.DeploymentId.String,
			Status:    p.DeploymentStatus.String,
			Branch:    p.DeploymentBranch.String,
			CreatedAt: p.DeploymentCreatedAt.Time,
		}
	}

	return dto, nil
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
