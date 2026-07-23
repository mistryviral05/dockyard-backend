package user

import (
	"context"
	"database/sql"
)

type Repository interface {
	Create(ctx context.Context, dto CreateUserDto) error
}

type repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) Create(ctx context.Context, dto CreateUserDto) error {

	q := "INSERT INTO users (clerk_id,email,name) VALUES (?,?,?)"

	stmt, err := r.db.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, dto.ClerkId, dto.Email, dto.Name)
	if err != nil {
		return err
	}

	return nil
}
