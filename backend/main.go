// Sennet independently deployable data-plane and control/query roles.
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"github.com/sennet/sennet/backend/auth"
	"github.com/sennet/sennet/backend/platform"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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
func envInt(name string, fallback int) int {
	v, err := strconv.Atoi(env(name, strconv.Itoa(fallback)))
	if err != nil || v < 1 {
		log.Fatalf("%s must be a positive integer", name)
	}
	return v
}
func kafkaConfig(topic string) platform.KafkaConfig {
	raw := strings.TrimSpace(os.Getenv("SENNET_KAFKA_BROKERS"))
	brokers := []string{}
	if raw != "" {
		for _, broker := range strings.Split(raw, ",") {
			if strings.TrimSpace(broker) != "" {
				brokers = append(brokers, strings.TrimSpace(broker))
			}
		}
	}
	return platform.KafkaConfig{Brokers: brokers, Topic: topic, Group: env("SENNET_KAFKA_GROUP", "sennet-storage-v1"), User: os.Getenv("SENNET_KAFKA_USER"), Password: os.Getenv("SENNET_KAFKA_PASSWORD"), TLS: os.Getenv("SENNET_KAFKA_TLS") == "true", BatchSize: envInt("SENNET_KAFKA_BATCH_EVENTS", 500), BatchBytes: int64(envInt("SENNET_KAFKA_BATCH_BYTES", 1<<20)), BatchTimeout: time.Duration(envInt("SENNET_KAFKA_BATCH_MS", 10)) * time.Millisecond, WriteTimeout: time.Duration(envInt("SENNET_KAFKA_WRITE_TIMEOUT_MS", 10000)) * time.Millisecond}
}
func clickHouse() (*platform.ClickHouse, error) {
	endpoint := os.Getenv("SENNET_CLICKHOUSE_URL")
	if endpoint == "" {
		return nil, errors.New("SENNET_CLICKHOUSE_URL is required")
	}
	return &platform.ClickHouse{URL: endpoint, User: env("SENNET_CLICKHOUSE_USER", "default"), Password: os.Getenv("SENNET_CLICKHOUSE_PASSWORD"), Client: &http.Client{Timeout: 20 * time.Second}, InitMode: env("SENNET_CLICKHOUSE_INIT", "local")}, nil
}
func archive() platform.Archive {
	if root := os.Getenv("SENNET_ARCHIVE_DIR"); root != "" {
		return &platform.FileArchive{Root: root}
	}
	return nil
}
func configureIdentity(api *platform.API, store *platform.Store) {
	if env("SENNET_AUTH_MODE", "firebase") == "development" {
		if os.Getenv("SENNET_ENV") == "production" {
			log.Fatal("development login sessions are forbidden in production")
		}
		token := os.Getenv("SENNET_DEVELOPMENT_SESSION_TOKEN")
		if len(token) < 24 {
			log.Fatal("development auth requires SENNET_DEVELOPMENT_SESSION_TOKEN with at least 24 characters")
		}
		workspace := env("SENNET_DEVELOPMENT_TENANT", "local")
		if err := store.SeedDevelopmentHuman(context.Background(), workspace); err != nil {
			log.Fatal("development identity bootstrap failed: ", err)
		}
		api.Resolve = func(ctx context.Context, candidate, workspace string) (platform.Principal, error) {
			if subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) != 1 {
				return platform.Principal{}, errors.New("invalid development login session")
			}
			return store.PrincipalForHuman(ctx, "development", "development-login", workspace)
		}
		return
	}
	if env("SENNET_AUTH_MODE", "firebase") != "firebase" {
		log.Fatal("SENNET_AUTH_MODE must be firebase or development")
	}
	if os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON") == "" && os.Getenv("FIREBASE_SERVICE_ACCOUNT_PATH") == "" {
		return
	}
	fa, err := auth.NewFirebaseAuth()
	if err != nil {
		log.Fatal("configured Firebase auth failed: ", err)
	}
	api.Resolve = func(ctx context.Context, token, workspace string) (platform.Principal, error) {
		verified, err := fa.VerifyToken(ctx, token)
		if err != nil {
			return platform.Principal{}, err
		}
		return store.PrincipalForHuman(ctx, "firebase", verified.UID, workspace)
	}
}

var processPort string

func server(handler http.Handler) *http.Server {
	return &http.Server{Addr: env("SENNET_BIND", "127.0.0.1") + ":" + processPort, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
}
func serve(ctx context.Context, s *http.Server) error {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			_ = s.Shutdown(shutdownCtx)
			cancel()
		case <-done:
		}
	}()
	err := s.ListenAndServe()
	close(done)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func readyAll(checks ...platform.Pinger) func(context.Context) error {
	return func(ctx context.Context) error {
		for _, check := range checks {
			if check != nil {
				if err := check.Ping(ctx); err != nil {
					return err
				}
			}
		}
		return nil
	}
}
func internalHandler(role, token string, metrics *platform.DataPlaneMetrics, ready func(context.Context) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/live" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"live"}`))
			return
		}
		if r.URL.Path == "/ready" || r.URL.Path == "/health" {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			if ready != nil && ready(ctx) != nil {
				http.Error(w, "dependency unavailable", 503)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ready","role":"` + role + `"}`))
			return
		}
		if r.URL.Path == "/internal/metrics" {
			parts := strings.Fields(r.Header.Get("Authorization"))
			if token == "" || len(parts) != 2 || parts[0] != "Bearer" || subtle.ConstantTimeCompare([]byte(parts[1]), []byte(token)) != 1 {
				http.Error(w, "operator bearer credential required", 401)
				return
			}
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			_, _ = w.Write([]byte(metrics.Prometheus(role)))
			return
		}
		http.NotFound(w, r)
	})
}

func main() {
	port := flag.String("port", env("PORT", "8080"), "HTTP/operator port")
	dbPath := flag.String("db", env("SENNET_DATABASE_URL", "./sennet-platform.db"), "SQLite path (local) or PostgreSQL URL")
	role := flag.String("role", env("SENNET_ROLE", "all"), "gateway, storage-consumer, query-control, or all")
	migrateLegacy := flag.Bool("migrate-legacy", false, "run the explicit ownership/quarantine migration and exit")
	ownershipMap := flag.String("ownership-map", "", "JSON ownership mapping used only with -migrate-legacy")
	flag.Parse()
	processPort = *port
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *migrateLegacy {
		runMigration(ctx, *dbPath, *ownershipMap)
		return
	}
	metrics := &platform.DataPlaneMetrics{}
	switch *role {
	case "gateway":
		runGateway(ctx, *dbPath, metrics)
	case "storage-consumer":
		runConsumer(ctx, metrics)
	case "query-control":
		runQuery(ctx, *dbPath, metrics)
	case "all":
		runAll(ctx, *dbPath, metrics)
	default:
		log.Fatalf("unknown SENNET_ROLE %q", *role)
	}
}

func baseAPI(store *platform.Store, telemetry platform.Telemetry, ingest platform.Ingestor, role, mode string, metrics *platform.DataPlaneMetrics) *platform.API {
	api := platform.NewAPI(store, telemetry, ingest)
	api.Role, api.Mode, api.Metrics = role, mode, metrics
	api.Origins = strings.Split(env("SENNET_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"), ",")
	api.InternalToken = os.Getenv("SENNET_OPERATOR_TOKEN")
	proxies, err := platform.ParseTrustedProxies(os.Getenv("SENNET_TRUSTED_PROXIES"))
	if err != nil {
		log.Fatal("invalid SENNET_TRUSTED_PROXIES: ", err)
	}
	api.Proxies = proxies
	configureIdentity(api, store)
	return api
}
func bootstrapStore(ctx context.Context, store *platform.Store) {
	if key := os.Getenv("INIT_API_KEY"); key != "" {
		if err := store.Seed(ctx, key, env("SENNET_BOOTSTRAP_TENANT", "local")); err != nil {
			log.Fatal(err)
		}
	}
}
func runMigration(ctx context.Context, dbPath, ownershipMap string) {
	store, err := platform.Open(dbPath)
	if err != nil {
		log.Fatal("metadata store unavailable: ", err)
	}
	defer store.Close()
	mappings := []platform.OwnershipMapping{}
	if ownershipMap != "" {
		data, readErr := os.ReadFile(ownershipMap)
		if readErr != nil {
			log.Fatal("ownership map unavailable: ", readErr)
		}
		if jsonErr := json.Unmarshal(data, &mappings); jsonErr != nil {
			log.Fatal("invalid ownership map: ", jsonErr)
		}
	}
	report, err := store.MigrateLegacy(ctx, mappings)
	if err != nil {
		log.Fatal("legacy migration failed: ", err)
	}
	if err = json.NewEncoder(os.Stdout).Encode(report); err != nil {
		log.Fatal(err)
	}
}
func requireProduction(role, dbPath string, kafka, ch bool) {
	if os.Getenv("SENNET_ENV") != "production" {
		return
	}
	if (role == "gateway" || role == "query-control" || role == "all") && !strings.HasPrefix(dbPath, "postgres") {
		log.Fatal("production gateway/query roles require PostgreSQL")
	}
	if !kafka {
		log.Fatal("production distributed roles require Kafka")
	}
	if !ch && role != "gateway" {
		log.Fatal("production consumer/query roles require ClickHouse")
	}
	if ch && env("SENNET_CLICKHOUSE_INIT", "local") != "none" {
		log.Fatal("production requires pre-provisioned ClickHouse schema and SENNET_CLICKHOUSE_INIT=none")
	}
}
func runGateway(ctx context.Context, dbPath string, metrics *platform.DataPlaneMetrics) {
	cfg := kafkaConfig(env("SENNET_KAFKA_TOPIC", "sennet-events"))
	requireProduction("gateway", dbPath, len(cfg.Brokers) > 0, false)
	if len(cfg.Brokers) == 0 {
		log.Fatal("gateway requires SENNET_KAFKA_BROKERS")
	}
	store, err := platform.Open(dbPath)
	if err != nil {
		log.Fatal("metadata store unavailable: ", err)
	}
	defer store.Close()
	bootstrapStore(ctx, store)
	producer := platform.NewKafkaProducer(cfg, metrics)
	defer producer.Close()
	ingest := platform.NewBoundedIngestor(producer, envInt("SENNET_INGEST_CONCURRENCY", 64), metrics)
	api := baseAPI(store, nil, ingest, "gateway", "streaming", metrics)
	api.Ready = readyAll(store, producer)
	s := server(api.Handler())
	log.Printf("Sennet gateway listening on %s", s.Addr)
	if err = serve(ctx, s); err != nil {
		log.Fatal(err)
	}
}
func runQuery(ctx context.Context, dbPath string, metrics *platform.DataPlaneMetrics) {
	ch, err := clickHouse()
	if err != nil {
		log.Fatal(err)
	}
	cfg := kafkaConfig(env("SENNET_KAFKA_TOPIC", "sennet-events"))
	requireProduction("query-control", dbPath, len(cfg.Brokers) > 0, true)
	store, err := platform.Open(dbPath)
	if err != nil {
		log.Fatal("metadata store unavailable: ", err)
	}
	defer store.Close()
	bootstrapStore(ctx, store)
	if err = ch.Init(ctx); err != nil {
		log.Fatal(err)
	}
	api := baseAPI(store, ch, nil, "query-control", "streaming", metrics)
	api.Ready = readyAll(store, ch)
	var source *platform.KafkaProducer
	if len(cfg.Brokers) > 0 {
		source = platform.NewKafkaProducer(cfg, metrics)
		defer source.Close()
		dcfg := cfg
		dcfg.Topic = env("SENNET_KAFKA_DLQ_TOPIC", "sennet-events-dead-letter")
		api.DeadLetters = &platform.KafkaDeadLetterAdmin{Config: dcfg, Source: source, Metrics: metrics, ScanLimit: envInt("SENNET_DLQ_SCAN_LIMIT", 10000)}
		api.Ready = readyAll(store, ch, source)
	}
	go api.Run(ctx)
	s := server(api.Handler())
	log.Printf("Sennet query/control listening on %s", s.Addr)
	if err = serve(ctx, s); err != nil {
		log.Fatal(err)
	}
}
func newConsumer(cfg platform.KafkaConfig, ch *platform.ClickHouse, metrics *platform.DataPlaneMetrics) (*platform.StorageConsumer, *platform.KafkaProducer) {
	dcfg := cfg
	dcfg.Topic = env("SENNET_KAFKA_DLQ_TOPIC", "sennet-events-dead-letter")
	dcfg.BatchBytes = cfg.BatchBytes * 2
	dlq := platform.NewKafkaProducer(dcfg, metrics)
	consumer := platform.NewStorageConsumer(cfg, ch, archive(), platform.KafkaDeadLetterWriter{Producer: dlq}, metrics)
	consumer.MaxDeadLetter = int(cfg.BatchBytes / 2)
	return consumer, dlq
}
func runConsumer(ctx context.Context, metrics *platform.DataPlaneMetrics) {
	cfg := kafkaConfig(env("SENNET_KAFKA_TOPIC", "sennet-events"))
	ch, err := clickHouse()
	if err != nil {
		log.Fatal(err)
	}
	requireProduction("storage-consumer", "", len(cfg.Brokers) > 0, true)
	if len(cfg.Brokers) == 0 {
		log.Fatal("storage-consumer requires SENNET_KAFKA_BROKERS")
	}
	if err = ch.Init(ctx); err != nil {
		log.Fatal(err)
	}
	consumer, dlq := newConsumer(cfg, ch, metrics)
	defer consumer.Close()
	defer dlq.Close()
	errCh := make(chan error, 2)
	s := server(internalHandler("storage-consumer", os.Getenv("SENNET_OPERATOR_TOKEN"), metrics, readyAll(consumer)))
	go func() { errCh <- consumer.Run(ctx) }()
	go func() { errCh <- serve(ctx, s) }()
	log.Printf("Sennet storage consumer operator endpoint listening on %s", s.Addr)
	err = <-errCh
	cancelCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	_ = s.Shutdown(cancelCtx)
	cancel()
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
func runAll(ctx context.Context, dbPath string, metrics *platform.DataPlaneMetrics) {
	store, err := platform.Open(dbPath)
	if err != nil {
		log.Fatal("metadata store unavailable: ", err)
	}
	defer store.Close()
	bootstrapStore(ctx, store)
	var telemetry platform.Telemetry = store
	var ingest platform.Ingestor = store
	mode := "local"
	checks := []platform.Pinger{store}
	cfg := kafkaConfig(env("SENNET_KAFKA_TOPIC", "sennet-events"))
	var ch *platform.ClickHouse
	if os.Getenv("SENNET_CLICKHOUSE_URL") != "" {
		ch, err = clickHouse()
		if err != nil {
			log.Fatal(err)
		}
		if err = ch.Init(ctx); err != nil {
			log.Fatal(err)
		}
		telemetry = ch
		checks = append(checks, ch)
	}
	if ch != nil && len(cfg.Brokers) == 0 {
		log.Fatal("all role requires Kafka and ClickHouse together, or neither for local mode")
	}
	requireProduction("all", dbPath, len(cfg.Brokers) > 0, ch != nil)
	api := baseAPI(store, telemetry, ingest, "all", mode, metrics)
	var producer, dlq *platform.KafkaProducer
	var consumer *platform.StorageConsumer
	if len(cfg.Brokers) > 0 {
		if ch == nil {
			log.Fatal("streaming all role requires ClickHouse")
		}
		producer = platform.NewKafkaProducer(cfg, metrics)
		defer producer.Close()
		ingest = platform.NewBoundedIngestor(producer, envInt("SENNET_INGEST_CONCURRENCY", 64), metrics)
		consumer, dlq = newConsumer(cfg, ch, metrics)
		defer consumer.Close()
		defer dlq.Close()
		go func() {
			if e := consumer.Run(ctx); e != nil && !errors.Is(e, context.Canceled) {
				log.Printf("storage consumer stopped: %v", e)
			}
		}()
		dcfg := cfg
		dcfg.Topic = env("SENNET_KAFKA_DLQ_TOPIC", "sennet-events-dead-letter")
		api.DeadLetters = &platform.KafkaDeadLetterAdmin{Config: dcfg, Source: producer, Metrics: metrics, ScanLimit: envInt("SENNET_DLQ_SCAN_LIMIT", 10000)}
		checks = append(checks, producer, consumer)
		mode = "streaming"
	}
	api.Ingest, api.Mode, api.Ready = ingest, mode, readyAll(checks...)
	go api.Run(ctx)
	s := server(api.Handler())
	log.Printf("Sennet all-in-one %s mode listening on %s", mode, s.Addr)
	if err = serve(ctx, s); err != nil {
		log.Fatal(err)
	}
}
