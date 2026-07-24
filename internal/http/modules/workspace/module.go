package workspace

import "database/sql"

type Module struct {
	Service *Service
	Repo    Repository
}

func New(db *sql.DB) *Module {
	repo := NewRepository(db)
	service := NewService(db, repo)
	return &Module{
		Service: service,
		Repo:    repo,
	}
}
