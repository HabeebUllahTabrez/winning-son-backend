package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
)

type JournalHandler struct {
	db *sqlx.DB
}

func NewJournalHandler(db *sqlx.DB) *JournalHandler { return &JournalHandler{db: db} }

type journalRequest struct {
	Topics    string `json:"topics"`
	Rating    int    `json:"rating"`
	LocalDate string `json:"local_date"` // YYYY-MM-DD provided by frontend
}

// UpsertEntry creates a new journal entry or updates an existing one for the same user and local date
func (h *JournalHandler) UpsertEntry(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	var req journalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Topics == "" || req.Rating < 1 || req.Rating > 10 || req.LocalDate == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Parse the provided local_date (YYYY-MM-DD)
	parsedLocalDate, err := time.Parse("2006-01-02", req.LocalDate)
	if err != nil {
		http.Error(w, "invalid local_date format; expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	// Use UPSERT to either insert new entry or update existing one
	var isUpdate bool
	err = h.db.QueryRow(`INSERT INTO journal_entries (user_id, local_date, topics, rating, updated_at) 
	                      VALUES ($1, $2, $3, $4, NOW())
	                      ON CONFLICT (user_id, local_date) 
	                      DO UPDATE SET 
	                        topics = EXCLUDED.topics, 
	                        rating = EXCLUDED.rating, 
	                        updated_at = NOW()
	                      RETURNING (xmax = 0)`, userID, parsedLocalDate, req.Topics, req.Rating).Scan(&isUpdate)
	if err != nil {
		http.Error(w, "could not save", http.StatusInternalServerError)
		return
	}

	// Return success with the local date that was used
	response := map[string]interface{}{
		"message":    "Entry saved successfully",
		"local_date": parsedLocalDate.Format("2006-01-02"),
		"is_update":  !isUpdate, // xmax = 0 means it was an INSERT, otherwise UPDATE
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

type journalEntry struct {
	LocalDate string `json:"local_date"`
	Topics    string `json:"topics"`
	Rating    int    `json:"rating"`
}

// Delete removes a journal entry for the authenticated user by local_date (YYYY-MM-DD)
func (h *JournalHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)

	// Expect JSON body: { "local_date": "YYYY-MM-DD" }
	var body struct {
		LocalDate string `json:"local_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.LocalDate == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Parse and validate date
	parsedLocalDate, err := time.Parse("2006-01-02", body.LocalDate)
	if err != nil {
		http.Error(w, "invalid local_date format; expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	res, err := h.db.Exec(`DELETE FROM journal_entries WHERE user_id = $1 AND local_date = $2`, userID, parsedLocalDate)
	if err != nil {
		http.Error(w, "could not delete", http.StatusInternalServerError)
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *JournalHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	// Optional query params: start_date, end_date (YYYY-MM-DD)
	q := r.URL.Query()
	startDateStr := q.Get("start_date")
	endDateStr := q.Get("end_date")

	where := "WHERE user_id=$1"
	args := []interface{}{userID}

	if startDateStr != "" {
		startDate, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			http.Error(w, "invalid start_date format; expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		args = append(args, startDate)
		where += fmt.Sprintf(" AND local_date >= $%d", len(args))
	}

	if endDateStr != "" {
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			http.Error(w, "invalid end_date format; expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		args = append(args, endDate)
		where += fmt.Sprintf(" AND local_date <= $%d", len(args))
	}

	query := "SELECT local_date, topics, rating FROM journal_entries " + where + " ORDER BY local_date DESC LIMIT 100"
	rows, err := h.db.Queryx(query, args...)
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
			out = append(out, journalEntry{LocalDate: d.Format("2006-01-02"), Topics: t, Rating: r8})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
