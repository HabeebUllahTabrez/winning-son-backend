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
	if err := h.db.Get(&u, `SELECT id, email, email_blind_index, password_hash, created_at, first_name, last_name, avatar_id, goal, start_date, end_date, is_admin FROM users WHERE id=$1`, userID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// Decrypt sensitive fields
	if err := h.encSvc.DecryptUser(&u); err != nil {
		http.Error(w, "could not decrypt user data", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ToUserDTO(u))
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

	// Build dynamic update
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
	if body.Goal != nil {
		// Encrypt goal before storing
		tempUser := models.User{Goal: body.Goal}
		if err := h.encSvc.EncryptUser(&tempUser); err != nil {
			http.Error(w, "could not encrypt goal", http.StatusInternalServerError)
			return
		}
		setClauses = append(setClauses, "goal=$"+itoa(argIdx))
		args = append(args, tempUser.Goal)
		argIdx++
	}
	if body.StartDate != nil {
		if *body.StartDate == "" {
			setClauses = append(setClauses, "start_date=NULL")
		} else {
			if _, err := time.Parse("2006-01-02", *body.StartDate); err != nil {
				http.Error(w, "invalid start_date; expected YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			setClauses = append(setClauses, "start_date=$"+itoa(argIdx))
			args = append(args, *body.StartDate)
			argIdx++
		}
	}
	if body.EndDate != nil {
		if *body.EndDate == "" {
			setClauses = append(setClauses, "end_date=NULL")
		} else {
			if _, err := time.Parse("2006-01-02", *body.EndDate); err != nil {
				http.Error(w, "invalid end_date; expected YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			setClauses = append(setClauses, "end_date=$"+itoa(argIdx))
			args = append(args, *body.EndDate)
			argIdx++
		}
	}
	// is_admin only allowed to be updated if explicitly provided; keep simple for now
	if body.IsAdmin != nil {
		setClauses = append(setClauses, "is_admin=$"+itoa(argIdx))
		args = append(args, *body.IsAdmin)
		argIdx++
	}
	if len(setClauses) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	query := "UPDATE users SET " + join(setClauses, ", ") + " WHERE id=$" + itoa(argIdx)
	args = append(args, userID)
	if _, err := h.db.Exec(query, args...); err != nil {
		http.Error(w, "could not update", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
