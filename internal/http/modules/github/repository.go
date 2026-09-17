package github

import (
	"context"
	"database/sql"
	"errors"

	postgres "github.com/mistryviral05/dockyard/internal/http/database"
	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

type Repository interface {
	WithTx(db postgres.DBTX) Repository // rebinds the repo onto a transaction
	CreateInstallation(ctx context.Context, p ConnectDto) (string, error)
	GetInstallations(ctx context.Context, clerkId string) ([]InstallationDto, error)
	GetInstallationId(ctx context.Context, instRowId, clerkId string) (string, error)
	FindRepositoryByGithubID(ctx context.Context, githubRepoId string) (*RepoRef, error)
	RecordWebhookEvent(ctx context.Context, e WebhookEventDto) (bool, error)
	MarkWebhookProcessed(ctx context.Context, deliveryId string) error
}

type repository struct {
	db postgres.DBTX // *sql.DB in normal use, *sql.Tx inside a transaction
}

var ErrUserNotFound = errors.New("user not found")
var ErrInstallationNotFound = errors.New("installation not found")

// ErrRepositoryNotTracked means the push came from a repo the app is installed
// on but nobody has linked to a project here. Expected, not a failure.
var ErrRepositoryNotTracked = errors.New("repository not tracked")

func (r *repository) WithTx(db postgres.DBTX) Repository {
	return &repository{db: db}
}

func NewRepository(db postgres.DBTX) Repository {
	return &repository{db: db}
}

func (r *repository) GetInstallationId(ctx context.Context, instRowId, clerkId string) (string, error) {
	var installationId string
	err := r.db.QueryRowContext(ctx, `
		SELECT gi.installation_id 
		FROM github_installations gi 
		JOIN users u ON gi.user_id = u.id
		WHERE gi.id = $1 AND u.clerk_id = $2
	`, instRowId, clerkId).Scan(&installationId)

	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrInstallationNotFound // not found OR not owned by this user
	}
	if err != nil {
		return "", err
	}
	return installationId, nil
}

// FindRepositoryByGithubID looks a repo up by GitHub's numeric id rather than
// its full name, because the id survives renames and transfers while the name
// does not.
func (r *repository) FindRepositoryByGithubID(ctx context.Context, githubRepoId string) (*RepoRef, error) {
	var ref RepoRef

	err := r.db.QueryRowContext(ctx, `
		SELECT rp.id, rp.project_id, COALESCE(p.default_branch, rp.default_branch)
		FROM repositories rp
		LEFT JOIN projects p ON p.id = rp.project_id
		WHERE rp.github_repo_id = $1
	`, githubRepoId).Scan(&ref.RepositoryId, &ref.ProjectId, &ref.DefaultBranch)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRepositoryNotTracked
	}
	if err != nil {
		return nil, err
	}

	return &ref, nil
}

// RecordWebhookEvent stores the delivery and reports whether it was new.
// GitHub redelivers on timeouts and on a click in the UI, so the unique
// delivery_id is what stops one push from deploying twice.
func (r *repository) RecordWebhookEvent(ctx context.Context, e WebhookEventDto) (bool, error) {
	var inserted string

	err := r.db.QueryRowContext(ctx, `
		INSERT INTO webhook_events (id, repository_id, event_type, delivery_id, payload, signature_valid)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)
		ON CONFLICT (delivery_id) DO NOTHING
		RETURNING id
	`, id.WebhookEventID(), e.RepositoryId, e.EventType, e.DeliveryId, e.Payload, e.SignatureValid).Scan(&inserted)

	// No row back means the conflict fired: we have seen this delivery before.
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *repository) MarkWebhookProcessed(ctx context.Context, deliveryId string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE webhook_events SET processed_at = now() WHERE delivery_id = $1
	`, deliveryId)

	return err
}

func (r *repository) GetInstallations(ctx context.Context, clerkId string) ([]InstallationDto, error) {

	rows, err := r.db.QueryContext(ctx, `
		SELECT gi.id,gi.account_login 
		FROM github_installations gi 
		JOIN users u ON gi.user_id = u.id 
		WHERE u.clerk_id = $1 
		ORDER BY gi.created_at DESC
	`, clerkId)

	if err != nil {
		return nil, err
	}

	installations := []InstallationDto{}
	for rows.Next() {
		var installation InstallationDto
		if err := rows.Scan(&installation.Id, &installation.AccountLogin); err != nil {
			return nil, err
		}
		installations = append(installations, installation)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return installations, nil
}

func (r *repository) CreateInstallation(ctx context.Context, p ConnectDto) (string, error) {
	rowId := id.InstallationID()
	var returnedId string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO github_installations (id,user_id,installation_id,account_login,account_type,repository_selection,permissions)
		SELECT $1,u.id,$3,$4,$5,$6,$7::jsonb
		FROM users u
		WHERE u.clerk_id = $2
		ON CONFLICT (installation_id) DO UPDATE
			SET account_login        = EXCLUDED.account_login,
			    account_type         = EXCLUDED.account_type,
			    repository_selection = EXCLUDED.repository_selection,
				permissions           = EXCLUDED.permissions,
			    updated_at           = now()
		RETURNING id
	`, rowId, p.ClerkId, p.InstallationId, p.AccountLogin, p.AccountType, p.RepositorySelection, p.Permissions).Scan(&returnedId)

	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUserNotFound
	}

	if err != nil {
		return "", err
	}
	return returnedId, nil
}
