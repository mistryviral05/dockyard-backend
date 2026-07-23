package user

import "context"

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) CreateUser(ctx context.Context,dto CreateUserDto) error {
	return s.repo.Create(ctx,dto)
}
