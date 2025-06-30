package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gen1us1100/api-gateway/internal/handlers"
	"github.com/gen1us1100/api-gateway/pkg/config"
	"github.com/gen1us1100/api-gateway/pkg/db"
	"github.com/gen1us1100/api-gateway/pkg/middleware"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	router := mux.NewRouter()
	cfg := config.LoadConfig()
	db, err := db.NewDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations/",
		"postgres", driver)
	if err != nil {
		log.Fatalf("Migration setup failed: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Migration failed: %v", err)
	}
	m.Up()

	defer db.Close()
	userHandler := handlers.NewUserHandler(db, cfg)
	// Public routes
	router.HandleFunc("/api/auth/register", userHandler.Register).Methods("POST")
	router.HandleFunc("/api/auth/login", userHandler.Login).Methods("POST")
	//	router.HandleFunc("/api/api-gateways/", api-gatewayHandler.Createapi-gateway).Methods("POST")

	// Protected routes
	protected := router.PathPrefix("/api").Subrouter()
	protected.Use(middleware.AuthMiddleware(cfg))

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Replace with your frontend's origin(s)
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, "UPDATE", http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With"}, // Add any custom headers your frontend sends
		AllowCredentials: true,                                                          // If you're using cookies or session-based authentication
		// Enable Debugging for verbose output:
		//	Debug: true,
	})
	handler := c.Handler(router)
	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// --- NEW GRACEFUL SHUTDOWN LOGIC ---
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler, // your cors-wrapped handler
	}

	// Run the server in a goroutine so that it doesn't block.
	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	// signal.Notify will send the signal to the channel.
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// or SIGTERM (used by Docker, Kubernetes, etc).
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // This will block until a signal is received
	log.Println("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the requests it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
