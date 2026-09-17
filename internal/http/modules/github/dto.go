package github

import (
	"database/sql"
	"time"
)

type ConnectRequest struct {
	InstallationId string `json:"installation_id" validate:"required"`
	State          string `json:"state" validate:"required"`
	ClerkId        string `validate:"required"`
}

type ConnectDto struct {
	ClerkId             string
	InstallationId      string
	AccountLogin        string
	AccountType         string
	RepositorySelection string
	Permissions         string
}

type InstallationDto struct {
	Id           string `json:"id"`
	AccountLogin string `json:"account_login"`
}
// RepoRef is everything the webhook needs about a repository we track,
// fetched in one query so a push costs a single round trip before any work.
type RepoRef struct {
	RepositoryId string // our repositories row id, not GitHub's

	// A repository row can exist without a project: the link is dropped with
	// ON DELETE SET NULL when the project goes away.
	ProjectId sql.NullString

	// The branch a push must be on to deploy. Comes from the project when the
	// repo is linked, otherwise from the repository row.
	DefaultBranch string
}

// WebhookEventDto is one row of the delivery log. DeliveryId is GitHub's
// X-GitHub-Delivery header and is unique, which is what makes redeliveries
// safe to ignore.
type WebhookEventDto struct {
	RepositoryId   string
	EventType      string
	DeliveryId     string
	Payload        string // raw JSON exactly as GitHub sent it
	SignatureValid bool
}

// PushOutcome tells the handler what came of a push. Ignored is the normal
// case, not an error: most pushes are on branches nobody deploys.
type PushOutcome struct {
	Ignored      bool
	Reason       string
	DeploymentId string
}

type RepositoryDto struct {
	RepoId        string    `json:"repo_id"`
	RepoName      string    `json:"repo_name"`
	RepoFullName  string    `json:"repo_full_name"`
	RepoUrl       string    `json:"repo_url"`
	IsPrivate     bool      `json:"is_private"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
}
