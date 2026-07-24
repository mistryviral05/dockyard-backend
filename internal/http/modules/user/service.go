package user

import "context"

type Service struct {
	repo Repository
}

func newService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) createUser(ctx context.Context, dto *CreateUserDto) error {
	return s.repo.create(ctx, dto)
}
