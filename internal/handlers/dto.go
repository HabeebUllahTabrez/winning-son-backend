package handlers

import (
	"time"

	"winsonin/internal/models"
)

// UserDTO ensures date-only strings for start_date and end_date
// and a consistent created_at string
type UserDTO struct {
	ID        int     `json:"id"`
	Email     string  `json:"email"`
	CreatedAt string  `json:"created_at"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	AvatarID  *int    `json:"avatar_id,omitempty"`
	Goal      *string `json:"goal,omitempty"`
	StartDate *string `json:"start_date,omitempty"`
	EndDate   *string `json:"end_date,omitempty"`
	IsAdmin   bool    `json:"is_admin"`
}

func toDateStringPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func ToUserDTO(u models.User) UserDTO {
	created := u.CreatedAt.Format(time.RFC3339)
	return UserDTO{
		ID:        u.ID,
		Email:     u.Email,
		CreatedAt: created,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		AvatarID:  u.AvatarID,
		Goal:      u.Goal,
		StartDate: toDateStringPtr(u.StartDate),
		EndDate:   toDateStringPtr(u.EndDate),
		IsAdmin:   u.IsAdmin,
	}
}
