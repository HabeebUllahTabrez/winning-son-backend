package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
)

type ProgressHandler struct {
	db *sqlx.DB
}

func NewProgressHandler(db *sqlx.DB) *ProgressHandler {
	return &ProgressHandler{db: db}
}

type addProgressRequest struct {
	Amount int `json:"amount"`
}

func (h *ProgressHandler) AddProgress(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	var req addProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Amount < 0 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	_, err := h.db.Exec(`INSERT INTO progress_entries (user_id, date, amount) VALUES ($1, $2, $3)
                          ON CONFLICT (user_id, date) DO UPDATE SET amount = progress_entries.amount + EXCLUDED.amount`,
		userID, today, req.Amount)
	if err != nil {
		http.Error(w, "could not save progress", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type summaryResponse struct {
	Today     int `json:"today"`
	ThisWeek  int `json:"thisWeek"`
	ThisMonth int `json:"thisMonth"`
}

func (h *ProgressHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)

	var res summaryResponse
	_ = h.db.Get(&res.Today, `SELECT COALESCE(SUM(amount),0) FROM progress_entries WHERE user_id=$1 AND date=$2`, userID, today)
	_ = h.db.Get(&res.ThisWeek, `SELECT COALESCE(SUM(amount),0) FROM progress_entries WHERE user_id=$1 AND date >= $2`, userID, weekStart)
	_ = h.db.Get(&res.ThisMonth, `SELECT COALESCE(SUM(amount),0) FROM progress_entries WHERE user_id=$1 AND date >= $2`, userID, monthStart)

	json.NewEncoder(w).Encode(res)
}

type reportEntry struct {
	Date   string `json:"date"`
	Amount int    `json:"amount"`
}

func (h *ProgressHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	rows, err := h.db.Queryx(`SELECT date, amount FROM progress_entries WHERE user_id=$1 ORDER BY date DESC LIMIT 30`, userID)
	if err != nil {
		http.Error(w, "could not fetch report", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var out []reportEntry
	for rows.Next() {
		var d time.Time
		var a int
		if err := rows.Scan(&d, &a); err == nil {
			out = append(out, reportEntry{Date: d.Format("2006-01-02"), Amount: a})
		}
	}
	json.NewEncoder(w).Encode(out)
}
