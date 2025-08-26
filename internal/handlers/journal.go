package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
)

type JournalHandler struct {
	db *sqlx.DB
}

func NewJournalHandler(db *sqlx.DB) *JournalHandler { return &JournalHandler{db: db} }

type journalRequest struct {
	Topics string `json:"topics"`
	Rating int    `json:"rating"`
}

func (h *JournalHandler) AddToday(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	var req journalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Topics == "" || req.Rating < 1 || req.Rating > 10 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	_, err := h.db.Exec(`INSERT INTO journal_entries (user_id, date, topics, rating) VALUES ($1, $2, $3, $4)
	                      ON CONFLICT (user_id, date) DO UPDATE SET topics = EXCLUDED.topics, rating = EXCLUDED.rating`,
		userID, today, req.Topics, req.Rating)
	if err != nil {
		http.Error(w, "could not save", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type journalEntry struct {
	Date   string `json:"date"`
	Topics string `json:"topics"`
	Rating int    `json:"rating"`
}

func (h *JournalHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	rows, err := h.db.Queryx(`SELECT date, topics, rating FROM journal_entries WHERE user_id=$1 ORDER BY date DESC LIMIT 100`, userID)
	if err != nil {
		http.Error(w, "could not fetch", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var out []journalEntry
	for rows.Next() {
		var d time.Time
		var t string
		var r8 int
		if err := rows.Scan(&d, &t, &r8); err == nil {
			out = append(out, journalEntry{Date: d.Format("2006-01-02"), Topics: t, Rating: r8})
		}
	}
	json.NewEncoder(w).Encode(out)
}
