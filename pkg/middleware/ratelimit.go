// pkg/middleware/ratelimit.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gen1us1100/go-gateway/internal/services"
)

// RateLimitMiddleware checks if the client has exceeded their request limit.
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the IP address of the client.
		// `r.RemoteAddr` can include the port, so we split it.
		ip := strings.Split(r.RemoteAddr, ":")[0]

		// Get the rate limiter for this IP address.
		limiter := services.GetVisitorLimiter(ip)

		// Check if the request is allowed. Allow() is the key method.
		if !limiter.Allow() {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return // Reject the request
		}

		// The request is allowed, call the next handler.
		next.ServeHTTP(w, r)
	})
}
