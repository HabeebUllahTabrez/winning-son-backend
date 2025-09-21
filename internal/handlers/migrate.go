package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type MigrateHandler struct {
	db *sqlx.DB
}

func NewMigrateHandler(db *sqlx.DB) *MigrateHandler {
	return &MigrateHandler{db: db}
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

// MigrateData receives a list of journal entries and/or user profile data and upserts them for the authenticated user.
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
		setClauses := []string{}
		args := []interface{}{}
		argIdx := 1

		if req.Profile.FirstName != nil {
			setClauses = append(setClauses, fmt.Sprintf("first_name=$%d", argIdx))
			args = append(args, *req.Profile.FirstName)
			argIdx++
		}
		if req.Profile.LastName != nil {
			setClauses = append(setClauses, fmt.Sprintf("last_name=$%d", argIdx))
			args = append(args, *req.Profile.LastName)
			argIdx++
		}
		if req.Profile.AvatarID != nil {
			setClauses = append(setClauses, fmt.Sprintf("avatar_id=$%d", argIdx))
			args = append(args, *req.Profile.AvatarID)
			argIdx++
		}
		if req.Profile.Goal != nil {
			setClauses = append(setClauses, fmt.Sprintf("goal=$%d", argIdx))
			args = append(args, *req.Profile.Goal)
			argIdx++
		}
		if req.Profile.StartDate != nil {
			if *req.Profile.StartDate == "" {
				setClauses = append(setClauses, "start_date=NULL")
			} else {
				if _, err := time.Parse("2006-01-02", *req.Profile.StartDate); err != nil {
					http.Error(w, "invalid start_date; expected YYYY-MM-DD", http.StatusBadRequest)
					return
				}
				setClauses = append(setClauses, fmt.Sprintf("start_date=$%d", argIdx))
				args = append(args, *req.Profile.StartDate)
				argIdx++
			}
		}
		if req.Profile.EndDate != nil {
			if *req.Profile.EndDate == "" {
				setClauses = append(setClauses, "end_date=NULL")
			} else {
				if _, err := time.Parse("2006-01-02", *req.Profile.EndDate); err != nil {
					http.Error(w, "invalid end_date; expected YYYY-MM-DD", http.StatusBadRequest)
					return
				}
				setClauses = append(setClauses, fmt.Sprintf("end_date=$%d", argIdx))
				args = append(args, *req.Profile.EndDate)
				argIdx++
			}
		}

		if len(setClauses) > 0 {
			query := "UPDATE users SET " + strings.Join(setClauses, ", ") + fmt.Sprintf(" WHERE id=$%d", argIdx)
			args = append(args, userID)
			if _, err := tx.Exec(query, args...); err != nil {
				http.Error(w, "could not update user profile", http.StatusInternalServerError)
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

			if _, err := stmt.Exec(userID, parsedLocalDate, entry.Topics, entry.AlignmentRating, entry.ContentmentRating); err != nil {
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
