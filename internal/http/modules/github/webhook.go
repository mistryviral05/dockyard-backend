package github

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	gogithub "github.com/google/go-github/v66/github"
)

// branchRefPrefix: GitHub sends a full ref. Tag pushes arrive on the same
// event with refs/tags/, so the prefix is also how we tell them apart.
const branchRefPrefix = "refs/heads/"

// Deployer is the deployment module seen from here. It is an interface, and
// takes plain strings, so this package does not import the deployment package
// at all — the two modules stay independent.
type Deployer interface {
	CreateFromPush(ctx context.Context, projectId, branch, commitSha, commitMessage string) (string, error)
}

// VerifyWebhook checks GitHub's HMAC signature and returns the raw body.
// This is the only thing that proves a request really came from GitHub, so
// nothing else in the flow may run before it succeeds.
func (s *Service) VerifyWebhook(r *http.Request) ([]byte, error) {
	return gogithub.ValidatePayload(r, []byte(s.cfg.Github.WebhookSecret))
}

// HandlePush turns a verified push into a deployment, the same one the deploy
// button creates. Most pushes do not deploy — wrong branch, unlinked repo, a
// branch deletion — so an ignored outcome is the common path and carries the
// reason for the log.
//
// The returned error is reserved for storage failures, where a GitHub retry
// has a real chance of working. Anything the retry cannot fix is an ignore.
func (s *Service) HandlePush(ctx context.Context, e *gogithub.PushEvent, deliveryId string, payload []byte) (PushOutcome, error) {
	// Resolve the repo first: the delivery log has a foreign key to it, so
	// there is nowhere to record a push from a repo we do not track.
	githubRepoId := strconv.FormatInt(e.GetRepo().GetID(), 10)

	repoRef, err := s.repo.FindRepositoryByGithubID(ctx, githubRepoId)
	if err != nil {
		if errors.Is(err, ErrRepositoryNotTracked) {
			return ignored("repository is not linked to any project"), nil
		}
		return PushOutcome{}, err
	}

	fresh, err := s.repo.RecordWebhookEvent(ctx, WebhookEventDto{
		RepositoryId:   repoRef.RepositoryId,
		EventType:      "push",
		DeliveryId:     deliveryId,
		Payload:        string(payload),
		SignatureValid: true, // VerifyWebhook already rejected anything else
	})
	if err != nil {
		return PushOutcome{}, err
	}
	if !fresh {
		// A redelivery of a push we have already acted on.
		return ignored("duplicate delivery"), nil
	}

	// Whatever we decide below, the delivery has been dealt with.
	defer func() {
		if err := s.repo.MarkWebhookProcessed(ctx, deliveryId); err != nil {
			s.logger.Error("failed to mark webhook processed",
				slog.String("delivery_id", deliveryId),
				slog.String("error", err.Error()),
			)
		}
	}()

	ref := e.GetRef()
	if !strings.HasPrefix(ref, branchRefPrefix) {
		return ignored("not a branch push"), nil
	}
	branch := strings.TrimPrefix(ref, branchRefPrefix)

	// A deleted branch still arrives as a push, with a zero head commit.
	if e.GetDeleted() {
		return ignored("branch was deleted"), nil
	}

	if !repoRef.ProjectId.Valid {
		return ignored("repository is not linked to any project"), nil
	}

	// Only the deploy branch builds. Feature branches push constantly and
	// building each one would burn the queue for nothing.
	if branch != repoRef.DefaultBranch {
		return ignored("branch " + branch + " is not the deploy branch"), nil
	}

	head := e.GetHeadCommit()

	deploymentId, err := s.deployer.CreateFromPush(ctx, repoRef.ProjectId.String, branch, head.GetID(), head.GetMessage())
	if err != nil {
		// Business refusals live here too — no agent attached, a build already
		// running. GitHub retrying would not change any of them, so the push is
		// recorded and dropped rather than failed.
		return ignored(err.Error()), nil
	}

	return PushOutcome{DeploymentId: deploymentId}, nil
}

func ignored(reason string) PushOutcome {
	return PushOutcome{Ignored: true, Reason: reason}
}
