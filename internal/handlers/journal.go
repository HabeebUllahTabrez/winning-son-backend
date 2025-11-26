package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"winsonin/internal/models"
	"winsonin/internal/services"

	"github.com/jmoiron/sqlx"
)

type JournalHandler struct {
	db     *sqlx.DB
	encSvc *services.EncryptionService
}

func NewJournalHandler(db *sqlx.DB, encSvc *services.EncryptionService) *JournalHandler {
	return &JournalHandler{db: db, encSvc: encSvc}
}

type journalRequest struct {
	Topics            string `json:"topics"`
	AlignmentRating   int    `json:"alignment_rating"`
	ContentmentRating int    `json:"contentment_rating"`
	LocalDate         string `json:"local_date"` // YYYY-MM-DD provided by frontend
}

// UpsertEntry creates a new journal entry or updates an existing one for the same user and local date
func (h *JournalHandler) UpsertEntry(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	var req journalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Topics == "" || req.AlignmentRating < 1 || req.AlignmentRating > 10 || req.ContentmentRating < 1 || req.ContentmentRating > 10 || req.LocalDate == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Parse the provided local_date (YYYY-MM-DD)
	parsedLocalDate, err := time.Parse("2006-01-02", req.LocalDate)
	if err != nil {
		http.Error(w, "invalid local_date format; expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	// Calculate Karma
	karma := (float64(req.AlignmentRating) + float64(req.ContentmentRating) - 2) / 18.0

	// Encrypt topics before storing
	tempJournal := models.Journal{Topics: req.Topics}
	if err := h.encSvc.EncryptJournal(&tempJournal); err != nil {
		http.Error(w, "could not encrypt topics", http.StatusInternalServerError)
		return
	}

	// Start transaction to keep user_stats in sync with journal_entries
	tx, err := h.db.Beginx()
	if err != nil {
		http.Error(w, "could not start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Get old values if this is an update (for incremental stats adjustment)
	var oldKarma sql.NullFloat64
	_ = tx.QueryRow(`SELECT karma FROM journal_entries WHERE user_id=$1 AND local_date=$2`,
		userID, parsedLocalDate).Scan(&oldKarma)

	// Use UPSERT to either insert new entry or update existing one
	var isUpdate bool
	err = tx.QueryRow(`INSERT INTO journal_entries (user_id, local_date, topics, alignment_rating, contentment_rating, karma, updated_at)
	                      VALUES ($1, $2, $3, $4, $5, $6, NOW())
	                      ON CONFLICT (user_id, local_date)
	                      DO UPDATE SET
	                        topics = EXCLUDED.topics,
	                        alignment_rating = EXCLUDED.alignment_rating,
							contentment_rating = EXCLUDED.contentment_rating,
							karma = EXCLUDED.karma,
	                        updated_at = NOW()
	                      RETURNING (xmax = 0)`, userID, parsedLocalDate, tempJournal.Topics, req.AlignmentRating, req.ContentmentRating, karma).Scan(&isUpdate)
	if err != nil {
		http.Error(w, "could not save", http.StatusInternalServerError)
		return
	}

	// Update user_stats table incrementally
	if !isUpdate { // This is an INSERT (new entry)
		err = h.updateUserStatsOnInsert(tx, userID, parsedLocalDate, karma)
	} else { // This is an UPDATE (existing entry)
		err = h.updateUserStatsOnUpdate(tx, userID, parsedLocalDate, oldKarma.Float64, karma)
	}
	if err != nil {
		http.Error(w, "could not update stats", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "could not commit transaction", http.StatusInternalServerError)
		return
	}

	// Track first log creation for this user
	_, err = h.db.Exec(`
		UPDATE users
		SET has_created_first_log = true,
		    first_log_created_at = COALESCE(first_log_created_at, NOW())
		WHERE id = $1 AND has_created_first_log = false`, userID)
	if err != nil {
		// Log error but don't fail the request
		// The journal entry was successfully saved
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
	LocalDate         string  `json:"local_date"`
	Topics            string  `json:"topics"`
	AlignmentRating   int     `json:"alignment_rating"`
	ContentmentRating int     `json:"contentment_rating"`
	Karma             float64 `json:"karma"`
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

	// Start transaction
	tx, err := h.db.Beginx()
	if err != nil {
		http.Error(w, "could not start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Get the entry data before deleting (to adjust stats)
	var karma float64
	err = tx.QueryRow(`SELECT karma FROM journal_entries WHERE user_id=$1 AND local_date=$2`,
		userID, parsedLocalDate).Scan(&karma)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "could not fetch entry", http.StatusInternalServerError)
		return
	}

	// Delete the entry
	res, err := tx.Exec(`DELETE FROM journal_entries WHERE user_id = $1 AND local_date = $2`, userID, parsedLocalDate)
	if err != nil {
		http.Error(w, "could not delete", http.StatusInternalServerError)
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Update user_stats: decrement counters
	err = h.updateUserStatsOnDelete(tx, userID, parsedLocalDate, karma)
	if err != nil {
		http.Error(w, "could not update stats", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "could not commit transaction", http.StatusInternalServerError)
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

	query := "SELECT local_date, topics, alignment_rating, contentment_rating, karma FROM journal_entries " + where + " ORDER BY local_date DESC LIMIT 100"
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
		var ar int
		var cr int
		var k float64
		if err := rows.Scan(&d, &t, &ar, &cr, &k); err == nil {
			// Decrypt topics
			tempJournal := models.Journal{Topics: t}
			if err := h.encSvc.DecryptJournal(&tempJournal); err == nil {
				out = append(out, journalEntry{
					LocalDate:         d.Format("2006-01-02"),
					Topics:            tempJournal.Topics,
					AlignmentRating:   ar,
					ContentmentRating: cr,
					Karma:             k,
				})
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// updateUserStatsOnInsert increments user_stats when a new journal entry is created
func (h *JournalHandler) updateUserStatsOnInsert(tx *sqlx.Tx, userID int, localDate time.Time, karma float64) error {
	// Ensure user_stats row exists
	_, err := tx.Exec(`
		INSERT INTO user_stats (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING`, userID)
	if err != nil {
		return err
	}

	// Update basic counters incrementally
	_, err = tx.Exec(`
		UPDATE user_stats
		SET
			total_days_logged = total_days_logged + 1,
			total_karma = total_karma + $2,
			positive_days_count = CASE WHEN $2 > 0.5 THEN positive_days_count + 1 ELSE positive_days_count END,
			last_updated_at = NOW()
		WHERE user_id = $1`,
		userID, karma)
	if err != nil {
		return err
	}

	// Recalculate streaks from scratch since the new entry could be anywhere in the timeline
	return h.recalculateStreaks(tx, userID)
}

// updateUserStatsOnUpdate adjusts user_stats when an existing journal entry is modified
func (h *JournalHandler) updateUserStatsOnUpdate(tx *sqlx.Tx, userID int, localDate time.Time,
	oldKarma float64, newKarma float64) error {

	// Calculate karma difference
	karmaDiff := newKarma - oldKarma

	var positiveCountChange int
	if oldKarma <= 0.5 && newKarma > 0.5 {
		positiveCountChange = 1
	} else if oldKarma > 0.5 && newKarma <= 0.5 {
		positiveCountChange = -1
	}

	_, err := tx.Exec(`
		UPDATE user_stats
		SET
			total_karma = total_karma + $2,
			positive_days_count = positive_days_count + $3,
			last_updated_at = NOW()
		WHERE user_id = $1`,
		userID, karmaDiff, positiveCountChange)

	if err != nil {
		return err
	}

	// Recalculate streaks to ensure consistency
	return h.recalculateStreaks(tx, userID)
}

// updateUserStatsOnDelete decrements user_stats when a journal entry is deleted
// Note: This is complex because deleting an entry can affect streaks
func (h *JournalHandler) updateUserStatsOnDelete(tx *sqlx.Tx, userID int, deletedDate time.Time, karma float64) error {
	// Decrement basic counters
	var positiveCountChange int
	if karma > 0.5 {
		positiveCountChange = -1
	}

	_, err := tx.Exec(`
		UPDATE user_stats
		SET
			total_days_logged = GREATEST(0, total_days_logged - 1),
			total_karma = total_karma - $2,
			positive_days_count = GREATEST(0, positive_days_count + $3),
			last_updated_at = NOW()
		WHERE user_id = $1`,
		userID, karma, positiveCountChange)
	if err != nil {
		return err
	}

	// Recalculate streaks (expensive, but deletion is rare)
	// This is a simplified version - for production, you might want to optimize this
	return h.recalculateStreaks(tx, userID)
}

// recalculateStreaks fully recalculates streak data for a user (used after deletion)
func (h *JournalHandler) recalculateStreaks(tx *sqlx.Tx, userID int) error {
	query := `
		WITH user_entries AS (
			SELECT local_date
			FROM journal_entries
			WHERE user_id = $1
			ORDER BY local_date DESC
		),
		streak_groups AS (
			SELECT
				local_date,
				local_date - (ROW_NUMBER() OVER (ORDER BY local_date))::int AS grp
			FROM user_entries
		),
		streaks AS (
			SELECT
				COUNT(*) AS streak_length,
				MAX(local_date) AS streak_end_date,
				MIN(local_date) AS streak_start_date
			FROM streak_groups
			GROUP BY grp
		),
		latest_entry AS (
			SELECT MAX(local_date) AS max_date FROM user_entries
		)
		SELECT
			COALESCE(MAX(s.streak_length), 0) AS longest_streak,
			COALESCE(MAX(CASE WHEN s.streak_end_date = l.max_date THEN s.streak_length ELSE 0 END), 0) AS current_streak,
			MAX(CASE WHEN s.streak_end_date = l.max_date THEN s.streak_start_date END) AS current_start,
			l.max_date AS last_entry
		FROM streaks s, latest_entry l
		GROUP BY l.max_date`

	var longestStreak, currentStreak int
	var currentStart, lastEntry sql.NullTime
	err := tx.QueryRow(query, userID).Scan(&longestStreak, &currentStreak, &currentStart, &lastEntry)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	_, err = tx.Exec(`
		UPDATE user_stats
		SET
			longest_streak_ever = $2,
			current_streak_days = $3,
			current_streak_start_date = $4,
			last_entry_date = $5,
			last_updated_at = NOW()
		WHERE user_id = $1`,
		userID, longestStreak, currentStreak, currentStart, lastEntry)

	return err
}
