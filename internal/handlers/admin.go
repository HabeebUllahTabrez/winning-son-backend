package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/jmoiron/sqlx"
)

type AdminHandler struct {
	db *sqlx.DB
}

func NewAdminHandler(db *sqlx.DB) *AdminHandler { return &AdminHandler{db: db} }

type adminOverview struct {
	TotalUsers          int `json:"total_users"`
	TotalJournalEntries int `json:"total_journal_entries"`
	ActiveUsersThisWeek int `json:"active_users_this_week"`
	EntriesThisWeek     int `json:"entries_this_week"`
	EntriesThisMonth    int `json:"entries_this_month"`
}

// mustBeAdmin checks the current user is admin
func (h *AdminHandler) mustBeAdmin(userID int) (bool, error) {
	var isAdmin bool
	if err := h.db.QueryRowx(`SELECT is_admin FROM users WHERE id=$1`, userID).Scan(&isAdmin); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return isAdmin, nil
}

// Overview godoc
// @Summary Get admin overview
// @Description Returns administrative statistics and metrics (admin only)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} adminOverview
// @Failure 403 {string} string "Forbidden"
// @Failure 500 {string} string "Internal server error"
// @Router /admin/overview [get]
func (h *AdminHandler) Overview(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	if ok, err := h.mustBeAdmin(userID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var out adminOverview
	if err := h.db.QueryRowx(`SELECT COUNT(*) FROM users`).Scan(&out.TotalUsers); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := h.db.QueryRowx(`SELECT COUNT(*) FROM journal_entries`).Scan(&out.TotalJournalEntries); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := h.db.QueryRowx(`SELECT COUNT(DISTINCT user_id) FROM journal_entries WHERE local_date >= date_trunc('week', CURRENT_DATE) AND local_date <= CURRENT_DATE`).Scan(&out.ActiveUsersThisWeek); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := h.db.QueryRowx(`SELECT COUNT(*) FROM journal_entries WHERE local_date >= date_trunc('week', CURRENT_DATE) AND local_date <= CURRENT_DATE`).Scan(&out.EntriesThisWeek); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := h.db.QueryRowx(`SELECT COUNT(*) FROM journal_entries WHERE date_trunc('month', local_date) = date_trunc('month', CURRENT_DATE)`).Scan(&out.EntriesThisMonth); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
