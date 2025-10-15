package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"winsonin/internal/models"
	"winsonin/internal/services"

	"github.com/jmoiron/sqlx"
)

type UserHandler struct {
	db     *sqlx.DB
	encSvc *services.EncryptionService
}

func NewUserHandler(db *sqlx.DB, encSvc *services.EncryptionService) *UserHandler {
	return &UserHandler{db: db, encSvc: encSvc}
}

// GetMe returns the current user's profile
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	var u models.User
	if err := h.db.Get(&u, `SELECT id, email, email_blind_index, password_hash, created_at, first_name, last_name, avatar_id, is_admin, has_created_first_log, first_log_created_at, has_used_analyzer, first_analyzer_used_at FROM users WHERE id=$1`, userID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// Decrypt sensitive fields
	if err := h.encSvc.DecryptUser(&u); err != nil {
		http.Error(w, "could not decrypt user data", http.StatusInternalServerError)
		return
	}

	// Fetch user's goal if it exists
	var goal models.Goal
	var goalPtr *models.Goal
	err := h.db.Get(&goal, `SELECT id, user_id, goal, start_date, end_date, created_at, updated_at FROM goals WHERE user_id=$1`, userID)
	if err == nil {
		// Goal exists, decrypt it
		if err := h.encSvc.DecryptGoal(&goal); err != nil {
			http.Error(w, "could not decrypt goal data", http.StatusInternalServerError)
			return
		}
		goalPtr = &goal
	}
	// If error is sql.ErrNoRows, goalPtr remains nil which is fine

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ToUserDTO(u, goalPtr))
}

// GetFeatureStatus returns onboarding/feature completion status for the current user
func (h *UserHandler) GetFeatureStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)

	var status struct {
		HasCreatedFirstLog  bool       `json:"has_created_first_log"`
		FirstLogCreatedAt   *time.Time `json:"first_log_created_at,omitempty"`
		HasUsedAnalyzer     bool       `json:"has_used_analyzer"`
		FirstAnalyzerUsedAt *time.Time `json:"first_analyzer_used_at,omitempty"`
	}

	err := h.db.QueryRow(`
		SELECT has_created_first_log, first_log_created_at, has_used_analyzer, first_analyzer_used_at
		FROM users
		WHERE id = $1`, userID).Scan(
		&status.HasCreatedFirstLog,
		&status.FirstLogCreatedAt,
		&status.HasUsedAnalyzer,
		&status.FirstAnalyzerUsedAt,
	)

	if err != nil {
		http.Error(w, "could not fetch feature status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// UpdateMe updates provided fields on the current user's profile
func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	var body struct {
		FirstName *string `json:"first_name"`
		LastName  *string `json:"last_name"`
		AvatarID  *int    `json:"avatar_id"`
		Goal      *string `json:"goal"`
		StartDate *string `json:"start_date"` // YYYY-MM-DD
		EndDate   *string `json:"end_date"`   // YYYY-MM-DD
		IsAdmin   *bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Build dynamic update for user table
	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1
	if body.FirstName != nil {
		setClauses = append(setClauses, "first_name=$"+itoa(argIdx))
		args = append(args, *body.FirstName)
		argIdx++
	}
	if body.LastName != nil {
		setClauses = append(setClauses, "last_name=$"+itoa(argIdx))
		args = append(args, *body.LastName)
		argIdx++
	}
	if body.AvatarID != nil {
		setClauses = append(setClauses, "avatar_id=$"+itoa(argIdx))
		args = append(args, *body.AvatarID)
		argIdx++
	}
	// is_admin only allowed to be updated if explicitly provided; keep simple for now
	if body.IsAdmin != nil {
		setClauses = append(setClauses, "is_admin=$"+itoa(argIdx))
		args = append(args, *body.IsAdmin)
		argIdx++
	}
	if len(setClauses) > 0 {
		query := "UPDATE users SET " + join(setClauses, ", ") + " WHERE id=$" + itoa(argIdx)
		args = append(args, userID)
		if _, err := h.db.Exec(query, args...); err != nil {
			http.Error(w, "could not update", http.StatusInternalServerError)
			return
		}
	}

	// Handle goal update separately in goals table
	hasGoalUpdate := body.Goal != nil || body.StartDate != nil || body.EndDate != nil
	if hasGoalUpdate {
		// Parse dates if provided
		var startDate, endDate interface{}
		startDate = nil
		endDate = nil

		if body.StartDate != nil {
			if *body.StartDate == "" {
				startDate = nil
			} else {
				parsed, err := parseDate(*body.StartDate)
				if err != nil {
					http.Error(w, "invalid start_date; expected YYYY-MM-DD", http.StatusBadRequest)
					return
				}
				startDate = parsed
			}
		}
		if body.EndDate != nil {
			if *body.EndDate == "" {
				endDate = nil
			} else {
				parsed, err := parseDate(*body.EndDate)
				if err != nil {
					http.Error(w, "invalid end_date; expected YYYY-MM-DD", http.StatusBadRequest)
					return
				}
				endDate = parsed
			}
		}

		if body.Goal != nil {
			// Encrypt goal before storing
			tempGoal := models.Goal{Goal: *body.Goal}
			if err := h.encSvc.EncryptGoal(&tempGoal); err != nil {
				http.Error(w, "could not encrypt goal", http.StatusInternalServerError)
				return
			}

			// Upsert goal with dates
			if body.StartDate != nil && body.EndDate != nil {
				_, err := h.db.Exec(`
					INSERT INTO goals (user_id, goal, start_date, end_date, created_at, updated_at)
					VALUES ($1, $2, $3, $4, NOW(), NOW())
					ON CONFLICT (user_id) DO UPDATE
					SET goal = EXCLUDED.goal,
						start_date = EXCLUDED.start_date,
						end_date = EXCLUDED.end_date,
						updated_at = NOW()`,
					userID, tempGoal.Goal, startDate, endDate)
				if err != nil {
					http.Error(w, "could not save goal", http.StatusInternalServerError)
					return
				}
			} else if body.StartDate != nil {
				_, err := h.db.Exec(`
					INSERT INTO goals (user_id, goal, start_date, created_at, updated_at)
					VALUES ($1, $2, $3, NOW(), NOW())
					ON CONFLICT (user_id) DO UPDATE
					SET goal = EXCLUDED.goal,
						start_date = EXCLUDED.start_date,
						updated_at = NOW()`,
					userID, tempGoal.Goal, startDate)
				if err != nil {
					http.Error(w, "could not save goal", http.StatusInternalServerError)
					return
				}
			} else if body.EndDate != nil {
				_, err := h.db.Exec(`
					INSERT INTO goals (user_id, goal, end_date, created_at, updated_at)
					VALUES ($1, $2, $3, NOW(), NOW())
					ON CONFLICT (user_id) DO UPDATE
					SET goal = EXCLUDED.goal,
						end_date = EXCLUDED.end_date,
						updated_at = NOW()`,
					userID, tempGoal.Goal, endDate)
				if err != nil {
					http.Error(w, "could not save goal", http.StatusInternalServerError)
					return
				}
			} else {
				_, err := h.db.Exec(`
					INSERT INTO goals (user_id, goal, created_at, updated_at)
					VALUES ($1, $2, NOW(), NOW())
					ON CONFLICT (user_id) DO UPDATE
					SET goal = EXCLUDED.goal,
						updated_at = NOW()`,
					userID, tempGoal.Goal)
				if err != nil {
					http.Error(w, "could not save goal", http.StatusInternalServerError)
					return
				}
			}
		} else {
			// Only updating dates, not goal text
			if body.StartDate != nil && body.EndDate != nil {
				_, err := h.db.Exec(`UPDATE goals SET start_date = $1, end_date = $2, updated_at = NOW() WHERE user_id = $3`, startDate, endDate, userID)
				if err != nil {
					http.Error(w, "could not update goal dates", http.StatusInternalServerError)
					return
				}
			} else if body.StartDate != nil {
				_, err := h.db.Exec(`UPDATE goals SET start_date = $1, updated_at = NOW() WHERE user_id = $2`, startDate, userID)
				if err != nil {
					http.Error(w, "could not update start_date", http.StatusInternalServerError)
					return
				}
			} else if body.EndDate != nil {
				_, err := h.db.Exec(`UPDATE goals SET end_date = $1, updated_at = NOW() WHERE user_id = $2`, endDate, userID)
				if err != nil {
					http.Error(w, "could not update end_date", http.StatusInternalServerError)
					return
				}
			}
		}
	}

	if len(setClauses) == 0 && !hasGoalUpdate {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseDate(dateStr string) (string, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return "", err
	}
	return t.Format("2006-01-02"), nil
}

// Minimal helpers to avoid bringing another package just for this
func itoa(i int) string { return fmt.Sprintf("%d", i) }
func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += sep + parts[i]
	}
	return out
}
