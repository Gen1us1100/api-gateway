package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gen1us1100/go-gateway/pkg/config"
	"github.com/gen1us1100/go-gateway/pkg/middleware"
	"github.com/stretchr/testify/assert"
)

func TestProxyHandler(t *testing.T) {

	t.Run("should proxy a valid request to the correct upstream service", func(t *testing.T) {

		// Create a mock backend server. This server will receive the proxied request.
		// We need to capture the request it receives so we can inspect it.
		var receivedRequest *http.Request
		mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Capture the request that hits the backend.
			receivedRequest = r
			// Send a dummy response back.
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello from the backend!"))
		}))
		defer mockBackend.Close()

		// Create a configuration that points to our mock server.
		cfg := &config.Config{
			Routes: []config.Route{
				{
					PathPrefix:  "/users",
					UpstreamURL: mockBackend.URL, // Use the mock server's dynamic URL.
				},
			},
		}

		// Create an instance of our ProxyHandler with the test config.
		proxyHandler := NewProxyHandler(cfg)

		// request that we will send TO our API Gateway.
		// This request simulates a client calling `GET /api/users/123`.
		// Note: We use "/users/123" because we assume `http.StripPrefix("/api", ...)`
		// has already happened in main.go. We are unit testing the handler itself.
		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		req.Header.Set("Authorization", "Bearer my-secret-token") // Add a header to test propagation.

		// Create a ResponseRecorder to capture the gateway's response.
		recorder := httptest.NewRecorder()

		// Execute the handler.
		proxyHandler.ServeHTTP(recorder, req)

		// Assert that the gateway's response to the client is what we expect.
		// It should have passed along the 200 OK from our mock backend.
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "Hello from the backend!", recorder.Body.String())

		// Assert that the mock backend actually received a request.
		assert.NotNil(t, receivedRequest, "The mock backend should have received a request")

		// Assert that the request received by the backend has the correct path and headers.
		if receivedRequest != nil {
			assert.Equal(t, "/users/123", receivedRequest.URL.Path)
			assert.Equal(t, "Bearer my-secret-token", receivedRequest.Header.Get("Authorization"))
		}
	})

	// This sub-test covers the case where no route matches.
	t.Run("should return 404 Not Found for an unknown route", func(t *testing.T) {

		cfg := &config.Config{
			Routes: []config.Route{
				{
					PathPrefix:  "/users",
					UpstreamURL: "http://some-service", // URL doesn't matter, it shouldn't be called.
				},
			},
		}

		proxyHandler := NewProxyHandler(cfg)
		req := httptest.NewRequest(http.MethodGet, "/orders/456", nil)

		recorder := httptest.NewRecorder()

		proxyHandler.ServeHTTP(recorder, req)

		// Assert that the gateway returned a 404 directly.
		assert.Equal(t, http.StatusNotFound, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Route not found")
	})

	t.Run("should correctly proxy requests with a body", func(t *testing.T) {
		var receivedBody string
		mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			receivedBody = string(body)
			w.WriteHeader(http.StatusCreated)
		}))
		defer mockBackend.Close()

		cfg := &config.Config{Routes: []config.Route{{PathPrefix: "/create", UpstreamURL: mockBackend.URL}}}
		proxyHandler := NewProxyHandler(cfg)

		requestBody := `{"name":"test"}`
		req := httptest.NewRequest(http.MethodPost, "/create", strings.NewReader(requestBody))
		recorder := httptest.NewRecorder()

		proxyHandler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, requestBody, receivedBody, "The backend should receive the exact request body")
	})

	t.Run("should return 502 Bad Gateway if upstream service is down", func(t *testing.T) {
		// NOTE: We don't start a server, so this URL will result in a connection refused error.
		// To get a free, guaranteed-to-be-closed port is tricky, but for most test runs
		// a high, unassigned port number will work.
		const deadUpstreamURL = "http://127.0.0.1:9999"

		cfg := &config.Config{Routes: []config.Route{{PathPrefix: "/broken", UpstreamURL: deadUpstreamURL}}}
		proxyHandler := NewProxyHandler(cfg)

		req := httptest.NewRequest(http.MethodGet, "/broken", nil)
		recorder := httptest.NewRecorder()

		proxyHandler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})

	t.Run("should prioritize the more specific route when prefixes overlap", func(t *testing.T) {
		// Mock server for the generic /users path
		userBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("generic user backend"))
		}))
		defer userBackend.Close()

		// Mock server for the specific /users/profiles path
		profileBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("profile backend"))
		}))
		defer profileBackend.Close()

		cfg := &config.Config{
			Routes: []config.Route{
				{PathPrefix: "/users", UpstreamURL: userBackend.URL},
				{PathPrefix: "/users/profiles", UpstreamURL: profileBackend.URL},
			},
		}
		proxyHandler := NewProxyHandler(cfg)

		req := httptest.NewRequest(http.MethodGet, "/users/profiles/1", nil)
		recorder := httptest.NewRecorder()
		proxyHandler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "profile backend", recorder.Body.String(), "Request should be routed to the more specific backend")
	})

	t.Run("should correctly propagate query parameters", func(t *testing.T) {
		var receivedQuery string
		mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Capture the raw query string received by the backend.
			receivedQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusOK)
		}))
		defer mockBackend.Close()

		cfg := &config.Config{Routes: []config.Route{{PathPrefix: "/search", UpstreamURL: mockBackend.URL}}}
		proxyHandler := NewProxyHandler(cfg)

		// Request with query parameters.
		req := httptest.NewRequest(http.MethodGet, "/search?q=golang&limit=10", nil)
		recorder := httptest.NewRecorder()

		proxyHandler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "q=golang&limit=10", receivedQuery, "Query parameters should be propagated unchanged")
	})

	t.Run("should transparently proxy upstream error status codes (500, 404)", func(t *testing.T) {
		mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/error" {
				http.Error(w, "Backend crashed!", http.StatusInternalServerError)
			}
			if r.URL.Path == "/api/missing" {
				http.Error(w, "Item not found in backend", http.StatusNotFound)
			}
		}))
		defer mockBackend.Close()

		cfg := &config.Config{Routes: []config.Route{{PathPrefix: "/api", UpstreamURL: mockBackend.URL}}}
		proxyHandler := NewProxyHandler(cfg)

		req500 := httptest.NewRequest(http.MethodGet, "/api/error", nil)
		recorder500 := httptest.NewRecorder()
		proxyHandler.ServeHTTP(recorder500, req500)
		assert.Equal(t, http.StatusInternalServerError, recorder500.Code, "Gateway should proxy the 500 status")
		assert.Contains(t, recorder500.Body.String(), "Backend crashed!")

		req404 := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
		recorder404 := httptest.NewRecorder()
		proxyHandler.ServeHTTP(recorder404, req404)
		assert.Equal(t, http.StatusNotFound, recorder404.Code, "Gateway should proxy the 404 status")
	})

	t.Run("should inject X-User-ID and X-Request-ID headers from context", func(t *testing.T) {
		const testUserID = "user-test-456"
		const testRequestID = "req-id-789"

		var receivedUserIDHeader, receivedRequestIDHeader string
		mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedUserIDHeader = r.Header.Get("X-User-ID")
			receivedRequestIDHeader = r.Header.Get("X-Request-ID")
			w.WriteHeader(http.StatusOK)
		}))
		defer mockBackend.Close()

		cfg := &config.Config{Routes: []config.Route{{PathPrefix: "/secure", UpstreamURL: mockBackend.URL}}}
		proxyHandler := NewProxyHandler(cfg)

		req := httptest.NewRequest(http.MethodGet, "/secure/data", nil)

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, testUserID)
		ctx = context.WithValue(ctx, middleware.CtxRequestIDKey, testRequestID)
		req = req.WithContext(ctx)

		recorder := httptest.NewRecorder()

		proxyHandler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, testUserID, receivedUserIDHeader, "X-User-ID header should be set from context")
		assert.Equal(t, testRequestID, receivedRequestIDHeader, "X-Request-ID header should be set from context")
	})
}
