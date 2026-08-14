package deployment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	postgres "github.com/mistryviral05/dockyard/internal/http/database"
)

const (
	ErrInvalidTransition = "invalid transition"
)

type Repository interface {
	WithTx(db postgres.DBTX) Repository
}

type repository struct {
	db postgres.DBTX // *sql.DB in normal use, *sql.Tx inside a transaction
}

func (r *repository) WithTx(db postgres.DBTX) Repository {
	return &repository{db: db}
}

func NewRepository(db postgres.DBTX) Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) Transition(ctx context.Context, id, to string, extra map[string]interface{}) error {

	from, ok := PredecessorsOf(to)

	if !ok {
		// Handle invalid transition
		return errors.New(ErrInvalidTransition)
	}

	q := `UPDATE deployments
		  SET status = $1, updated_at = NOW()
		  WHERE id = $2 AND status = ANY($3)
		  RETURNING id`

	var returned string
	err := r.db.QueryRowContext(ctx, q, to, id, from).Scan(&returned)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("failed to transition deployment: %w", err)
	}

	return nil
}
