package workspace

import (
	"context"

	postgres "github.com/mistryviral05/dockyard/internal/http/database"
	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

type Repository interface {
	CreateDefault(ctx context.Context, userId string) (string, error)
	WithTx(db postgres.DBTX) Repository // rebinds the repo onto a transaction
}

type repository struct {
	db postgres.DBTX // *sql.DB in normal use, *sql.Tx inside a transaction
}

func NewRepository(db postgres.DBTX) Repository {
	return &repository{db: db}
}

// WithTx returns a NEW repository bound to the given tx (does not mutate this one).
func (r *repository) WithTx(db postgres.DBTX) Repository {
	return &repository{db: db}
}

// CreateDefault seeds a user's first workspace.
// slug = userID guarantees uniqueness (uq_workspaces_wsp_slug).
func (r *repository) CreateDefault(ctx context.Context, userId string) (string, error) {
	wsId := id.WorkspaceID()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workspaces (id, user_id, wsp_name, wsp_slug)
		VALUES ($1, $2, $3, $4)
	`, wsId, userId, "Personal Workspace", "personal-workspace")
	if err != nil {
		return "", err
	}
	return wsId, nil
}
