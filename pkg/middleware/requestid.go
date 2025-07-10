package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// Define a new type for our context key. This prevents collisions.
// CtxRequestIDKey is the key for the request ID in the context.
const CtxRequestIDKey = contextKey("requestID")

// RequestIDMiddleware generates a unique request ID for each incoming request.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()

		// Add the request ID to the response headers so the client can see it.
		w.Header().Set("X-Request-ID", requestID)

		// Create a new context with the request ID and attach it to the request.
		// This makes the ID available to all subsequent handlers.
		ctx := context.WithValue(r.Context(), CtxRequestIDKey, requestID)
		r = r.WithContext(ctx)

		// Call the next handler in the chain.
		next.ServeHTTP(w, r)
	})
}
