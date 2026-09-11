package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Meowizz/gophermart/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	store     *repository.Store
	jwtSecret []byte
}

func NewHandler(store *repository.Store, jwtSecret []byte) *Handler {
	return &Handler{
		store:     store,
		jwtSecret: jwtSecret,
	}
}

func (h *Handler) generateToken(login string) (string, error) {
	claims := jwt.MapClaims{
		"login": login,
		"exp":   time.Now().Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(h.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("Error generating token: %w", err)
	}
	return signedToken, nil
}

// Auth Handler for register new users
func (h *Handler) RegisterHandler(rw http.ResponseWriter, rq *http.Request) {
	if rq.Method != http.MethodPost {
		http.Error(rw, "Method should be POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		LoginUser string `json:"login"`
		Password  string `json:"password"`
	}
	if err := json.NewDecoder(rq.Body).Decode(&req); err != nil {
		http.Error(rw, "Can`t parse JSON", http.StatusInternalServerError)
		return
	}

	req.LoginUser = strings.TrimSpace(req.LoginUser)
	req.Password = strings.TrimSpace(req.Password)

	if req.LoginUser == "" || req.Password == "" {
		http.Error(rw, "Login and password are required", http.StatusBadRequest)
		return
	}
	if _, err := h.store.GetUserByLogin(rq.Context(), req.LoginUser); err == nil {
		http.Error(rw, "User already exist", http.StatusConflict)
		return
	}

	hashPass, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(rw, "Error hashing password", http.StatusInternalServerError)
		return
	}

	user, err := h.store.CreateUser(rq.Context(), req.LoginUser, string(hashPass))
	if err != nil {
		http.Error(rw, "Error creating user", http.StatusInternalServerError)
		return
	}
	token, err := h.generateToken(user.Login)
	if err != nil {
		http.Error(rw, "Error generating token", http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]string{"token": token})
}

// Login handler for existing users
func (h *Handler) LoginHandler(rw http.ResponseWriter, rq *http.Request) {
	if rq.Method != http.MethodPost {
		http.Error(rw, "Method should be POST", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		LoginUser string `json:"login"`
		Password  string `json:"password"`
	}

	if err := json.NewDecoder(rq.Body).Decode(&req); err != nil {
		http.Error(rw, "Invalid JSON", http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUserByLogin(rq.Context(), req.LoginUser)
	if err != nil {
		http.Error(rw, "Invalid login or password", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		http.Error(rw, "Invalid login or password", http.StatusUnauthorized)
		return
	}
	token, err := h.generateToken(user.Login)
	if err != nil {
		http.Error(rw, "Error generating token", http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]string{"access_token": token})
}
