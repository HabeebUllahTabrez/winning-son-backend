package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"winsonin/internal/models"
	"winsonin/internal/services"

	"github.com/jmoiron/sqlx"
)

type MigrateHandler struct {
	db     *sqlx.DB
	encSvc *services.EncryptionService
}

func NewMigrateHandler(db *sqlx.DB, encSvc *services.EncryptionService) *MigrateHandler {
	return &MigrateHandler{db: db, encSvc: encSvc}
}

type UserProfileData struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	AvatarID  *int    `json:"avatar_id"`
	Goal      *string `json:"goal"`
	StartDate *string `json:"start_date"` // YYYY-MM-DD
	EndDate   *string `json:"end_date"`   // YYYY-MM-DD
}

type MigratedJournalEntry struct {
	Topics            string `json:"topics"`
	AlignmentRating   int    `json:"alignment_rating"`
	ContentmentRating int    `json:"contentment_rating"`
	LocalDate         string `json:"local_date"` // YYYY-MM-DD
}

type MigrateRequest struct {
	Entries []MigratedJournalEntry `json:"entries"`
	Profile *UserProfileData       `json:"profile"`
}

// MigrateData godoc
// @Summary Migrate user data
// @Description Receives a list of journal entries and/or user profile data and upserts them for the authenticated user
// @Tags migrate
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param data body MigrateRequest true "Migration data"
// @Success 201 {object} map[string]interface{} "Data migrated successfully"
// @Failure 400 {string} string "Bad request"
// @Failure 500 {string} string "Internal server error"
// @Router /migrate [post]
func (h *MigrateHandler) MigrateData(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)

	var req MigrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Entries) == 0 && req.Profile == nil {
		http.Error(w, "no entries or profile data provided", http.StatusBadRequest)
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		http.Error(w, "could not start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback() // Rollback on any error.

	// Handle profile update
	if req.Profile != nil {
		// Update user fields (non-goal fields)
		userSetClauses := []string{}
		userArgs := []interface{}{}
		userArgIdx := 1

		if req.Profile.FirstName != nil {
			userSetClauses = append(userSetClauses, fmt.Sprintf("first_name=$%d", userArgIdx))
			userArgs = append(userArgs, *req.Profile.FirstName)
			userArgIdx++
		}
		if req.Profile.LastName != nil {
			userSetClauses = append(userSetClauses, fmt.Sprintf("last_name=$%d", userArgIdx))
			userArgs = append(userArgs, *req.Profile.LastName)
			userArgIdx++
		}
		if req.Profile.AvatarID != nil {
			userSetClauses = append(userSetClauses, fmt.Sprintf("avatar_id=$%d", userArgIdx))
			userArgs = append(userArgs, *req.Profile.AvatarID)
			userArgIdx++
		}

		if len(userSetClauses) > 0 {
			query := "UPDATE users SET " + strings.Join(userSetClauses, ", ") + fmt.Sprintf(" WHERE id=$%d", userArgIdx)
			userArgs = append(userArgs, userID)
			if _, err := tx.Exec(query, userArgs...); err != nil {
				http.Error(w, "could not update user profile", http.StatusInternalServerError)
				return
			}
		}

		// Handle goal separately - upsert into goals table
		if req.Profile.Goal != nil && *req.Profile.Goal != "" {
			// Parse dates if provided
			var startDate, endDate *time.Time
			if req.Profile.StartDate != nil && *req.Profile.StartDate != "" {
				parsed, err := time.Parse("2006-01-02", *req.Profile.StartDate)
				if err != nil {
					http.Error(w, "invalid start_date; expected YYYY-MM-DD", http.StatusBadRequest)
					return
				}
				startDate = &parsed
			}
			if req.Profile.EndDate != nil && *req.Profile.EndDate != "" {
				parsed, err := time.Parse("2006-01-02", *req.Profile.EndDate)
				if err != nil {
					http.Error(w, "invalid end_date; expected YYYY-MM-DD", http.StatusBadRequest)
					return
				}
				endDate = &parsed
			}

			// Encrypt goal before storing
			tempGoal := models.Goal{Goal: *req.Profile.Goal}
			if err := h.encSvc.EncryptGoal(&tempGoal); err != nil {
				http.Error(w, "could not encrypt goal", http.StatusInternalServerError)
				return
			}

			// Upsert goal
			_, err := tx.Exec(`
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
		}
	}

	if len(req.Entries) > 0 {
		stmt, err := tx.Prepare(`INSERT INTO journal_entries (user_id, local_date, topics, alignment_rating, contentment_rating, updated_at) 
								 VALUES ($1, $2, $3, $4, $5, NOW())
								 ON CONFLICT (user_id, local_date) 
								 DO UPDATE SET 
								   topics = EXCLUDED.topics, 
								   alignment_rating = EXCLUDED.alignment_rating,
								   contentment_rating = EXCLUDED.contentment_rating,
								   updated_at = NOW()`)
		if err != nil {
			http.Error(w, "could not prepare statement", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		for _, entry := range req.Entries {
			if entry.Topics == "" || entry.AlignmentRating < 1 || entry.AlignmentRating > 10 || entry.ContentmentRating < 1 || entry.ContentmentRating > 10 || entry.LocalDate == "" {
				http.Error(w, fmt.Sprintf("invalid entry data: %+v", entry), http.StatusBadRequest)
				return
			}

			parsedLocalDate, err := time.Parse("2006-01-02", entry.LocalDate)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid local_date format for entry: %s", entry.LocalDate), http.StatusBadRequest)
				return
			}

			// Encrypt topics before storing
			tempJournal := models.Journal{Topics: entry.Topics}
			if err := h.encSvc.EncryptJournal(&tempJournal); err != nil {
				http.Error(w, "could not encrypt topics", http.StatusInternalServerError)
				return
			}

			if _, err := stmt.Exec(userID, parsedLocalDate, tempJournal.Topics, entry.AlignmentRating, entry.ContentmentRating); err != nil {
				http.Error(w, "could not save entry", http.StatusInternalServerError)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "could not commit transaction", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message": "Data migrated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}
