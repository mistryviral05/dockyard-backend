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
}

type repository struct {
	db postgres.DBTX // *sql.DB in normal use, *sql.Tx inside a transaction
}

var ErrUserNotFound = errors.New("user not found")
var ErrInstallationNotFound = errors.New("installation not found")

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
