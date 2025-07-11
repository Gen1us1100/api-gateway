package middleware

import (
	"net/http"
)

// SecureHeadersMiddleware adds common security headers to every response.
func SecureHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// These headers instruct the browser on how to handle content,
		// preventing common security vulnerabilities.

		// Prevents the browser from "sniffing" the content-type, which can
		// lead to security exploits.
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevents your site from being rendered in a <frame>, <iframe>, <embed>
		// or <object>, which helps to mitigate clickjacking attacks.
		w.Header().Set("X-Frame-Options", "DENY")

		// Enables the XSS filter in browsers that support it.
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Call the next handler in the chain. The headers are set and will
		// be included in the final response.
		next.ServeHTTP(w, r)
	})
}
