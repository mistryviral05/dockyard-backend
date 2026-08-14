package deployment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
	postgres "github.com/mistryviral05/dockyard/internal/http/database"
)

var updatableColumns = []string{
	"agent_id",
	"image_tag",
	"container_id",
	"error_message",
}

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
		return errors.New(ErrInvalidTransition)
	}

	sets := []string{"status = $1", "updated_at = NOW()"}
	args := []interface{}{to}

	switch {
	case to == StatusBuilding:
		sets = append(sets, "started_at = COALESCE(started_at, NOW())")
	case IsTerminal(to):
		sets = append(sets, "completed_at = NOW()")
	}

	for _, col := range updatableColumns {
		val, ok := extra[col]
		if !ok {
			continue
		}
		args = append(args, val)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}

	args = append(args, id)
	idPos := len(args)

	args = append(args, pq.Array(from))
	fromPos := len(args)

	q := fmt.Sprintf(`UPDATE deployments
		  SET %s
		  WHERE id = $%d AND status = ANY($%d)
		  RETURNING id`, strings.Join(sets, ", "), idPos, fromPos)

	var returned string
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&returned)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("failed to transition deployment: %w", err)
	}

	return nil
}
