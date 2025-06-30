package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gen1us1100/api-gateway/pkg/config"
	"github.com/golang-jwt/jwt/v4"
)

// Define a custom type for your context key. This prevents collisions
// with other context keys that might be used in other packages.
type contextKey string
type AppClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// UserIDKey is the key used to store the user ID in the context.
// Export it if your handlers are in a different package and need to use it.
const UserIDKey contextKey = "userID"

func AuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header is required", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" { // Make Bearer check case-insensitive
				http.Error(w, "Invalid authorization header format (expected Bearer <token>)", http.StatusUnauthorized)
				return
			}

			claims := &AppClaims{}
			tokenString := parts[1]
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				// Don't forget to validate the alg is what you expect:
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(cfg.JWTSecret), nil
			})

			if err != nil {
				// More specific error messages can be helpful for debugging but avoid leaking too much info to client
				// For example, differentiate between parsing error and signature error.
				// For client, "Invalid token" is often sufficient.
				if e, ok := err.(*jwt.ValidationError); ok {
					if e.Errors&jwt.ValidationErrorMalformed != 0 {
						http.Error(w, "Malformed token", http.StatusUnauthorized)
					} else if e.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
						http.Error(w, "Token is expired or not yet valid", http.StatusUnauthorized)
					} else {
						http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
					}
				} else {
					http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
				}
				return
			}

			if !token.Valid {
				http.Error(w, "Token is not valid", http.StatusUnauthorized)
				return
			}

			// --- THIS IS THE NEW PART ---
			// 1. Extract Claims
			//			claims, ok := token.Claims.(jwt.MapClaims)
			//			if !ok {
			//				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			//				return
			//			}
			// 2. Get the User ID (or subject) from claims
			//    The claim name 'sub' (subject) is common for user ID.
			//    Or you might have a custom claim like 'user_id'. Adjust as needed.

			userID := claims.UserID

			// 3. Store the User ID in the request's context
			// Create a new context with the userID value
			//fmt.Println(claims)
			ctx := context.WithValue(r.Context(), UserIDKey, userID)

			// Create a new request with the new context and pass it to the next handler
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
