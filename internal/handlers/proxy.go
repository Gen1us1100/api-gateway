package handlers

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gen1us1100/go-gateway/pkg/config"
	"github.com/gen1us1100/go-gateway/pkg/middleware"
	"github.com/rs/zerolog/log"
)

// ProxyHandler now holds the configuration, not a map of proxies.
type ProxyHandler struct {
	config *config.Config
}

// NewProxyHandler creates a new ProxyHandler.
func NewProxyHandler(cfg *config.Config) *ProxyHandler {
	return &ProxyHandler{
		config: cfg,
	}
}

// ServeHTTP is the main entry point for proxying.
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Find the correct upstream service for the request path.
	var targetRoute *config.Route
	for _, route := range p.config.Routes {
		if strings.HasPrefix(r.URL.Path, route.PathPrefix) {
			targetRoute = &route // Found a matching route.
			break
		}
	}

	// If no route is found, return an error.
	if targetRoute == nil {
		http.Error(w, "Route not found", http.StatusNotFound)
		return
	}

	// Parse the upstream URL.
	upstreamURL, err := url.Parse(targetRoute.UpstreamURL)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse upstream URL")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// --- LEVEL 2 LOGGING IMPLEMENTATION ---

	// Create a new reverse proxy instance for this specific request.
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	// Get the request ID from the context to correlate logs.
	requestID, _ := r.Context().Value(middleware.CtxRequestIDKey).(string)

	// This function is called just BEFORE the request is sent to the backend.
	proxy.Director = func(req *http.Request) {
		// Preserve the original Host and other necessary headers.
		req.URL.Scheme = upstreamURL.Scheme
		req.URL.Host = upstreamURL.Host
		req.URL.Path = r.URL.Path // Use the original path
		req.Host = upstreamURL.Host

		// Get the original request's context
		originalCtx := r.Context()

		// Get the userID that your AuthMiddleware added
		userID, ok := originalCtx.Value(middleware.UserIDKey).(string) // Use your actual key
		if !ok {
			log.Warn().Msg("Could not find userID in context for proxied request")
		} else {
			// Add the userID as a custom header for the backend service to read.
			req.Header.Set("X-User-ID", userID)
		}
		req.Header.Set("X-Request-ID", requestID)
	}

	// This function is called AFTER the backend responds, but BEFORE the gateway
	// sends the response back to the client. This is our "split time" hook.
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Log the details of the backend interaction.
		log.Info().
			Str("request_id", requestID).
			Str("upstream_service", targetRoute.UpstreamURL).
			Int("upstream_status", resp.StatusCode).
			Msg("Response received from upstream")
		return nil // Return nil to not modify the response.
	}

	// This function handles errors that occur during the proxying, like connection refused.
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error().
			Err(err).
			Str("request_id", requestID).
			Str("upstream_service", targetRoute.UpstreamURL).
			Msg("Upstream service error")
		http.Error(w, fmt.Sprintf("Upstream service unavailable: %v", err), http.StatusBadGateway)
	}

	// Start a timer for the upstream request.
	upstreamStartTime := time.Now()

	// Let the proxy handle the request.
	proxy.ServeHTTP(w, r)

	// Calculate and log the duration of the upstream request.
	upstreamDuration := time.Since(upstreamStartTime)
	log.Info().
		Str("request_id", requestID).
		Str("upstream_service", targetRoute.UpstreamURL).
		Dur("upstream_latency_ms", upstreamDuration).
		Msg("Upstream request completed")
}
