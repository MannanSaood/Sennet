// Sennet Control Plane Server
// A ConnectRPC server for managing Sennet agents

package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/sennet/sennet/backend/auth"
	"github.com/sennet/sennet/backend/cloud"
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

	// Check for INIT_API_KEY environment variable (for ephemeral deployments like Render)
	if initKey := os.Getenv("INIT_API_KEY"); initKey != "" {
		log.Printf("  Found INIT_API_KEY (length=%d, prefix=%s...)", len(initKey), initKey[:min(10, len(initKey))])
		if err := database.EnsureAPIKey(initKey, "init-key"); err != nil {
			log.Printf("Warning: Failed to seed initial API key: %v", err)
		} else {
			log.Printf("  Initial API key loaded successfully")
		}
	} else {
		log.Printf("  No INIT_API_KEY environment variable set")
	}

	// Initialize Firebase Auth (optional - for dashboard users)
	var firebaseAuth *auth.FirebaseAuth
	if os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON") != "" || os.Getenv("FIREBASE_SERVICE_ACCOUNT_PATH") != "" {
		fa, err := auth.NewFirebaseAuth()
		if err != nil {
			log.Printf("Warning: Firebase Auth failed to initialize: %v", err)
			log.Printf("  Dashboard will use API key authentication")
		} else {
			firebaseAuth = fa
			log.Printf("  Firebase Auth: enabled")
		}
	} else {
		log.Printf("  Firebase Auth: disabled (no service account configured)")
	}

	// Create handler
	sentinelHandler := handler.NewSentinelHandler(database, latestVersion)

	// Initialize cloud provider registry
	cloudRegistry := cloud.NewRegistry()
	log.Printf("  Cloud provider registry initialized")

	// Load existing cloud configs from database
	cloudConfigs, err := database.GetCloudConfigs()
	if err != nil {
		log.Printf("Warning: Failed to load cloud configs: %v", err)
	} else {
		for _, cfg := range cloudConfigs {
			parsed, err := cloud.CloudConfigFromJSON(cfg.ConfigJSON)
			if err != nil {
				log.Printf("Warning: Failed to parse cloud config %s: %v", cfg.ID, err)
				continue
			}
			provider, err := cloud.CreateProvider(parsed)
			if err != nil {
				log.Printf("Warning: Failed to create provider %s: %v", cfg.ID, err)
				continue
			}
			cloudRegistry.Register(cfg.ID, provider)
			log.Printf("  Loaded cloud config: %s (%s)", cfg.ID, cfg.Provider)
		}
	}

	// Create cost handler
	costHandler := handler.NewCostHandler(database, cloudRegistry)

	// Create health handler
	healthHandler := handler.NewHealthHandler(database, latestVersion)

	// Initialize middleware
	rateLimiter := middleware.NewRateLimiter(100, 20) // 100 req/min, burst 20
	loggingMiddleware := middleware.NewLoggingMiddleware(log.Default())
	corsMiddleware := middleware.CORS(middleware.DefaultCORSConfig())

	// Setup routes
	mux := http.NewServeMux()

	// Health and probe endpoints (no auth, no rate limit)
	mux.HandleFunc("/health", healthHandler.HandleHealth)
	mux.HandleFunc("/ready", healthHandler.HandleReady)
	mux.HandleFunc("/live", healthHandler.HandleLive)
	mux.HandleFunc("/debug", healthHandler.HandleDebug)

	// Prometheus metrics endpoint (no auth required)
	mux.Handle("/metrics", metrics.Handler())
	log.Printf("  Metrics endpoint: GET http://localhost:%s/metrics", port)
	log.Printf("  Health endpoints: /health, /ready, /live")

	// ConnectRPC handler with auth middleware
	path, connectHandler := sentinelv1connect.NewSentinelServiceHandler(
		sentinelHandler,
		connect.WithInterceptors(middleware.NewAuthInterceptor(database)),
	)
	mux.Handle(path, connectHandler)

	// Cost API endpoints (with auth)
	authWrapper := middleware.NewHTTPAuthMiddleware(database)
	mux.Handle("/api/costs", authWrapper(http.HandlerFunc(costHandler.HandleGetCosts)))
	mux.Handle("/api/costs/summary", authWrapper(http.HandlerFunc(costHandler.HandleGetCostsSummary)))
	mux.Handle("/api/clouds", authWrapper(http.HandlerFunc(costHandler.HandleClouds)))
	mux.Handle("/api/recommendations", authWrapper(http.HandlerFunc(costHandler.HandleGetRecommendations)))
	mux.Handle("/api/sync-costs", authWrapper(http.HandlerFunc(costHandler.HandleSyncCosts)))
	log.Printf("  Cost API endpoints: /api/costs, /api/clouds, /api/recommendations")

	// Dashboard endpoints (stats requires auth, dashboard is public)
	statsHandler := handler.NewStatsHandler(database)

	// Use Firebase auth for dashboard if available, otherwise API key
	var dashboardAuthWrapper func(http.Handler) http.Handler
	if firebaseAuth != nil {
		dashboardAuthWrapper = auth.FirebaseMiddleware(firebaseAuth)
		log.Printf("  Dashboard auth: Firebase")
	} else {
		dashboardAuthWrapper = authWrapper
		log.Printf("  Dashboard auth: API Key")
	}

	// Create key handler
	keyHandler := handler.NewKeyHandler(database)
	mux.Handle("/api/keys", dashboardAuthWrapper(http.HandlerFunc(keyHandler.HandleGetKeys)))
	mux.Handle("/api/keys/create", dashboardAuthWrapper(http.HandlerFunc(keyHandler.HandleCreateKey)))
	log.Printf("  Key API endpoints: /api/keys, /api/keys/create")

	mux.Handle("/api/stats", dashboardAuthWrapper(http.HandlerFunc(statsHandler.HandleStats)))
	mux.HandleFunc("/dashboard", serveDashboard)
	mux.HandleFunc("/dashboard/", serveDashboard)
	log.Printf("  Dashboard: http://localhost:%s/dashboard", port)

	// Wrap mux with middleware chain: Security Heads -> Audit -> Signature -> CORS -> logging -> rate limiting -> mux
	var finalHandler http.Handler = mux
	finalHandler = rateLimiter.Middleware(finalHandler)
	finalHandler = loggingMiddleware.Middleware(finalHandler)
	finalHandler = corsMiddleware(finalHandler)
	finalHandler = middleware.SignatureMiddleware(database)(finalHandler)
	finalHandler = middleware.AuditMiddleware(middleware.DefaultAuditLogger())(finalHandler)
	finalHandler = middleware.SecurityHeaders()(finalHandler)

	// Create server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      finalHandler,
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

//go:embed dashboard/index.html
var dashboardHTML []byte

func serveDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(dashboardHTML)
}
