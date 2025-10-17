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

// Karma is a custom type to handle rounding to 2 decimal places in JSON
type Karma float64

func (k Karma) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.2f", float64(k))), nil
}

type DashboardHandler struct {
	db     *sqlx.DB
	encSvc *services.EncryptionService
}

func NewDashboardHandler(db *sqlx.DB, encSvc *services.EncryptionService) *DashboardHandler {
	return &DashboardHandler{db: db, encSvc: encSvc}
}

type trendPoint struct {
	LocalDate string `json:"local_date"`
	Karma     Karma  `json:"karma"`
}

type dashboardResponse struct {
	ReferenceDate     string       `json:"reference_date"`
	HasTodayEntry     bool         `json:"has_today_entry"`
	DayKarma          Karma        `json:"day_karma"`
	WeekKarma         Karma        `json:"week_karma"`
	MonthKarma        Karma        `json:"month_karma"`
	YearKarma         Karma        `json:"year_karma"`
	EntriesThisWeek   int          `json:"entries_this_week"`
	EntriesThisYear   int          `json:"entries_this_year"`
	AverageMonthKarma Karma        `json:"average_month_karma"`
	CurrentStreakDays int          `json:"current_streak_days"`
	Last7DaysTrend    []trendPoint `json:"last7_days_trend"`
	User              UserDTO      `json:"user"`
}

type submissionHistoryPoint struct {
	LocalDate     string `json:"local_date"`
	HasSubmission bool   `json:"has_submission"`
}

type submissionHistoryResponse struct {
	StartDate string                   `json:"start_date"`
	EndDate   string                   `json:"end_date"`
	History   []submissionHistoryPoint `json:"history"`
}

// Get godoc
// @Summary Get dashboard metrics
// @Description Aggregates and useful metrics to power the dashboard
// @Tags dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param local_date query string false "Reference date (YYYY-MM-DD)"
// @Success 200 {object} dashboardResponse
// @Failure 500 {string} string "Internal server error"
// @Router /dashboard [get]
func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)

	// Fetch basic user profile to include in dashboard
	var user models.User
	if err := h.db.Get(&user, `SELECT id, email, email_blind_index, password_hash, created_at, first_name, last_name, avatar_id, is_admin, has_created_first_log, first_log_created_at, has_used_analyzer, first_analyzer_used_at FROM users WHERE id=$1`, userID); err != nil {
		http.Error(w, "could not fetch user", http.StatusInternalServerError)
		return
	}

	// Decrypt sensitive user fields
	if err := h.encSvc.DecryptUser(&user); err != nil {
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
	} else if err != sql.ErrNoRows {
		// Real error (not just missing goal)
		http.Error(w, "could not fetch goal", http.StatusInternalServerError)
		return
	}
	// If err == sql.ErrNoRows, goalPtr remains nil

	// Determine reference date from query or default to CURRENT_DATE
	refDateStr := r.URL.Query().Get("local_date")
	var refDate time.Time
	var parseErr error
	if refDateStr == "" {
		// Use database's CURRENT_DATE as canonical reference by reading it
		if parseErr = h.db.QueryRowx("SELECT CURRENT_DATE").Scan(&refDate); parseErr != nil {
			http.Error(w, "could not determine current date", http.StatusInternalServerError)
			return
		}
	} else {
		refDate, parseErr = time.Parse("2006-01-02", refDateStr)
		if parseErr != nil {
			http.Error(w, "invalid local_date format; expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}

	// 1) Aggregate karma and counts in a single query using FILTER
	aggQuery := `
		SELECT
			COALESCE(SUM(karma) FILTER (WHERE local_date = $2), 0) AS day_karma,
			COALESCE(SUM(karma) FILTER (WHERE local_date >= date_trunc('week', $2::timestamp)::date AND local_date <= $2), 0) AS week_karma,
			COALESCE(SUM(karma) FILTER (WHERE date_trunc('month', local_date) = date_trunc('month', $2::date)), 0) AS month_karma,
			COALESCE(SUM(karma) FILTER (WHERE date_trunc('year', local_date) = date_trunc('year', $2::date)), 0) AS year_karma,
			COALESCE(COUNT(*) FILTER (WHERE local_date >= date_trunc('week', $2::timestamp)::date AND local_date <= $2), 0) AS entries_this_week,
			COALESCE(COUNT(*) FILTER (WHERE date_trunc('year', local_date) = date_trunc('year', $2::date)), 0) AS entries_this_year,
			COALESCE(AVG(karma) FILTER (WHERE date_trunc('month', local_date) = date_trunc('month', $2::date)), 0) AS avg_month_karma
		FROM journal_entries
		WHERE user_id = $1`

	var dayKarma, weekKarma, monthKarma, yearKarma float64
	var entriesWeek, entriesYear int
	var avgMonthKarma float64
	if err := h.db.QueryRowx(aggQuery, userID, refDate).Scan(&dayKarma, &weekKarma, &monthKarma, &yearKarma, &entriesWeek, &entriesYear, &avgMonthKarma); err != nil {
		http.Error(w, "could not fetch aggregates", http.StatusInternalServerError)
		return
	}

	// 2) Has entry on reference date
	var hasToday bool
	if err := h.db.QueryRowx(`SELECT EXISTS (SELECT 1 FROM journal_entries WHERE user_id=$1 AND local_date=$2)`, userID, refDate).Scan(&hasToday); err != nil {
		http.Error(w, "could not check today's entry", http.StatusInternalServerError)
		return
	}

	// 3) Current streak up to reference date (consecutive days ending at refDate)
	streakQuery := `
		WITH d AS (
			SELECT local_date FROM journal_entries WHERE user_id=$1 AND local_date <= $2
		), g AS (
			SELECT local_date, local_date - (ROW_NUMBER() OVER (ORDER BY local_date))::int AS grp FROM d
		), c AS (
			SELECT COUNT(*) AS cnt, MAX(local_date) AS maxd FROM g GROUP BY grp
		)
		SELECT COALESCE((SELECT cnt FROM c WHERE maxd = $2), 0)`
	var streak int
	if err := h.db.QueryRowx(streakQuery, userID, refDate).Scan(&streak); err != nil {
		http.Error(w, "could not compute streak", http.StatusInternalServerError)
		return
	}

	// 4) Last 7 days trend ending at reference date (inclusive)
	trendRows, err := h.db.Queryx(`
		SELECT d::date AS local_date, COALESCE(e.karma, 0) AS karma
		FROM generate_series($2::date - INTERVAL '6 days', $2::date, INTERVAL '1 day') AS d
		LEFT JOIN journal_entries e ON e.user_id=$1 AND e.local_date = d::date
		ORDER BY d`, userID, refDate)
	if err != nil {
		http.Error(w, "could not fetch trend", http.StatusInternalServerError)
		return
	}
	defer trendRows.Close()
	var trend []trendPoint
	for trendRows.Next() {
		var d time.Time
		var p float64
		if err := trendRows.Scan(&d, &p); err == nil {
			trend = append(trend, trendPoint{LocalDate: d.Format("2006-01-02"), Karma: Karma(p)})
		}
	}

	resp := dashboardResponse{
		ReferenceDate:     refDate.Format("2006-01-02"),
		HasTodayEntry:     hasToday,
		DayKarma:          Karma(dayKarma),
		WeekKarma:         Karma(weekKarma),
		MonthKarma:        Karma(monthKarma),
		YearKarma:         Karma(yearKarma),
		EntriesThisWeek:   entriesWeek,
		EntriesThisYear:   entriesYear,
		AverageMonthKarma: Karma(avgMonthKarma),
		CurrentStreakDays: streak,
		Last7DaysTrend:    trend,
		User:              ToUserDTO(user, goalPtr),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetSubmissionHistory godoc
// @Summary Get submission history
// @Description Returns whether the user has submissions for each day in a date range (max 365 days)
// @Tags dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Success 200 {object} submissionHistoryResponse
// @Failure 400 {string} string "Bad request"
// @Failure 500 {string} string "Internal server error"
// @Router /dashboard/submission-history [get]
func (h *DashboardHandler) GetSubmissionHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)

	// Parse and validate start_date
	startDateStr := r.URL.Query().Get("start_date")
	if startDateStr == "" {
		http.Error(w, "start_date is required (format: YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		http.Error(w, "invalid start_date format; expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	// Parse and validate end_date
	endDateStr := r.URL.Query().Get("end_date")
	if endDateStr == "" {
		http.Error(w, "end_date is required (format: YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		http.Error(w, "invalid end_date format; expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	// Validate date range
	if endDate.Before(startDate) {
		http.Error(w, "end_date must be on or after start_date", http.StatusBadRequest)
		return
	}

	// Limit to 365 days
	daysDiff := endDate.Sub(startDate).Hours() / 24
	if daysDiff > 365 {
		http.Error(w, "date range cannot exceed 365 days", http.StatusBadRequest)
		return
	}

	// Query to get submission history
	query := `
		SELECT d::date AS local_date,
		       EXISTS (SELECT 1 FROM journal_entries WHERE user_id=$1 AND local_date=d::date) AS has_submission
		FROM generate_series($2::date, $3::date, INTERVAL '1 day') AS d
		ORDER BY d`

	rows, err := h.db.Queryx(query, userID, startDate, endDate)
	if err != nil {
		http.Error(w, "could not fetch submission history", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []submissionHistoryPoint
	for rows.Next() {
		var d time.Time
		var hasSubmission bool
		if err := rows.Scan(&d, &hasSubmission); err == nil {
			history = append(history, submissionHistoryPoint{
				LocalDate:     d.Format("2006-01-02"),
				HasSubmission: hasSubmission,
			})
		}
	}

	resp := submissionHistoryResponse{
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.Format("2006-01-02"),
		History:   history,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
