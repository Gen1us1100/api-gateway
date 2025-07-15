package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecureHeadersMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	var nextCalled bool
	mockNextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success!"))
	})

	testHandler := SecureHeadersMiddleware(mockNextHandler)

	testHandler.ServeHTTP(recorder, req)

	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", recorder.Header().Get("X-XSS-Protection"))

	assert.Equal(t, http.StatusOK, recorder.Code)

	assert.Equal(t, "Success!", recorder.Body.String())
	assert.True(t, nextCalled, "The next handler in the chain was not called")
}
