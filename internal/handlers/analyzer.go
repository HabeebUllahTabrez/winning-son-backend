package handlers

import (
	"encoding/json"
	"net/http"

	"winsonin/internal/services"

	"github.com/jmoiron/sqlx"
)

type AnalyzerHandler struct {
	db     *sqlx.DB
	encSvc *services.EncryptionService
}

func NewAnalyzerHandler(db *sqlx.DB, encSvc *services.EncryptionService) *AnalyzerHandler {
	return &AnalyzerHandler{db: db, encSvc: encSvc}
}

// MarkAnalyzerUsed godoc
// @Summary Mark analyzer as used
// @Description Marks that the user has used the analyzer feature for the first time
// @Tags analyzer
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "Analyzer usage recorded successfully"
// @Failure 500 {string} string "Internal server error"
// @Router /analyzer/mark-used [post]
func (h *AnalyzerHandler) MarkAnalyzerUsed(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)

	// Update the user record to mark analyzer as used
	// Uses COALESCE to only set first_analyzer_used_at if it's currently NULL
	_, err := h.db.Exec(`
		UPDATE users
		SET has_used_analyzer = true,
		    first_analyzer_used_at = COALESCE(first_analyzer_used_at, NOW())
		WHERE id = $1 AND has_used_analyzer = false`, userID)
	if err != nil {
		http.Error(w, "could not update analyzer status", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message": "Analyzer usage recorded successfully",
		"success": true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
