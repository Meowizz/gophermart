package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

const userClaimsContext contextKey = "userClaims"

type contextKey string

func GetUserLogin(r *http.Request) (string, bool) {
	userClaims, ok := r.Context().Value(userClaimsContext).(jwt.MapClaims)
	if !ok {
		return "", false
	}
	login, ok := userClaims["login"].(string)
	if !ok {
		return "", false
	}
	return login, true
}

// middleware for authorization of the request by JWT tokens
func AuthTokenMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
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
				return jwtSecret, nil
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

			ctx := context.WithValue(rq.Context(), userClaimsContext, claims)

			next.ServeHTTP(rw, rq.WithContext(ctx))
		})
	}
}
