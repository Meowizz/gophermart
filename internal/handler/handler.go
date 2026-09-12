package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	luhn "github.com/EClaesson/go-luhn"

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
	json.NewEncoder(rw).Encode(map[string]string{"access_token": token})
}

/*
* #### **Загрузка номера заказа**

Хендлер: `POST /api/user/orders`.

Хендлер доступен только аутентифицированным пользователям. Номером заказа является последовательность цифр произвольной длины.

Номер заказа может быть проверен на корректность ввода с помощью [алгоритма Луна](https://ru.wikipedia.org/wiki/Алгоритм_Луна){target="_blank"}.

Формат запроса:

```
POST /api/user/orders HTTP/1.1
Content-Type: text/plain
...

12345678903
```

Возможные коды ответа:

- `200` — номер заказа уже был загружен этим пользователем;
- `202` — новый номер заказа принят в обработку;
- `400` — неверный формат запроса;
- `401` — пользователь не аутентифицирован;
- `409` — номер заказа уже был загружен другим пользователем;
- `422` — неверный формат номера заказа;
- `500` — внутренняя ошибка сервера.
*/
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
