package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gen1us1100/go-gateway/internal/models"
	"github.com/gen1us1100/go-gateway/pkg/config"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type UserHandler struct {
	db  *sqlx.DB
	cfg *config.Config
}

func NewUserHandler(db *sqlx.DB, cfg *config.Config) *UserHandler {
	return &UserHandler{
		db:  db,
		cfg: cfg,
	}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	UserName string `json:"username"`
	Password string `json:"password"`
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	var user models.User
	err := h.db.Get(&user, "SELECT * FROM users WHERE email = $1", req.Email)
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		log.Printf("Email gandlay")
		return
	}

	if err := user.CheckPassword(req.Password); err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		log.Printf("password gandlay")

		//		log.Printf("Login attempt failed for email: %s", req.Email)
		//		log.Printf("Stored hashed password: %s", user.Password)
		//		log.Printf("Provided password: %s", req.Password)
		//		log.Printf("Password comparison error: %v", err)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(LoginResponse{Token: tokenString})
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.UserName) == "" || strings.TrimSpace(req.Password) == "" {
		http.Error(w, "Email, username, and password cannot be empty", http.StatusBadRequest)
		return
	}

	user := models.User{
		ID:        uuid.New().String(),
		UserName:  req.UserName,
		Email:     req.Email,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := user.HashPassword(req.Password); err != nil {
		log.Printf("Error hashing password: %v", err)
		http.Error(w, "Error hashing password", http.StatusInternalServerError)
		return
	}

	_, err := h.db.Exec(`
		INSERT INTO users (id, user_name, email, password, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, user.ID, user.UserName, user.Email, user.Password, user.CreatedAt, user.UpdatedAt)

	if err != nil {
		log.Printf("Database error: %v", err)
		log.Printf("psql Error code is " + string(err.(*pq.Error).Code))
		if string(err.(*pq.Error).Code) == "23505" {
			if strings.Contains(err.(*pq.Error).Message, "email") {
				http.Error(w, "email already exists", http.StatusConflict)
				return
			}
			if strings.Contains(err.(*pq.Error).Message, "user_name") {
				http.Error(w, "username already exists", http.StatusConflict)
				return
			}
		}
		http.Error(w, "Error creating user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}
