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
}

type repository struct {
	db postgres.DBTX // *sql.DB in normal use, *sql.Tx inside a transaction
}

var ErrUserNotFound = errors.New("user not found")

func (r *repository) WithTx(db postgres.DBTX) Repository {
	return &repository{db: db}
}

func NewRepository(db postgres.DBTX) Repository {
	return &repository{db: db}
}

func (r *repository) CreateInstallation(ctx context.Context, p ConnectDto) (string, error) {
	rowId := id.InstallationID()
	var returnedId string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO github_installations (id,user_id,installation_id,account_login,account_type,repository_selection)
		SELECT $1,u.id,$3,$4,$5,$6
		FROM users u
		WHERE u.clerk_id = $2
		ON CONFLICT (installation_id) DO UPDATE
			SET account_login        = EXCLUDED.account_login,
			    account_type         = EXCLUDED.account_type,
			    repository_selection = EXCLUDED.repository_selection,
			    updated_at           = now()
		RETURNING id
	`, rowId, p.ClerkId, p.InstallationId, p.AccountLogin, p.AccountType, p.RepositorySelection).Scan(&returnedId)

	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUserNotFound
	}

	if err != nil {
		return "", err
	}
	return returnedId, nil
}
