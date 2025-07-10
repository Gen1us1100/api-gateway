// pkg/middleware/logging.go
package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// responseWriterInterceptor is a wrapper around http.ResponseWriter to capture the status code.
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriterInterceptor(w http.ResponseWriter) *responseWriterInterceptor {
	// Default to 200 OK, as this is what's assumed if WriteHeader is not called.
	return &responseWriterInterceptor{w, http.StatusOK}
}

// WriteHeader captures the status code before writing it to the actual response writer.
func (rwi *responseWriterInterceptor) WriteHeader(code int) {
	rwi.statusCode = code
	rwi.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs details about each request.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Wrap the original response writer to capture the status code.
		rwi := newResponseWriterInterceptor(w)

		// Get the request ID from the context (set by the RequestIDMiddleware).
		requestID, _ := r.Context().Value(CtxRequestIDKey).(string)

		// Call the next handler in the chain with our wrapped response writer.
		next.ServeHTTP(rwi, r)

		duration := time.Since(startTime)

		// Now we have all the information to log.
		log.Info().
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status_code", rwi.statusCode).
			Dur("latency_ms", duration).
			Str("client_ip", r.RemoteAddr).
			Msg("Incoming request")
	})
}
