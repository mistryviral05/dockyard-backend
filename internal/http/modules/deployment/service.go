package deployment

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

type Enqueuer interface {
	EnqueueBuild(ctx context.Context, deploymentID string) error
}

type Service struct {
	db     *sql.DB
	logger *slog.Logger
	repo   Repository
	queue  Enqueuer
}

func NewService(db *sql.DB, logger *slog.Logger, repo Repository) *Service {
	return &Service{
		db:     db,
		logger: logger,
		repo:   repo,
	}
}

func (s *Service) CreateManual(ctx context.Context, clerkId, projectId, branch string) error {
	// resolver called
	ref, err := s.repo.ResolveProjectForUser(ctx, projectId, clerkId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProjectNotFound
		}
		return err
	}

	// checking agent
	if !ref.AgentId.Valid {
		return ErrAgentNotSet
	}

	// if any deployment is going to the running ?
	active, err := s.repo.HasActiveDeployment(ctx, projectId)
	if err != nil {
		return err
	}

	if active {
		return ErrDeployInProgress
	}

	if branch == "" {
		branch = ref.DefaultBranch
	}

	// creating a row for the deployment after all checks passes
	d := &CreateDeploymentDto{
		DeploymentId:  id.DeploymentID(),
		ProjectId:     ref.ProjectId,
		AgentId:       ref.AgentId.String,
		TriggeredBy:   &ref.UserId,
		TriggerSource: TriggerManual,
		Branch:        branch,
	}

	if err := s.repo.Create(ctx, d); err != nil {
		return err
	}
	if err := s.queue.EnqueueBuild(ctx, d.DeploymentId); err != nil {
		return err
	}

	if err := s.repo.Transition(ctx, d.DeploymentId, StatusQueued, nil); err != nil {
		return err
	}
	return nil

}
