package workspace

import "database/sql"

type Service struct {
	db   *sql.DB
	repo Repository
}

func NewService(db *sql.DB, repo Repository) *Service {
	return &Service{
		db:   db,
		repo: repo,
	}
}
