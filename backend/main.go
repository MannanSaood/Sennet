// Sennet control, ingestion and query services.
package main

import (
	"context"
	"flag"
	"github.com/sennet/sennet/backend/auth"
	"github.com/sennet/sennet/backend/platform"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
func main() {
	port := flag.String("port", env("PORT", "8080"), "HTTP port")
	path := flag.String("db", env("SENNET_DATABASE_URL", "./sennet-platform.db"), "SQLite path (local) or PostgreSQL URL")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	store, err := platform.Open(*path)
	if err != nil {
		log.Fatal("metadata store unavailable: ", err)
	}
	defer store.Close()
	if key := os.Getenv("INIT_API_KEY"); key != "" {
		if err = store.Seed(ctx, key, env("SENNET_BOOTSTRAP_TENANT", "local")); err != nil {
			log.Fatal(err)
		}
	}
	var telemetry platform.Telemetry = store
	var ingest platform.Ingestor = store
	mode := "local"
	if endpoint := os.Getenv("SENNET_CLICKHOUSE_URL"); endpoint != "" {
		ch := &platform.ClickHouse{URL: endpoint, User: env("SENNET_CLICKHOUSE_USER", "default"), Password: os.Getenv("SENNET_CLICKHOUSE_PASSWORD"), Client: &http.Client{Timeout: 20 * time.Second}}
		if err = ch.Init(ctx); err != nil {
			log.Fatal(err)
		}
		telemetry = ch
		ingest = ch
		mode = "analytics"
	}
	brokers := []string{}
	var stream *platform.KafkaLog
	if raw := os.Getenv("SENNET_KAFKA_BROKERS"); raw != "" {
		brokers = strings.Split(raw, ",")
		stream = platform.NewKafka(brokers, env("SENNET_KAFKA_TOPIC", "sennet-events"), os.Getenv("SENNET_KAFKA_USER"), os.Getenv("SENNET_KAFKA_PASSWORD"), os.Getenv("SENNET_KAFKA_TLS") == "true")
		defer stream.Close()
		ingest = stream
		go stream.Run(ctx, telemetry)
		mode = "streaming"
	}
	if os.Getenv("SENNET_ENV") == "production" && (!strings.HasPrefix(*path, "postgres") || mode != "streaming" || os.Getenv("SENNET_CLICKHOUSE_URL") == "") {
		log.Fatal("production requires PostgreSQL, Kafka and ClickHouse")
	}
	api := platform.NewAPI(store, telemetry, ingest)
	go api.Run(ctx)
	api.Mode = mode
	api.Brokers = brokers
	api.Origins = strings.Split(env("SENNET_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"), ",")
	if os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON") != "" || os.Getenv("FIREBASE_SERVICE_ACCOUNT_PATH") != "" {
		fa, e := auth.NewFirebaseAuth()
		if e != nil {
			log.Fatal("configured Firebase auth failed: ", e)
		}
		api.Resolve = func(ctx context.Context, token string) (platform.Principal, error) {
			verified, err := fa.VerifyToken(ctx, token)
			if err != nil {
				return platform.Principal{}, err
			}
			return platform.Principal{Tenant: "firebase:" + verified.UID, Subject: verified.UID, Role: "admin"}, nil
		}
	}
	server := &http.Server{Addr: env("SENNET_BIND", "127.0.0.1") + ":" + *port, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	go func() {
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 20*time.Second)
		defer c()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("Sennet %s listening on :%s", mode, *port)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
