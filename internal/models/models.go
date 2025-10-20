package models

import "time"

type User struct {
	ID                   int        `db:"id" json:"id"`
	Email                string     `db:"email" json:"email"`                                   // Encrypted in DB
	EmailBlindIndex      string     `db:"email_blind_index" json:"-"`                           // HMAC hash for searching
	PasswordHash         string     `db:"password_hash" json:"-"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
	FirstName            *string    `db:"first_name" json:"first_name,omitempty"`
	LastName             *string    `db:"last_name" json:"last_name,omitempty"`
	AvatarID             *int       `db:"avatar_id" json:"avatar_id,omitempty"`
	IsAdmin              bool       `db:"is_admin" json:"is_admin"`
	HasCreatedFirstLog   bool       `db:"has_created_first_log" json:"has_created_first_log"`
	FirstLogCreatedAt    *time.Time `db:"first_log_created_at" json:"first_log_created_at,omitempty"`
	HasUsedAnalyzer      bool       `db:"has_used_analyzer" json:"has_used_analyzer"`
	FirstAnalyzerUsedAt  *time.Time `db:"first_analyzer_used_at" json:"first_analyzer_used_at,omitempty"`
}

type Goal struct {
	ID        int        `db:"id" json:"id"`
	UserID    int        `db:"user_id" json:"user_id"`
	Goal      string     `db:"goal" json:"goal"`                       // Encrypted in DB
	StartDate *time.Time `db:"start_date" json:"start_date,omitempty"`
	EndDate   *time.Time `db:"end_date" json:"end_date,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
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

type UserStats struct {
	UserID                   int        `db:"user_id" json:"user_id"`
	TotalDaysLogged          int        `db:"total_days_logged" json:"total_days_logged"`
	TotalKarma               float64    `db:"total_karma" json:"total_karma"`
	CurrentStreakDays        int        `db:"current_streak_days" json:"current_streak_days"`
	LongestStreakEver        int        `db:"longest_streak_ever" json:"longest_streak_ever"`
	CurrentStreakStartDate   *time.Time `db:"current_streak_start_date" json:"current_streak_start_date,omitempty"`
	LastEntryDate            *time.Time `db:"last_entry_date" json:"last_entry_date,omitempty"`
	LastWeekKarma            float64    `db:"last_week_karma" json:"last_week_karma"`
	LastWeekStartDate        *time.Time `db:"last_week_start_date" json:"last_week_start_date,omitempty"`
	LastWeekEndDate          *time.Time `db:"last_week_end_date" json:"last_week_end_date,omitempty"`
	DayOfWeekStats           *string    `db:"day_of_week_stats" json:"day_of_week_stats,omitempty"` // JSONB as string
	LastUpdatedAt            time.Time  `db:"last_updated_at" json:"last_updated_at"`
	PositiveDaysCount        int        `db:"positive_days_count" json:"positive_days_count"`
	ComebackCount            int        `db:"comeback_count" json:"comeback_count"`
}
