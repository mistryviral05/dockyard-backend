package github

import postgres "github.com/mistryviral05/dockyard/internal/http/database"

type Repository interface {
	WithTx(db postgres.DBTX) Repository // rebinds the repo onto a transaction
	
}

type repository struct {
	db postgres.DBTX // *sql.DB in normal use, *sql.Tx inside a transaction
}

func (r *repository) WithTx(db postgres.DBTX) Repository {
	return &repository{db: db}
}

func NewRepository(db postgres.DBTX) Repository {
	return &repository{db: db}
}
