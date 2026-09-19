package user

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
	var user User
	query := `SELECT id, email, username, avatar_url, created_at, updated_at FROM users WHERE id = $1`
	err := r.db.GetContext(ctx, &user, query, id)
	return &user, err
}

func (r *UserRepository) Update(ctx context.Context, user *User) error {
	query := `
		UPDATE users 
		SET username = :username, avatar_url = :avatar_url, updated_at = :updated_at
		WHERE id = :id
	`
	_, err := r.db.NamedExecContext(ctx, query, user)
	return err
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]User, error) {
	var users []User
	query := `SELECT id, email, username, avatar_url, created_at, updated_at FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	err := r.db.SelectContext(ctx, &users, query, limit, offset)
	return users, err
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM users`
	err := r.db.GetContext(ctx, &count, query)
	return count, err
}
