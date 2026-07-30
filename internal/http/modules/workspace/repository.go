package workspace

import (
	"context"

	postgres "github.com/mistryviral05/dockyard/internal/http/database"
	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

type Repository interface {
	CreateDefault(ctx context.Context, userId string) (string, error)
	Create(ctx context.Context, dto *CreateWorkspaceDto) (string, error)
	GetByUserId(ctx context.Context, userId string) ([]*Workspace, error)
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

func (r *repository) Create(ctx context.Context, dto *CreateWorkspaceDto) (string, error) {
	wsID := id.WorkspaceID()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workspaces (id, user_id, wsp_name, wsp_slug)
		VALUES ($1, $2, $3, $4)
	`, wsID, dto.UserId, dto.Name, dto.Slug)
	if err != nil {
		return "", err
	}
	return wsID, nil
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

// Create Get Workspaces details by user id
func (r *repository) GetByUserId(ctx context.Context,userId string) ([]*Workspace, error) {

	rows,err := r.db.QueryContext(ctx,"SELECT id, wsp_name, wsp_slug FROM workspaces WHERE user_id = $1", userId)

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var workspaces []*Workspace
	for rows.Next() {
		var ws Workspace
		if err := rows.Scan(&ws.Id, &ws.WspName, &ws.WspSlug); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, &ws)
	}
	return workspaces, nil
}
