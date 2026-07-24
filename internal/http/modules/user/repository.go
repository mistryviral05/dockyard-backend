package user

import (
	"context"
	"database/sql"
	"errors"

	postgres "github.com/mistryviral05/dockyard/internal/http/database"
	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

type Repository interface {
	Create(ctx context.Context, dto *CreateUserDto) (string, bool, error)
	WithTx(db postgres.DBTX) Repository
}

type repository struct {
	db postgres.DBTX
}

func NewRepository(db *sql.DB) Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) WithTx(db postgres.DBTX) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, dto *CreateUserDto) (string, bool, error) {

	newID := id.UserID()

	var insertedID string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO users (id, clerk_id, email, name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (clerk_id) DO NOTHING
		RETURNING id
	`, newID, dto.ClerkId, dto.Email, dto.Name).Scan(&insertedID)

	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return insertedID, true, nil
}
