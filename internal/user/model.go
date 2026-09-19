package user

import "time"

type User struct {
	ID        string    `db:"id" json:"id"`
	Email     string    `db:"email" json:"email"`
	Username  string    `db:"username" json:"username"`
	AvatarURL *string   `db:"avatar_url" json:"avatar_url,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type UpdateUserRequest struct {
	Username  *string `json:"username,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type PaginationQuery struct {
	Page  int `query:"page" default:"1"`
	Limit int `query:"limit" default:"10"`
}
