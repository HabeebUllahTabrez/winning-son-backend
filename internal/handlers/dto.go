package handlers

import (
	"time"

	"winsonin/internal/models"
)

// UserDTO ensures date-only strings for start_date and end_date
// and a consistent created_at string
type UserDTO struct {
	ID                  int     `json:"id"`
	Email               string  `json:"email"`
	CreatedAt           string  `json:"created_at"`
	FirstName           *string `json:"first_name,omitempty"`
	LastName            *string `json:"last_name,omitempty"`
	AvatarID            *int    `json:"avatar_id,omitempty"`
	Goal                *string `json:"goal,omitempty"`
	StartDate           *string `json:"start_date,omitempty"`
	EndDate             *string `json:"end_date,omitempty"`
	IsAdmin             bool    `json:"is_admin"`
	HasCreatedFirstLog  bool    `json:"has_created_first_log"`
	FirstLogCreatedAt   *string `json:"first_log_created_at,omitempty"`
	HasUsedAnalyzer     bool    `json:"has_used_analyzer"`
	FirstAnalyzerUsedAt *string `json:"first_analyzer_used_at,omitempty"`
}

func toDateStringPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func toDateTimeStringPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// ToUserDTO converts User and optional Goal to UserDTO
func ToUserDTO(u models.User, g *models.Goal) UserDTO {
	created := u.CreatedAt.Format(time.RFC3339)
	dto := UserDTO{
		ID:                  u.ID,
		Email:               u.Email,
		CreatedAt:           created,
		FirstName:           u.FirstName,
		LastName:            u.LastName,
		AvatarID:            u.AvatarID,
		IsAdmin:             u.IsAdmin,
		HasCreatedFirstLog:  u.HasCreatedFirstLog,
		FirstLogCreatedAt:   toDateTimeStringPtr(u.FirstLogCreatedAt),
		HasUsedAnalyzer:     u.HasUsedAnalyzer,
		FirstAnalyzerUsedAt: toDateTimeStringPtr(u.FirstAnalyzerUsedAt),
	}

	// Add goal fields if goal exists
	if g != nil {
		dto.Goal = &g.Goal
		dto.StartDate = toDateStringPtr(g.StartDate)
		dto.EndDate = toDateStringPtr(g.EndDate)
	}

	return dto
}
