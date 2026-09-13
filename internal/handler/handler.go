package handler

import (
	"encoding/json"
	"fmt"
	"io"
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
	json.NewEncoder(rw).Encode(map[string]string{"token": token})
}

func (h *Handler) CreateOrder(rw http.ResponseWriter, rq *http.Request) {

	// Check that the request method is POST and the content type is text/plain
	if rq.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userLogin, ok := middleware.GetUserLogin(rq)
	if !ok {
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.store.GetUserByLogin(rq.Context(), userLogin)
	if err != nil {
		http.Error(rw, "Failed to get user", http.StatusInternalServerError)
		return
	}

	// Read the request body
	body, err := io.ReadAll(rq.Body)
	if err != nil {
		http.Error(rw, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer rq.Body.Close()

	orderNumber := strings.TrimSpace(string(body))
	if orderNumber == "" {
		http.Error(rw, "Order number is empty", http.StatusBadRequest)
		return
	}

	isValid, err := luhn.IsValid(orderNumber)
	if err != nil || !isValid {
		http.Error(rw, "Invalid order number", http.StatusUnprocessableEntity)
		return
	}

	existingOrder, err := h.store.GetOrderByNumber(rq.Context(), orderNumber)
	if err == nil {
		if existingOrder.UserID == user.ID {
			rw.WriteHeader(http.StatusOK)
			return
		} else {
			http.Error(rw, "Order already uploaded by another user", http.StatusConflict)
			return
		}
	}

	err = h.store.CreateOrder(rq.Context(), user.ID, orderNumber)
	if err != nil {
		http.Error(rw, "Failed to create order", http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusAccepted)
}

func (h *Handler) GetOrders(rw http.ResponseWriter, rq *http.Request) {
	if rq.Method != http.MethodGet {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userLogin, ok := middleware.GetUserLogin(rq)
	if !ok {
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.store.GetUserByLogin(rq.Context(), userLogin)
	if err != nil {
		http.Error(rw, "Failed to get user", http.StatusInternalServerError)
		return
	}

	orders, err := h.store.GetOrderByUserID(rq.Context(), user.ID)
	if err != nil {
		http.Error(rw, "Failed to get order", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		rw.WriteHeader(http.StatusNoContent)
		return
	}

	response := converter.ConvertOrdersToResponse(orders)
	rw.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(rw).Encode(response); err != nil {
		http.Error(rw, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) OrdersHandler(rw http.ResponseWriter, rq *http.Request) {
	switch rq.Method {
	case http.MethodGet:
		h.GetOrders(rw, rq)
		return
	case http.MethodPost:
		h.CreateOrder(rw, rq)
		return
	default:
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}
