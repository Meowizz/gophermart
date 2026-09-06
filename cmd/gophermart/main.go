package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Meowizz/gophermart/internal/database"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

var secretKey []byte

type RegisterRequest struct {
	LoginUser string `json:"login"`
	Password  string `json:"password"`
}

// User`s key for JWT context
type contextKey string

const userClaimsContextKey contextKey = "user_claims"

func generateJWT(login string) (string, error) {

	claims := jwt.MapClaims{
		"subject": login,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(secretKey)
}

// Hash Password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 7)
	return string(bytes), err
}

// Compare and check password
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Find User by his/her login. If already exists return error
func findUserByLogin(login string) (*RegisterRequest, error) {
	fileReader, err := os.Open("users.txt")
	if err != nil {
		if os.IsExist(err) {
			return nil, errors.New("user not found")
		}
		fmt.Println("Can`t open file")
		return nil, err
	}
	scanner := bufio.NewScanner(fileReader)
	defer fileReader.Close()
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) > 0 {
			fileLogin := fields[0]

			if fileLogin == login {
				fmt.Println("User already exist")
				return &RegisterRequest{
					LoginUser: fileLogin,
					Password:  fields[1],
				}, nil
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("error reading file: %s", err)
		}
	}
	return nil, errors.New("user not found")
}

// Auth Handler for register new users
func AuthHandler(rw http.ResponseWriter, rq *http.Request) {
	if rq.Method != http.MethodPost {
		http.Error(rw, "Method should be POST", http.StatusMethodNotAllowed)
		return
	}
	var rr RegisterRequest
	if err := json.NewDecoder(rq.Body).Decode(&rr); err != nil {
		http.Error(rw, "Can`t parse JSON", http.StatusInternalServerError)
		return
	}
	fmt.Println("Login", rr.LoginUser, "Password", rr.Password)

	if _, err := findUserByLogin(rr.LoginUser); err == nil {
		http.Error(rw, "User already exist", http.StatusConflict)
		return
	}

	fileReader, err := os.OpenFile("users.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		http.Error(rw, "Error save to file", http.StatusInternalServerError)
		return
	}
	defer fileReader.Close()
	hashPass, err := HashPassword(rr.Password)

	if err != nil {
		http.Error(rw, "Error hashing password", http.StatusInternalServerError)
	}
	_, err = fileReader.WriteString(strings.TrimSpace(rr.LoginUser) + " " + strings.TrimSpace(hashPass) + "\n")

	if err != nil {
		http.Error(rw, "Error write to file", http.StatusInternalServerError)
		return
	}
}

// Login handler for existing users
func LoginHandler(rw http.ResponseWriter, rq *http.Request) {
	if rq.Method != http.MethodPost {
		http.Error(rw, "Method should be POST", http.StatusMethodNotAllowed)
		return
	}

	var rr RegisterRequest
	if err := json.NewDecoder(rq.Body).Decode(&rr); err != nil {
		http.Error(rw, "Can`t parse JSON", http.StatusInternalServerError)
		return
	}

	fmt.Println("Login from auth form", rr.LoginUser, "Password from auth form", rr.Password)

	user, err := findUserByLogin(rr.LoginUser)
	if err != nil {
		http.Error(rw, "Invalid login or password", http.StatusUnauthorized)
		return
	}
	checkPassword := CheckPasswordHash(rr.Password, user.Password)
	if !checkPassword {
		fmt.Println("Users.txt password is ", user.Password, "JSON parse password is", rr.Password)
		http.Error(rw, "\nWrong Password! Try again.", http.StatusUnauthorized)
	} else {
		token, err := generateJWT(user.LoginUser)
		if err != nil {
			http.Error(rw, "Error generate token", http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]string{"access_token": token})
	}

}

func ProtectedHandler(rw http.ResponseWriter, rq *http.Request) {

	claims, ok := rq.Context().Value(userClaimsContextKey).(jwt.MapClaims)
	if !ok {
		http.Error(rw, "No user data in context", http.StatusInternalServerError)
		return
	}

	userID := claims["subject"]

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]interface{}{
		"message": "Welcome to the protected area!",
		"user_id": userID,
	})
}

// middleware for authorization of the request by JWT tokens
func AuthTokenMiddleware(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request) {
			authHeader := rq.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(rw, "Authorization header is missing", http.StatusUnauthorized)
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(rw, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return secret, nil
			})

			if err != nil || !token.Valid {
				http.Error(rw, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(rw, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(rq.Context(), userClaimsContextKey, claims)

			next.ServeHTTP(rw, rq.WithContext(ctx))
		})
	}
}
func main() {
	_ = godotenv.Load()

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable is not set")
	}
	secretKey = []byte(secret)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := database.InitDB(ctx); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	serverAddr := os.Getenv("SERVER_ADDRESS")
	if serverAddr == "" {
		serverAddr = ":8080"
	}

	http.HandleFunc("/auth", AuthHandler)
	http.HandleFunc("/login", LoginHandler)

	protectedHandler := http.HandlerFunc(ProtectedHandler)
	http.Handle("/api/user/orders", AuthTokenMiddleware(secretKey)(protectedHandler))

	http.ListenAndServe(serverAddr, nil)

}
