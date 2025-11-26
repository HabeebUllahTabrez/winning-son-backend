package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"winsonin/internal/models"

	"github.com/jmoiron/sqlx"
)

type PreferencesHandler struct {
	db *sqlx.DB
}

func NewPreferencesHandler(db *sqlx.DB) *PreferencesHandler {
	return &PreferencesHandler{db: db}
}

// GetPreferences returns the current user's preferences
func (h *PreferencesHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)

	var prefs models.UserPreferences
	err := h.db.Get(&prefs, `
		SELECT user_id, honesty_level, language_style, created_at, updated_at
		FROM user_preferences
		WHERE user_id = $1`, userID)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "preferences not found", http.StatusNotFound)
			return
		}
		http.Error(w, "could not fetch preferences", http.StatusInternalServerError)
		return
	}

	dto := PreferencesDTO{
		HonestyLevel:  prefs.HonestyLevel,
		LanguageStyle: prefs.LanguageStyle,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto)
}

// UpdatePreferences updates the current user's preferences (partial updates allowed)
func (h *PreferencesHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)

	var body struct {
		HonestyLevel  *string `json:"honesty_level"`
		LanguageStyle *string `json:"language_style"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Build dynamic update query
	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if body.HonestyLevel != nil {
		// Validate against lookup table
		if !h.isValidHonestyLevel(*body.HonestyLevel) {
			http.Error(w, "invalid honesty_level", http.StatusBadRequest)
			return
		}
		setClauses = append(setClauses, "honesty_level=$"+itoa(argIdx))
		args = append(args, *body.HonestyLevel)
		argIdx++
	}

	if body.LanguageStyle != nil {
		if !h.isValidLanguageStyle(*body.LanguageStyle) {
			http.Error(w, "invalid language_style", http.StatusBadRequest)
			return
		}
		setClauses = append(setClauses, "language_style=$"+itoa(argIdx))
		args = append(args, *body.LanguageStyle)
		argIdx++
	}

	if len(setClauses) == 0 {
		http.Error(w, "no fields to update", http.StatusBadRequest)
		return
	}

	// Always update updated_at
	setClauses = append(setClauses, "updated_at=NOW()")

	query := "UPDATE user_preferences SET " + join(setClauses, ", ") + " WHERE user_id=$" + itoa(argIdx)
	args = append(args, userID)

	_, err := h.db.Exec(query, args...)
	if err != nil {
		http.Error(w, "could not update preferences", http.StatusInternalServerError)
		return
	}

	// Fetch and return updated preferences
	var prefs models.UserPreferences
	err = h.db.Get(&prefs, `
		SELECT user_id, honesty_level, language_style, created_at, updated_at
		FROM user_preferences
		WHERE user_id = $1`, userID)

	if err != nil {
		http.Error(w, "could not fetch updated preferences", http.StatusInternalServerError)
		return
	}

	dto := PreferencesDTO{
		HonestyLevel:  prefs.HonestyLevel,
		LanguageStyle: prefs.LanguageStyle,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto)
}

// GetPreferenceOptions returns all available preference options from lookup tables
func (h *PreferencesHandler) GetPreferenceOptions(w http.ResponseWriter, r *http.Request) {
	var honestyLevels []models.PreferenceOption
	var languageStyles []models.PreferenceOption

	// Fetch all options from lookup tables
	err := h.db.Select(&honestyLevels, `SELECT value, display_name, description, sort_order FROM preference_honesty_levels ORDER BY sort_order`)
	if err != nil {
		http.Error(w, "could not fetch honesty levels", http.StatusInternalServerError)
		return
	}

	err = h.db.Select(&languageStyles, `SELECT value, display_name, description, sort_order FROM preference_language_styles ORDER BY sort_order`)
	if err != nil {
		http.Error(w, "could not fetch language styles", http.StatusInternalServerError)
		return
	}

	dto := PreferenceOptionsDTO{
		HonestyLevels:  honestyLevels,
		LanguageStyles: languageStyles,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto)
}

// Validation helper functions
func (h *PreferencesHandler) isValidHonestyLevel(value string) bool {
	var count int
	err := h.db.Get(&count, `SELECT COUNT(*) FROM preference_honesty_levels WHERE value = $1`, value)
	return err == nil && count > 0
}

func (h *PreferencesHandler) isValidLanguageStyle(value string) bool {
	var count int
	err := h.db.Get(&count, `SELECT COUNT(*) FROM preference_language_styles WHERE value = $1`, value)
	return err == nil && count > 0
}
