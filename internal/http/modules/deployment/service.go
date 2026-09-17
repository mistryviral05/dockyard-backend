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

	// Ek build ki hazaaron lines ho sakti hain, isliye log page bada hai.
	defaultLogLimit = 200
	maxLogLimit     = 1000
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

func NewService(db *sql.DB, logger *slog.Logger, repo Repository, queue Enqueuer) *Service {
	return &Service{
		db:     db,
		logger: logger,
		repo:   repo,
		queue:  queue,
	}
}

// CreateManual naye deployment ki id return karta hai, taki frontend deploy
// ke turant baad us deployment ke page pe bhej sake.
func (s *Service) CreateManual(ctx context.Context, clerkId, projectId, branch string) (string, error) {
	// resolver called
	ref, err := s.repo.ResolveProjectForUser(ctx, projectId, clerkId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrProjectNotFound
		}
		return "", err
	}

	// checking agent
	if !ref.AgentId.Valid {
		return "", ErrAgentNotSet
	}

	// if any deployment is going to the running ?
	active, err := s.repo.HasActiveDeployment(ctx, projectId)
	if err != nil {
		return "", err
	}

	if active {
		return "", ErrDeployInProgress
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
	return s.enqueue(ctx, d)
}

// CreateFromPush queues a deployment for a GitHub push. It is CreateManual
// without the ownership check: there is no signed-in user on a webhook, and the
// caller has already verified GitHub's signature and resolved the project from
// a repository row we linked ourselves.
//
// triggered_by stays NULL on purpose — nobody clicked deploy, GitHub did.
func (s *Service) CreateFromPush(ctx context.Context, projectId, branch, commitSha, commitMessage string) (string, error) {
	ref, err := s.repo.ResolveProject(ctx, projectId)
	if err != nil {
		return "", err
	}

	if !ref.AgentId.Valid {
		return "", ErrAgentNotSet
	}

	// A push while a build is running is dropped rather than queued. Queueing
	// it would mean building a commit that a later push has already replaced.
	active, err := s.repo.HasActiveDeployment(ctx, projectId)
	if err != nil {
		return "", err
	}
	if active {
		return "", ErrDeployInProgress
	}

	if branch == "" {
		branch = ref.DefaultBranch
	}

	d := &CreateDeploymentDto{
		DeploymentId:  id.DeploymentID(),
		ProjectId:     ref.ProjectId,
		AgentId:       ref.AgentId.String,
		TriggeredBy:   nil,
		TriggerSource: TriggerGitHubPush,
		Branch:        branch,
	}

	// The worker resolves the branch head itself, so these are the commit we
	// know about now, not necessarily the one that ends up being built.
	if commitSha != "" {
		d.CommitSha = &commitSha
	}
	if commitMessage != "" {
		d.CommitMessage = &commitMessage
	}

	return s.enqueue(ctx, d)
}

// enqueue writes the deployment row and its queued transition in one
// transaction, then hands the id to the worker. The row must be committed
// before the job is published, otherwise the worker can pick it up before it
// exists.
func (s *Service) enqueue(ctx context.Context, d *CreateDeploymentDto) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	repo := s.repo.WithTx(tx)
	if err := repo.Create(ctx, d); err != nil {
		return "", err
	}
	if _, err := repo.Transition(ctx, d.DeploymentId, StatusQueued, nil); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	if err := s.queue.EnqueueBuild(ctx, d.DeploymentId); err != nil {
		return "", err
	}

	return d.DeploymentId, nil
}

func (s *Service) Get(ctx context.Context, clerkId, deploymentId string) (*Deployment, error) {
	return s.repo.GetByID(ctx, deploymentId, clerkId)
}

// Logs ek deployment ke saved logs deta hai, purane se naye.
// Ownership query me hi check hota hai, isliye kisi aur ka deployment
// hamesha khali page deta hai.
func (s *Service) Logs(ctx context.Context, clerkId, deploymentId, cursor string, limit int) ([]*DeploymentLog, error) {
	if limit <= 0 {
		limit = defaultLogLimit
	}
	if limit > maxLogLimit {
		limit = maxLogLimit
	}

	return s.repo.ListLogs(ctx, deploymentId, clerkId, cursor, limit)
}

// List gives a project's deployments, newest first.
// An empty cursor means the first page.
func (s *Service) List(ctx context.Context, clerkId, projectId, cursor string, limit int) ([]*Deployment, error) {
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	return s.repo.ListByProject(ctx, projectId, clerkId, cursor, limit)
}
