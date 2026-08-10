package agent

import "database/sql"

type Service struct {
	db   *sql.DB
	repo Repository
}

func NewService(repo Repository, db *sql.DB) *Service {

	return &Service{repo: repo, db: db}
}
