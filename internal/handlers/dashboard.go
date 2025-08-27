package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
)

type DashboardHandler struct {
	db *sqlx.DB
}

func NewDashboardHandler(db *sqlx.DB) *DashboardHandler { return &DashboardHandler{db: db} }

type trendPoint struct {
	LocalDate string `json:"local_date"`
	Points    int    `json:"points"`
}

type dashboardResponse struct {
	ReferenceDate      string       `json:"reference_date"`
	HasTodayEntry      bool         `json:"has_today_entry"`
	DayPoints          int          `json:"day_points"`
	WeekPoints         int          `json:"week_points"`
	MonthPoints        int          `json:"month_points"`
	YearPoints         int          `json:"year_points"`
	EntriesThisWeek    int          `json:"entries_this_week"`
	EntriesThisYear    int          `json:"entries_this_year"`
	AverageMonthRating float64      `json:"average_month_rating"`
	CurrentStreakDays  int          `json:"current_streak_days"`
	Last7DaysTrend     []trendPoint `json:"last7_days_trend"`
}

// Get aggregates and useful metrics to power the dashboard.
// Accepts optional query param: local_date=YYYY-MM-DD to use as the user's "today".
func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)

	// Determine reference date from query or default to CURRENT_DATE
	refDateStr := r.URL.Query().Get("local_date")
	var refDate time.Time
	var err error
	if refDateStr == "" {
		// Use database's CURRENT_DATE as canonical reference by reading it
		if err = h.db.QueryRowx("SELECT CURRENT_DATE").Scan(&refDate); err != nil {
			http.Error(w, "could not determine current date", http.StatusInternalServerError)
			return
		}
	} else {
		refDate, err = time.Parse("2006-01-02", refDateStr)
		if err != nil {
			http.Error(w, "invalid local_date format; expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}

	// 1) Aggregate points and counts in a single query using FILTER
	aggQuery := `
		SELECT
			COALESCE(SUM(rating) FILTER (WHERE local_date = $2), 0) AS day_points,
			COALESCE(SUM(rating) FILTER (WHERE local_date >= date_trunc('week', $2::timestamp)::date AND local_date <= $2), 0) AS week_points,
			COALESCE(SUM(rating) FILTER (WHERE date_trunc('month', local_date) = date_trunc('month', $2::date)), 0) AS month_points,
			COALESCE(SUM(rating) FILTER (WHERE date_trunc('year', local_date) = date_trunc('year', $2::date)), 0) AS year_points,
			COALESCE(COUNT(*) FILTER (WHERE local_date >= date_trunc('week', $2::timestamp)::date AND local_date <= $2), 0) AS entries_this_week,
			COALESCE(COUNT(*) FILTER (WHERE date_trunc('year', local_date) = date_trunc('year', $2::date)), 0) AS entries_this_year,
			COALESCE(AVG(rating) FILTER (WHERE date_trunc('month', local_date) = date_trunc('month', $2::date)), 0) AS avg_month_rating
		FROM journal_entries
		WHERE user_id = $1`

	var dayPts, weekPts, monthPts, yearPts int
	var entriesWeek, entriesYear int
	var avgMonth float64
	if err := h.db.QueryRowx(aggQuery, userID, refDate).Scan(&dayPts, &weekPts, &monthPts, &yearPts, &entriesWeek, &entriesYear, &avgMonth); err != nil {
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
		SELECT d::date AS local_date, COALESCE(e.rating, 0) AS points
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
		var p int
		if err := trendRows.Scan(&d, &p); err == nil {
			trend = append(trend, trendPoint{LocalDate: d.Format("2006-01-02"), Points: p})
		}
	}

	resp := dashboardResponse{
		ReferenceDate:      refDate.Format("2006-01-02"),
		HasTodayEntry:      hasToday,
		DayPoints:          dayPts,
		WeekPoints:         weekPts,
		MonthPoints:        monthPts,
		YearPoints:         yearPts,
		EntriesThisWeek:    entriesWeek,
		EntriesThisYear:    entriesYear,
		AverageMonthRating: avgMonth,
		CurrentStreakDays:  streak,
		Last7DaysTrend:     trend,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
