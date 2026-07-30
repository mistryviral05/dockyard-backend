package workspace

import (
	"context"
	"errors"

	postgres "github.com/mistryviral05/dockyard/internal/http/database"
	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

type Repository interface {
	CreateDefault(ctx context.Context, userId string) (string, error)
	Create(ctx context.Context, dto *CreateWorkspaceDto) (string, error)
	GetByClerkId(ctx context.Context, clerkID string) ([]*Workspace, error)
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

// Create inserts a workspace for the user identified by their Clerk id.
func (r *repository) Create(ctx context.Context, dto *CreateWorkspaceDto) (string, error) {
	wsID := id.WorkspaceID()

	res, err := r.db.ExecContext(ctx, `
		INSERT INTO workspaces (id, user_id, wsp_name, wsp_slug)
		SELECT $1, u.id, $3, $4
		FROM users u
		WHERE u.clerk_id = $2
	`, wsID, dto.ClerkId, dto.Name, dto.Slug)
	if err != nil {
		return "", err
	}

	// If no user matched the clerk_id, nothing was inserted.
	n, err := res.RowsAffected()
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", errors.New("user not found") // define a sentinel error in the package
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
// GetByClerkId returns the workspaces owned by the user with this Clerk id.
func (r *repository) GetByClerkId(ctx context.Context, clerkID string) ([]*Workspace, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT w.id, w.wsp_name, w.wsp_slug
		FROM workspaces w
		JOIN users u ON u.id = w.user_id
		WHERE u.clerk_id = $1`, clerkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workspaces := []*Workspace{} // ← not nil (see Fix 2)
	for rows.Next() {
		var ws Workspace
		if err := rows.Scan(&ws.Id, &ws.WspName, &ws.WspSlug); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, &ws)
	}
	if err := rows.Err(); err != nil { // catches iteration errors
		return nil, err
	}
	return workspaces, nil
}
