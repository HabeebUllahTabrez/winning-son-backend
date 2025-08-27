package models

import "time"

type User struct {
	ID           int        `db:"id" json:"id"`
	Email        string     `db:"email" json:"email"`
	PasswordHash string     `db:"password_hash" json:"-"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	FirstName    *string    `db:"first_name" json:"first_name,omitempty"`
	LastName     *string    `db:"last_name" json:"last_name,omitempty"`
	AvatarID     *int       `db:"avatar_id" json:"avatar_id,omitempty"`
	Goal         *string    `db:"goal" json:"goal,omitempty"`
	StartDate    *time.Time `db:"start_date" json:"start_date,omitempty"`
	EndDate      *time.Time `db:"end_date" json:"end_date,omitempty"`
	IsAdmin      bool       `db:"is_admin" json:"is_admin"`
}
