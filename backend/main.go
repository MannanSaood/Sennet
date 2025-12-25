// Sennet Control Plane Server
// A ConnectRPC server for managing Sennet agents

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/sennet/sennet/backend/db"
	"github.com/sennet/sennet/backend/handler"
	"github.com/sennet/sennet/backend/metrics"
	"github.com/sennet/sennet/backend/middleware"

	"github.com/sennet/sennet/gen/go/sentinel/v1/sentinelv1connect"
)

const (
	defaultPort    = "8080"
	defaultDBPath  = "./sennet.db"
	defaultVersion = "1.0.0"
)

func main() {
	// CLI flags
	port := flag.String("port", defaultPort, "Server port")
	dbPath := flag.String("db", defaultDBPath, "SQLite database path")
	latestVersion := flag.String("version", defaultVersion, "Latest agent version to advertise")

	// Subcommands
	keygenCmd := flag.NewFlagSet("keygen", flag.ExitOnError)
	keygenName := keygenCmd.String("name", "", "Name/description for the API key")

	flag.Parse()

	// Handle subcommands
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "keygen":
			keygenCmd.Parse(os.Args[2:])
			runKeygen(*dbPath, *keygenName)
			return
		}
	}

	// Run server
	runServer(*port, *dbPath, *latestVersion)
}

func runKeygen(dbPath, name string) {
	if name == "" {
		name = "unnamed-key"
	}

	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	key, err := database.CreateAPIKey(name)
	if err != nil {
		log.Fatalf("Failed to create API key: %v", err)
	}

	fmt.Printf("Created API key: %s\n", key)
	fmt.Printf("Name: %s\n", name)
	fmt.Println("\nAdd this to your agent config:")
	fmt.Printf("  api_key: %s\n", key)
}

func runServer(port, dbPath, latestVersion string) {
	log.Printf("Sennet Control Plane starting...")
	log.Printf("  Port: %s", port)
	log.Printf("  Database: %s", dbPath)
	log.Printf("  Latest Version: %s", latestVersion)

	// Initialize Prometheus metrics
	metrics.Init()
	log.Printf("  Prometheus metrics: enabled")

	// Initialize database
	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Check for INIT_API_KEY environment variable (for ephemeral deployments like Koyeb)
	if initKey := os.Getenv("INIT_API_KEY"); initKey != "" {
		if err := database.EnsureAPIKey(initKey, "init-key"); err != nil {
			log.Printf("Warning: Failed to seed initial API key: %v", err)
		} else {
			log.Printf("  Initial API key loaded from environment")
		}
	}

	// Create handler
	sentinelHandler := handler.NewSentinelHandler(database, latestVersion)

	// Setup routes with middleware
	mux := http.NewServeMux()

	// Health check endpoint (no auth required)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Prometheus metrics endpoint (no auth required)
	mux.Handle("/metrics", metrics.Handler())
	log.Printf("  Metrics endpoint: GET http://localhost:%s/metrics", port)

	// ConnectRPC handler with auth middleware
	path, connectHandler := sentinelv1connect.NewSentinelServiceHandler(
		sentinelHandler,
		connect.WithInterceptors(middleware.NewAuthInterceptor(database)),
	)
	mux.Handle(path, connectHandler)

	// Create server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}
		close(done)
	}()

	// Start server
	log.Printf("Server listening on http://localhost:%s", port)
	log.Printf("Heartbeat endpoint: POST http://localhost:%s%sHeartbeat", port, path)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}

	<-done
	log.Println("Server stopped")
}
