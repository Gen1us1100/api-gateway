// internal/handlers/proxy.go
package handlers

import (
	"github.com/gen1us1100/api-gateway/pkg/config"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// ProxyHandler holds the routing configuration and proxies
type ProxyHandler struct {
	// A map where the key is the path prefix and the value is the proxy
	proxies map[string]*httputil.ReverseProxy
}

// NewProxyHandler creates a new ProxyHandler from the loaded config
func NewProxyHandler(cfg *config.Config) *ProxyHandler {
	proxies := make(map[string]*httputil.ReverseProxy)

	for _, route := range cfg.Routes {
		upstreamURL, err := url.Parse(route.UpstreamURL)
		if err != nil {
			log.Fatalf("Failed to parse upstream URL for route %s: %v", route.PathPrefix, err)
		}

		// Create a new reverse proxy for this route
		proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

		log.Printf("Mapping path %s -> %s", route.PathPrefix, route.UpstreamURL)
		proxies[route.PathPrefix] = proxy
	}

	return &ProxyHandler{
		proxies: proxies,
	}
}

// ServeHTTP is the entry point for all incoming requests
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Find the correct proxy for the request path
	log.Printf("ProxyHandler received request for path: %s", r.URL.Path)
	for path, proxy := range p.proxies {
		log.Printf(r.URL.Path)
		log.Printf(path)
		if strings.HasPrefix(r.URL.Path, path) {
			// Found a matching route, let the proxy handle it
			log.Printf("Proxying request for %s to %s", r.URL.Path, path)
			proxy.ServeHTTP(w, r)
			return
		}
	}

	// If no route is found, return an error
	http.Error(w, "Route not found", http.StatusNotFound)
}
