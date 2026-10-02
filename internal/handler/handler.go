package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	luhn "github.com/EClaesson/go-luhn"

	"github.com/Meowizz/gophermart/internal/converter"
	"github.com/Meowizz/gophermart/internal/middleware"
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
		jwtSecret: []byte(jwtSecret),
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
		return "", fmt.Errorf("error generating token: %w", err)
	}
	return signedToken, nil
}

// Auth Handler for register new users
func (h *Handler) RegisterHandler(rw http.ResponseWriter, rq *http.Request) {
	logger := middleware.GetLogger(rq.Context())
	var req struct {
		LoginUser string `json:"login"`
		Password  string `json:"password"`
	}
	if err := json.NewDecoder(rq.Body).Decode(&req); err != nil {
		http.Error(rw, "Can`t parse JSON", http.StatusBadRequest)
		return
	}

	req.LoginUser = strings.TrimSpace(req.LoginUser)
	req.Password = strings.TrimSpace(req.Password)

	if req.LoginUser == "" || req.Password == "" {
		logger.Warn("empty login or password")
		http.Error(rw, "login and password are required", http.StatusBadRequest)
		return
	}
	if _, err := h.store.GetUserByLogin(rq.Context(), req.LoginUser); err == nil {
		http.Error(rw, "user already exist", http.StatusConflict)
		return
	}

	hashPass, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("BCRYPT ERROR: %v", err)
		http.Error(rw, "error hashing password", http.StatusInternalServerError)
		return
	}

	user, err := h.store.CreateUser(rq.Context(), req.LoginUser, string(hashPass))
	if err != nil {
		logger.Error("create user error", "error", err, "login", req.LoginUser)
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			http.Error(rw, "login already exists", http.StatusConflict)
			return
		}
		http.Error(rw, "error creating user", http.StatusInternalServerError)
		return
	}
	token, err := h.generateToken(user.Login)
	if err != nil {
		log.Printf("JWT ERROR: %v", err)
		http.Error(rw, "error generating token", http.StatusInternalServerError)
		return
	}
	logger.Info("user registered successfully", "user_id", user.ID, "login", user.Login)
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Authorization", "Bearer "+token)
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}

// Login handler for existing users
func (h *Handler) LoginHandler(rw http.ResponseWriter, rq *http.Request) {
	var req struct {
		LoginUser string `json:"login"`
		Password  string `json:"password"`
	}

	if err := json.NewDecoder(rq.Body).Decode(&req); err != nil {
		http.Error(rw, "invalid JSON", http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUserByLogin(rq.Context(), req.LoginUser)
	if err != nil {
		http.Error(rw, "invalid login or password", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		http.Error(rw, "invalid login or password", http.StatusUnauthorized)
		return
	}
	token, err := h.generateToken(user.Login)
	if err != nil {
		http.Error(rw, "error generating token", http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Authorization", "Bearer "+token)
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}

func (h *Handler) CreateOrder(rw http.ResponseWriter, rq *http.Request) {
	logger := middleware.GetLogger(rq.Context())
	userLogin, ok := middleware.GetUserLogin(rq)
	if !ok {
		logger.Warn("unauthorized order creation attempt")
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.store.GetUserByLogin(rq.Context(), userLogin)
	if err != nil {
		logger.Error("failed to get user", "error", err, "login", userLogin)
		http.Error(rw, "failed to get user", http.StatusInternalServerError)
		return
	}

	// Read the request body
	body, err := io.ReadAll(rq.Body)
	if err != nil {
		logger.Error("failed to read request body", "error", err)
		http.Error(rw, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer rq.Body.Close()

	orderNumber := strings.TrimSpace(string(body))
	if orderNumber == "" {
		logger.Warn("empty order number provided")
		http.Error(rw, "order number is empty", http.StatusBadRequest)
		return
	}

	isValid, err := luhn.IsValid(orderNumber)
	if err != nil || !isValid {
		logger.Warn("invalid luhn order number", "order", orderNumber)
		http.Error(rw, "invalid order number", http.StatusUnprocessableEntity)
		return
	}

	existingOrder, err := h.store.GetOrderByNumber(rq.Context(), orderNumber)
	if err == nil {
		if existingOrder.UserID == user.ID {
			logger.Info("order already exists for this user", "order", orderNumber)
			rw.WriteHeader(http.StatusOK)
			return
		} else {
			logger.Warn("order already uploaded by another user", "order", orderNumber, "user_id", user.ID)
			http.Error(rw, "order already uploaded by another user", http.StatusConflict)
			return
		}
	}

	err = h.store.CreateOrder(rq.Context(), user.ID, orderNumber)
	if err != nil {
		logger.Error("failed to create order in DB", "error", err, "order", orderNumber)
		http.Error(rw, "failed to create order", http.StatusInternalServerError)
		return
	}
	logger.Info("order created successfully", "order", orderNumber, "user_id", user.ID)
	rw.WriteHeader(http.StatusAccepted)
}

func (h *Handler) GetOrders(rw http.ResponseWriter, rq *http.Request) {
	userLogin, ok := middleware.GetUserLogin(rq)
	if !ok {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.store.GetUserByLogin(rq.Context(), userLogin)
	if err != nil {
		http.Error(rw, "failed to get user", http.StatusInternalServerError)
		return
	}

	orders, err := h.store.GetOrderByUserID(rq.Context(), user.ID)
	if err != nil {
		http.Error(rw, "failed to get order", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		rw.WriteHeader(http.StatusNoContent)
		return
	}

	response := converter.ConvertOrdersToResponse(orders)
	rw.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(rw).Encode(response); err != nil {
		http.Error(rw, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
