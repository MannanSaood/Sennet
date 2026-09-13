package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Operation string

const (
	OpSession            Operation = "session.read"
	OpQuery              Operation = "telemetry.query"
	OpIngest             Operation = "telemetry.ingest"
	OpResourceRead       Operation = "resource.read"
	OpResourceWrite      Operation = "resource.write"
	OpCredentialManage   Operation = "credential.manage"
	OpOrganizationRead   Operation = "organization.read"
	OpOrganizationManage Operation = "organization.manage"
	OpAuditRead          Operation = "audit.read"
	OpCollectorManage    Operation = "collector.manage"
	OpCollectorEnroll    Operation = "collector.enroll"
	OpCollectorRotate    Operation = "collector.rotate"
	OpDeadLetterManage   Operation = "dead_letter.manage"
)

// Allowed is the executable counterpart of docs/workstreams/security-control-plane.md.
func Allowed(p Principal, op Operation) bool {
	if p.CredentialType == CredentialEnrollment {
		return op == OpCollectorEnroll
	}
	if p.CredentialType == CredentialCollector {
		return p.Role == "ingest" && (op == OpIngest || op == OpCollectorRotate)
	}
	role := p.Role
	if role == "reader" {
		role = "viewer"
	}
	switch op {
	case OpSession:
		return true
	case OpQuery, OpResourceRead:
		return role == "owner" || role == "admin" || role == "editor" || role == "viewer"
	case OpIngest:
		return role == "owner" || role == "admin" || role == "ingest"
	case OpResourceWrite:
		return role == "owner" || role == "admin" || role == "editor"
	case OpCredentialManage, OpCollectorManage:
		return role == "owner" || role == "admin"
	case OpDeadLetterManage:
		return role == "owner" || role == "admin"
	case OpOrganizationRead, OpAuditRead:
		return role == "owner" || role == "admin"
	case OpOrganizationManage:
		return p.SubjectType == "human" && (role == "owner" || role == "admin")
	case OpCollectorRotate:
		return role == "owner" || role == "admin"
	}
	return false
}

func routeOperation(method, path string) (Operation, bool) {
	switch {
	case path == "/api/session" || path == "/api/capabilities" || path == "/api/human-sessions":
		return OpSession, true
	case path == "/api/events" && method == "POST", path == "/sentinel.v1.SentinelService/Heartbeat", strings.HasPrefix(path, "/v1/"):
		return OpIngest, true
	case path == "/api/events" || path == "/api/summary" || path == "/api/stats" || path == "/api/finance/reconciliation" || path == "/api/topology" || path == "/api/trace" || path == "/api/correlations" || path == "/api/agent-runs" || path == "/api/query":
		return OpQuery, true
	case path == "/api/pipeline-health" || path == "/api/notification-status":
		return OpOrganizationRead, true
	case path == "/api/dashboard-versions":
		if method == "GET" {
			return OpResourceRead, true
		}
		return OpResourceWrite, true
	case path == "/api/agents" || path == "/api/workload-identities":
		return OpResourceRead, true
	case path == "/api/dashboards" || path == "/api/preferences" || path == "/api/alerts":
		if method == "GET" {
			return OpResourceRead, true
		}
		return OpResourceWrite, true
	case path == "/api/keys" || path == "/api/keys/create":
		return OpCredentialManage, true
	case path == "/api/organizations":
		return OpOrganizationRead, true
	case path == "/api/workspaces":
		if method == "GET" {
			return OpOrganizationRead, true
		}
		return OpOrganizationManage, true
	case path == "/api/memberships" || path == "/api/role-assignments":
		if method == "GET" {
			return OpOrganizationRead, true
		}
		return OpOrganizationManage, true
	case path == "/api/collector-enrollments" || path == "/api/collector-credentials":
		return OpCollectorManage, true
	case path == "/api/collectors/enroll":
		return OpCollectorEnroll, true
	case path == "/api/collector-credentials/rotate":
		return OpCollectorRotate, true
	case path == "/api/audit-events":
		return OpAuditRead, true
	case path == "/api/dead-letter":
		return OpDeadLetterManage, true
	}
	return "", false
}

// ScopedKey is the only cache/quota key constructor for control-plane state.
func ScopedKey(p Principal, namespace, id string) string {
	return "org:" + p.OrganizationID + ":workspace:" + p.WorkspaceID + ":" + namespace + ":" + id
}

type Quota interface {
	Allow(context.Context, string, int64, time.Duration, int64) (bool, error)
}
type SQLQuota struct{ store *Store }

func NewSQLQuota(s *Store) *SQLQuota { return &SQLQuota{store: s} }
func (q *SQLQuota) Allow(ctx context.Context, key string, limit int64, window time.Duration, cost int64) (bool, error) {
	if key == "" || limit < 1 || cost < 1 || cost > limit || window < time.Second {
		return false, errors.New("invalid quota")
	}
	now := time.Now().UnixMilli()
	width := window.Milliseconds()
	start := now - (now % width)
	expires := start + width*2
	query := `INSERT INTO platform_quota_windows(quota_key,window_start,used,expires) VALUES(?,?,?,?) ON CONFLICT(quota_key,window_start) DO UPDATE SET used=platform_quota_windows.used+excluded.used WHERE platform_quota_windows.used+excluded.used<=?`
	r, err := q.store.db.ExecContext(ctx, q.store.q(query), key, start, cost, expires, limit)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	if now%97 == 0 {
		_, _ = q.store.db.ExecContext(ctx, q.store.q(`DELETE FROM platform_quota_windows WHERE expires<?`), now)
	}
	return n == 1, nil
}

type ReplayStore interface {
	Consume(context.Context, string, string, int64) (bool, error)
}
type SQLReplayStore struct {
	store *Store
	max   int64
}

func NewSQLReplayStore(s *Store) *SQLReplayStore { return &SQLReplayStore{store: s, max: 100000} }
func (r *SQLReplayStore) Consume(ctx context.Context, scope, nonce string, expires int64) (bool, error) {
	now := time.Now().UnixMilli()
	tx, err := r.store.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, r.store.q(`DELETE FROM platform_replay_nonces WHERE expires<?`), now); err != nil {
		return false, err
	}
	var count int64
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM platform_replay_nonces`).Scan(&count); err != nil {
		return false, err
	}
	if count >= r.max {
		return false, errors.New("replay store capacity exhausted")
	}
	_, err = tx.ExecContext(ctx, r.store.q(`INSERT INTO platform_replay_nonces(scope,nonce,expires) VALUES(?,?,?)`), scope, nonce, expires)
	if err != nil {
		if isUnique(err) {
			return false, nil
		}
		return false, err
	}
	return true, tx.Commit()
}
func isUnique(err error) bool {
	v := strings.ToLower(err.Error())
	return strings.Contains(v, "unique") || strings.Contains(v, "duplicate")
}

type TrustedProxies struct{ nets []*net.IPNet }

func ParseTrustedProxies(raw string) (TrustedProxies, error) {
	var out TrustedProxies
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if ip := net.ParseIP(item); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			out.nets = append(out.nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, network, err := net.ParseCIDR(item)
		if err != nil {
			return out, err
		}
		out.nets = append(out.nets, network)
	}
	return out, nil
}
func (t TrustedProxies) trusted(ip net.IP) bool {
	for _, n := range t.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
func (t TrustedProxies) ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(strings.TrimSpace(host))
	if peer == nil {
		return "invalid"
	}
	if !t.trusted(peer) {
		return peer.String()
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(chain[i]))
		if ip == nil {
			continue
		}
		if !t.trusted(ip) {
			return ip.String()
		}
	}
	return peer.String()
}

type localBucket struct {
	tokens float64
	at     time.Time
}
type localLimiter struct {
	mu      sync.Mutex
	entries map[string]localBucket
}

func (l *localLimiter) allow(key string, rate, burst float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.entries[key]
	if !ok {
		if len(l.entries) >= 10000 {
			for k, v := range l.entries {
				if now.Sub(v.at) > 5*time.Minute {
					delete(l.entries, k)
				}
			}
			if len(l.entries) >= 10000 {
				return false
			}
		}
		b = localBucket{tokens: burst, at: now}
	}
	b.tokens += now.Sub(b.at).Seconds() * rate
	if b.tokens > burst {
		b.tokens = burst
	}
	b.at = now
	ok = b.tokens >= 1
	if ok {
		b.tokens--
	}
	l.entries[key] = b
	return ok
}

func signingHeadersPresent(r *http.Request) bool {
	for _, h := range []string{"X-Sennet-Signature-Version", "X-Sennet-Signature", "X-Sennet-Timestamp", "X-Sennet-Nonce"} {
		if r.Header.Get(h) != "" {
			return true
		}
	}
	return false
}
func verifySignature(ctx context.Context, replays ReplayStore, p Principal, r *http.Request, token string, body []byte) error {
	version := r.Header.Get("X-Sennet-Signature-Version")
	sig := r.Header.Get("X-Sennet-Signature")
	stampText := r.Header.Get("X-Sennet-Timestamp")
	nonce := r.Header.Get("X-Sennet-Nonce")
	requiredV2 := p.CredentialType == CredentialCollector
	if !signingHeadersPresent(r) {
		if requiredV2 {
			return errors.New("version 2 signature required")
		}
		return nil
	}
	stamp, err := strconv.ParseInt(stampText, 10, 64)
	now := time.Now().Unix()
	if err != nil || stamp < now-300 || stamp > now+300 || sig == "" {
		return errors.New("invalid signing headers")
	}
	mac := hmac.New(sha256.New, []byte(token))
	if version == "" || version == "1" {
		if nonce != "" {
			return errors.New("legacy signature cannot include nonce")
		}
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], uint64(stamp))
		mac.Write(b[:])
		mac.Write(body)
	} else if version == "2" {
		if len(nonce) < 16 || len(nonce) > 128 || strings.ContainsAny(nonce, "\r\n") {
			return errors.New("valid nonce required")
		}
		sum := sha256.Sum256(body)
		canonical := r.Method + "\n" + r.URL.EscapedPath() + "\n" + stampText + "\n" + nonce + "\n" + hex.EncodeToString(sum[:])
		mac.Write([]byte(canonical))
	} else {
		return errors.New("unsupported signature version")
	}
	given, err := hex.DecodeString(sig)
	if err != nil || !hmac.Equal(mac.Sum(nil), given) {
		return errors.New("invalid signature")
	}
	if version == "2" {
		ok, err := replays.Consume(ctx, p.CredentialID, nonce, (stamp+301)*1000)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("replayed request")
		}
	}
	return nil
}
