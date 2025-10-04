package models

import "time"

type User struct {
	ID              int        `db:"id" json:"id"`
	Email           string     `db:"email" json:"email"`                         // Encrypted in DB
	EmailBlindIndex string     `db:"email_blind_index" json:"-"`                 // HMAC hash for searching
	PasswordHash    string     `db:"password_hash" json:"-"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	FirstName       *string    `db:"first_name" json:"first_name,omitempty"`
	LastName        *string    `db:"last_name" json:"last_name,omitempty"`
	AvatarID        *int       `db:"avatar_id" json:"avatar_id,omitempty"`
	Goal            *string    `db:"goal" json:"goal,omitempty"`                 // Encrypted in DB
	StartDate       *time.Time `db:"start_date" json:"start_date,omitempty"`
	EndDate         *time.Time `db:"end_date" json:"end_date,omitempty"`
	IsAdmin         bool       `db:"is_admin" json:"is_admin"`
}

type Journal struct {
	ID                int       `db:"id" json:"id"`
	UserID            int       `db:"user_id" json:"user_id"`
	LocalDate         string    `db:"local_date" json:"local_date"`
	Topics            string    `db:"topics" json:"topics"`                       // Encrypted in DB
	AlignmentRating   int       `db:"alignment_rating" json:"alignment_rating"`
	ContentmentRating int       `db:"contentment_rating" json:"contentment_rating"`
	Karma             float64   `db:"karma" json:"karma"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
}
