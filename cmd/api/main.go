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
	// --- YOUR EXISTING SETUP (UNCHANGED) ---
	log.Println("Starting API Gateway setup...")
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := db.NewDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		log.Fatalf("Failed to create postgres driver instance: %v", err)
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations/",
		"postgres", driver)
	if err != nil {
		log.Fatalf("Migration setup failed: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("An error occurred while running migration: %v", err)
	} else {
		log.Println("Database migration completed successfully.")
	}

	// --- ROUTER & HANDLER SETUP ---
	router := mux.NewRouter()

	userHandler := handlers.NewUserHandler(db, cfg)

	// NEW CHANGE: Initialize the proxy handler here. It will be used later.
	// This handler reads your config.yaml and knows how to forward requests
	// to the correct upstream services (e.g., user-service, order-service).
	proxyHandler := handlers.NewProxyHandler(cfg)

	// --- PUBLIC ROUTES (No auth required) ---
	// These are handled directly by the gateway itself.
	log.Println("Registering public routes...")
	router.HandleFunc("/api/auth/register", userHandler.Register).Methods("POST")
	router.HandleFunc("/api/auth/login", userHandler.Login).Methods("POST")
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// --- PROTECTED ROUTES (Auth required) ---
	// We create a subrouter that will have the auth middleware applied to it.
	// Any route registered on 'protected' will require a valid JWT.
	log.Println("Registering protected routes...")
	protected := router.PathPrefix("/api").Subrouter()
	protected.Use(middleware.AuthMiddleware(cfg))

	// NEW CHANGE: Register the dynamic proxy as the "catch-all" handler for the protected subrouter.
	// The PathPrefix("/") here means that any request starting with "/api" that hasn't already
	// been matched by a more specific route (like /api/auth/login) will be sent to the proxy.
	// The proxy will then decide where to forward it based on your config.yaml.
	// For example:
	// - A request to "/api/users/123" will hit this handler.
	// - A request to "/api/orders" will also hit this handler.
	// This single line replaces the need to manually define every single backend route.
	//	protected.PathPrefix("/").Handler(proxyHandler)
	protected.PathPrefix("/").Handler(http.StripPrefix("/api", proxyHandler))

	// --- CORS & SERVER SETUP ---
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Replace with your frontend's origin(s) in production
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, "UPDATE", http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With"},
		AllowCredentials: true,
	})
	handler := c.Handler(router)

	port := cfg.Port // Use port from config

	// --- GRACEFUL SHUTDOWN LOGIC (UNCHANGED) ---
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
