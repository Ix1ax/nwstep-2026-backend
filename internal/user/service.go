package user

import (
	"context"
	"errors"
	"time"
)

type UserService struct {
	repo *UserRepository
}

func NewUserService(repo *UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) UpdateUser(ctx context.Context, id string, req UpdateUserRequest) (*User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}
	user.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) ListUsers(ctx context.Context, query PaginationQuery) ([]User, int, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 {
		query.Limit = 10
	}
	
	offset := (query.Page - 1) * query.Limit
	
	users, err := s.repo.List(ctx, query.Limit, offset)
	if err != nil {
		return nil, 0, err
	}
	
	count, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	
	return users, count, nil
}
