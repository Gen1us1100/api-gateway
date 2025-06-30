package handlers

import (
	"log" // Or your preferred structured logger, e.g., "log/slog", "go.uber.org/zap", "github.com/rs/zerolog"
	"net/http"
)

// HandleDatabaseError logs the given database error and writes a generic
// HTTP 500 Internal Server Error response to the client.
//
// Parameters:
//
//	w: The http.ResponseWriter to send the error response to.
//	err: The database error that occurred.
//	contextMsg: A string providing context about where the error occurred (e.g., "creating user", "fetching journis").
//	            This message is for server-side logging only and is not sent to the client.
func HandleDatabaseError(w http.ResponseWriter, err error, contextMsg string) {
	// 1. Log the error on the server side with context
	//    It's crucial to log the actual error for debugging.
	//    For production, use a structured logger for better observability.
	log.Printf("ERROR: Database operation failed - %s: %v", contextMsg, err)

	// 2. Send a generic error message to the client.
	//    Do NOT send the raw database error (err.Error()) to the client,
	//    as it might expose sensitive information or internal details.
	http.Error(w, "An unexpected error occurred on the server. Please try again later.", http.StatusInternalServerError)
}
