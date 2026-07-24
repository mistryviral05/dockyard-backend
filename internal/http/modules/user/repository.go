package user

import (
	"context"
	"database/sql"

	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

type Repository interface {
	create(ctx context.Context, dto *CreateUserDto) error
}

type repository struct {
	db *sql.DB
}

func newRepository(db *sql.DB) Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) create(ctx context.Context, dto *CreateUserDto) error {

	q := `
	INSERT INTO users (id,clerk_id, email, name)
	VALUES ($1, $2, $3,$4)
	`

	stmt, err := r.db.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer stmt.Close()
	userId := id.UserID()
	_, err = stmt.ExecContext(ctx, userId, dto.ClerkId, dto.Email, dto.Name)
	if err != nil {
		return err
	}

	return nil
}
