package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"winsonin/internal/models"
	"winsonin/internal/services"
)

type AuthHandler struct {
	db        *sqlx.DB
	jwtSecret []byte
	encSvc    *services.EncryptionService
}

func NewAuthHandler(db *sqlx.DB, jwtSecret []byte, encSvc *services.EncryptionService) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret, encSvc: encSvc}
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	c.Email = strings.TrimSpace(strings.ToLower(c.Email))
	if c.Email == "" || c.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "could not hash password", http.StatusInternalServerError)
		return
	}

	// Create a temporary user to encrypt
	tempUser := models.User{Email: c.Email}
	if err := h.encSvc.EncryptUser(&tempUser); err != nil {
		http.Error(w, "could not encrypt email", http.StatusInternalServerError)
		return
	}
	encryptedEmail := tempUser.Email
	emailBlindIndex := tempUser.EmailBlindIndex

	var user models.User
	err = h.db.QueryRowx(`INSERT INTO users (email, email_blind_index, password_hash) VALUES ($1, $2, $3) RETURNING id, email, email_blind_index, password_hash, created_at, first_name, last_name, avatar_id, goal, start_date, end_date, is_admin`, encryptedEmail, emailBlindIndex, string(hashed)).StructScan(&user)
	if err != nil {
		http.Error(w, "could not create user", http.StatusBadRequest)
		return
	}

	// Decrypt user data for response
	if err := h.encSvc.DecryptUser(&user); err != nil {
		http.Error(w, "could not decrypt user data", http.StatusInternalServerError)
		return
	}

	token, err := h.issueJWT(user.ID)
	if err != nil {
		http.Error(w, "could not issue token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"token": token})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	c.Email = strings.TrimSpace(strings.ToLower(c.Email))
	if c.Email == "" || c.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	// Generate blind index for email lookup
	emailBlindIndex := h.encSvc.GenerateEmailBlindIndex(c.Email)

	var user models.User
	err := h.db.Get(&user, `SELECT id, email, email_blind_index, password_hash, created_at, first_name, last_name, avatar_id, goal, start_date, end_date, is_admin FROM users WHERE email_blind_index=$1`, emailBlindIndex)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	// Decrypt user data
	if err := h.encSvc.DecryptUser(&user); err != nil {
		http.Error(w, "could not decrypt user data", http.StatusInternalServerError)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(c.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	token, err := h.issueJWT(user.ID)
	if err != nil {
		http.Error(w, "could not issue token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"token": token})
}

func (h *AuthHandler) issueJWT(userID int) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}
