package user

import "database/sql"

func New(db *sql.DB) *Handler {
	repo := NewRepository(db)
	service := NewService(repo)
	handler := NewHandler(service)
	return handler
}
