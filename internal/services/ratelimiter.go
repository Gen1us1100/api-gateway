package services

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Visitor represents a user with their own rate limiter.
type Visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// visitorMap is our in-memory store for visitor rate limiters.
var visitorMap = make(map[string]*Visitor)
var mu sync.Mutex // Mutex to protect access to the map.

// GetVisitorLimiter retrieves or creates a rate limiter for a given IP address.
func GetVisitorLimiter(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	v, exists := visitorMap[ip]
	if !exists {
		// Create a new limiter that allows 2 requests per second, with a burst of 5.
		// These values should ideally come from your config file!
		limiter := rate.NewLimiter(2, 5)
		visitorMap[ip] = &Visitor{limiter, time.Now()}
		return limiter
	}

	// Update the last seen time for the visitor.
	v.lastSeen = time.Now()
	return v.limiter
}

// CleanupVisitors periodically removes old entries from the map to prevent memory leaks.
// This should be run in a separate goroutine from your main application.
func CleanupVisitors() {
	for {
		time.Sleep(1 * time.Minute)

		mu.Lock()
		for ip, v := range visitorMap {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(visitorMap, ip)
			}
		}
		mu.Unlock()
	}
}
