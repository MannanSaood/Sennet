package platform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDevelopmentHumanBootstrapIsScopedAndIdempotent(t *testing.T) {
	s, err := Open(t.TempDir() + "/development.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := s.SeedDevelopmentHuman(ctx, "local"); err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.PrincipalForHuman(ctx, "development", "development-login", "local")
	if err != nil {
		t.Fatal(err)
	}
	if p.WorkspaceID != "local" || p.Tenant != "local" || p.SubjectType != "human" || p.Role != "admin" {
		t.Fatalf("unexpected development principal: %+v", p)
	}
	if _, err := s.PrincipalForHuman(ctx, "development", "development-login", "other"); err == nil {
		t.Fatal("development identity crossed workspace scope")
	}
}

func v2Call(h http.Handler, method, path, token, nonce string, body any) *httptest.ResponseRecorder {
	return v2CallSignedPath(h, method, path, path, token, nonce, body)
}
func v2CallSignedPath(h http.Handler, method, path, signedPath, token, nonce string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	sum := sha256.Sum256(b)
	canonical := method + "\n" + signedPath + "\n" + stamp + "\n" + nonce + "\n" + hex.EncodeToString(sum[:])
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(canonical))
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Sennet-Signature-Version", "2")
	r.Header.Set("X-Sennet-Timestamp", stamp)
	r.Header.Set("X-Sennet-Nonce", nonce)
	r.Header.Set("X-Sennet-Signature", hex.EncodeToString(mac.Sum(nil)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func legacyCall(h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	stamp := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(token))
	var raw [8]byte
	binary.LittleEndian.PutUint64(raw[:], uint64(stamp))
	mac.Write(raw[:])
	mac.Write(b)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-Sennet-Timestamp", strconv.FormatInt(stamp, 10))
	r.Header.Set("X-Sennet-Signature", hex.EncodeToString(mac.Sum(nil)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func externalCall(h http.Handler, method, path, token, workspace string, body any) *httptest.ResponseRecorder {
	var b bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&b).Encode(body)
	}
	r := httptest.NewRequest(method, path, &b)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Sennet-Workspace", workspace)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestTwoOrganizationEveryControlResourceIsIsolated(t *testing.T) {
	s, _, a, b := fixture(t)
	ctx := context.Background()
	pa, _ := s.Authenticate(ctx, a)
	pb, _ := s.Authenticate(ctx, b)
	ma, err := s.CreateMembership(ctx, pa, "firebase", "uid-a", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	mb, err := s.CreateMembership(ctx, pb, "firebase", "uid-b", "Bob")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssignRole(ctx, pa, "a", "human", ma.HumanID, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssignRole(ctx, pb, "b", "human", mb.HumanID, "admin"); err != nil {
		t.Fatal(err)
	}
	api := NewAPI(s, s, s)
	api.Resolve = func(ctx context.Context, token, workspace string) (Principal, error) {
		uid := ""
		if token == "firebase-a" {
			uid = "uid-a"
		} else if token == "firebase-b" {
			uid = "uid-b"
		}
		return s.PrincipalForHuman(ctx, "firebase", uid, workspace)
	}
	h := api.Handler()
	// Integration administrators cannot escalate into human/team administration.
	if w := call(h, "POST", "/api/memberships", a, map[string]any{"issuer": "firebase", "external_id": "evil", "display_name": "Evil"}); w.Code != 403 {
		t.Fatalf("integration privilege escalation: %d %s", w.Code, w.Body.String())
	}
	if w := externalCall(h, "POST", "/api/role-assignments", "firebase-b", "b", map[string]any{"workspace_id": "a", "subject_type": "human", "subject_id": ma.HumanID, "role": "admin"}); w.Code != 400 {
		t.Fatalf("cross-org role assignment: %d %s", w.Code, w.Body.String())
	}
	for _, kind := range []string{"dashboards", "preferences", "alerts"} {
		body := map[string]any{"id": "shared", "name": "A " + kind}
		if kind == "alerts" {
			body["threshold"] = 1
		}
		if w := call(h, "POST", "/api/"+kind, a, body); w.Code != 201 {
			t.Fatalf("create %s: %d %s", kind, w.Code, w.Body.String())
		}
		if w := call(h, "GET", "/api/"+kind, b, nil); w.Code != 200 || w.Body.String() != "[]\n" {
			t.Fatalf("cross-org %s read: %d %s", kind, w.Code, w.Body.String())
		}
	}
	if w := call(h, "POST", "/sentinel.v1.SentinelService/Heartbeat", a, map[string]any{"agentId": "a-node", "currentVersion": "1"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call(h, "GET", "/api/agents", b, nil); w.Body.String() != "[]\n" {
		t.Fatal("cross-org agent read", w.Body.String())
	}
	if w := call(h, "POST", "/api/events", a, map[string]any{"events": []Event{sample("org-a-event")}}); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call(h, "GET", "/api/events", b, nil); !strings.Contains(w.Body.String(), `"events":[]`) {
		t.Fatal("cross-org event read", w.Body.String())
	}
	created := call(h, "POST", "/api/keys", a, map[string]any{"name": "a-key", "role": "viewer", "expires": time.Now().Add(time.Hour).UnixMilli()})
	if created.Code != 201 {
		t.Fatal(created.Body.String())
	}
	var keyResult struct {
		Metadata Key    `json:"metadata"`
		Key      string `json:"key"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &keyResult)
	if w := call(h, "DELETE", "/api/keys?id="+keyResult.Metadata.ID, b, nil); w.Code != 404 {
		t.Fatal("cross-org key revoke", w.Code)
	}
	if w := call(h, "GET", "/api/keys", b, nil); strings.Contains(w.Body.String(), keyResult.Metadata.ID) || strings.Contains(w.Body.String(), keyResult.Key) {
		t.Fatal("cross-org key/secret disclosure")
	}
	if ScopedKey(pa, "cache", "same") == ScopedKey(pb, "cache", "same") {
		t.Fatal("cross-org cache key collision")
	}
	wa := externalCall(h, "GET", "/api/workspaces", "firebase-a", "a", nil)
	wb := externalCall(h, "GET", "/api/workspaces", "firebase-b", "b", nil)
	if strings.Contains(wa.Body.String(), `"id":"b"`) || strings.Contains(wb.Body.String(), `"id":"a"`) {
		t.Fatal("cross-org workspace read")
	}
	if w := externalCall(h, "GET", "/api/memberships", "firebase-b", "b", nil); strings.Contains(w.Body.String(), ma.HumanID) {
		t.Fatal("cross-org membership read")
	}
	if w := externalCall(h, "GET", "/api/role-assignments", "firebase-b", "b", nil); strings.Contains(w.Body.String(), ma.HumanID) {
		t.Fatal("cross-org role read")
	}
	if w := call(h, "GET", "/api/audit-events", b, nil); strings.Contains(w.Body.String(), keyResult.Metadata.ID) || strings.Contains(w.Body.String(), "org-a-event") {
		t.Fatal("cross-org audit read")
	}
	sessionResponse := externalCall(h, "POST", "/api/human-sessions", "firebase-a", "a", map[string]any{"expires": time.Now().Add(time.Hour).UnixMilli()})
	if sessionResponse.Code != 201 {
		t.Fatal("human session issue", sessionResponse.Code, sessionResponse.Body.String())
	}
	var session struct {
		Credential string `json:"session_credential"`
	}
	_ = json.Unmarshal(sessionResponse.Body.Bytes(), &session)
	if session.Credential == "" {
		t.Fatal("human session secret missing")
	}
	if w := call(h, "GET", "/api/session", session.Credential, nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"subject_type":"human"`) {
		t.Fatal("human session rejected", w.Code, w.Body.String())
	}
	if err := s.SetMembershipStatus(ctx, pa, ma.ID, "revoked"); err != nil {
		t.Fatal(err)
	}
	if w := call(h, "GET", "/api/session", session.Credential, nil); w.Code != 401 {
		t.Fatal("revoked membership retained session", w.Code)
	}
}

func TestCollectorEnrollmentV2ReplayRotationAndRevocation(t *testing.T) {
	_, h, a, b := fixture(t)
	created := call(h, "POST", "/api/collector-enrollments", a, map[string]any{"name": "node enrollment", "expires": time.Now().Add(5 * time.Minute).UnixMilli()})
	if created.Code != 201 {
		t.Fatal(created.Code, created.Body.String())
	}
	var enrollment struct {
		Metadata   Key    `json:"metadata"`
		Credential string `json:"enrollment_credential"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &enrollment)
	if enrollment.Credential == "" {
		t.Fatal("missing one-time enrollment credential")
	}
	enrolled := call(h, "POST", "/api/collectors/enroll", enrollment.Credential, map[string]any{"name": "collector-a"})
	if enrolled.Code != 201 {
		t.Fatal(enrolled.Code, enrolled.Body.String())
	}
	var result struct {
		Metadata   Key              `json:"metadata"`
		Credential string           `json:"collector_credential"`
		Identity   WorkloadIdentity `json:"identity"`
	}
	_ = json.Unmarshal(enrolled.Body.Bytes(), &result)
	if result.Credential == "" || result.Metadata.Expires > time.Now().Add(16*time.Minute).UnixMilli() {
		t.Fatal("collector credential not short lived")
	}
	if w := call(h, "POST", "/api/collectors/enroll", enrollment.Credential, map[string]any{"name": "again"}); w.Code != 401 {
		t.Fatal("enrollment credential reused", w.Code)
	}
	body := map[string]any{"events": []Event{sample("signed")}}
	if w := call(h, "POST", "/api/events", result.Credential, body); w.Code != 401 {
		t.Fatal("unsigned collector accepted", w.Code)
	}
	nonce := "0123456789abcdef-replay"
	if w := v2Call(h, "POST", "/api/events", result.Credential, nonce, body); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := v2Call(h, "POST", "/api/events", result.Credential, nonce, body); w.Code != 401 || !strings.Contains(w.Body.String(), "replayed") {
		t.Fatal("replay accepted", w.Code, w.Body.String())
	}
	if w := v2CallSignedPath(h, "POST", "/api/events", "/v1/traces", result.Credential, "0123456789abcdef-path", body); w.Code != 401 {
		t.Fatal("signature path not bound", w.Code, w.Body.String())
	}
	if w := legacyCall(h, "POST", "/api/events", a, map[string]any{"events": []Event{sample("legacy-signed")}}); w.Code != 202 {
		t.Fatal("legacy HMAC compatibility failed", w.Code, w.Body.String())
	}
	if w := call(h, "DELETE", "/api/collector-credentials?id="+result.Metadata.ID, b, nil); w.Code != 404 {
		t.Fatal("cross-org collector revoke", w.Code)
	}
	rotated := v2Call(h, "POST", "/api/collector-credentials/rotate", result.Credential, "0123456789abcdef-rotate", map[string]any{"credential_id": ""})
	if rotated.Code != 201 {
		t.Fatal(rotated.Code, rotated.Body.String())
	}
	var rotation struct {
		Credential string `json:"collector_credential"`
	}
	_ = json.Unmarshal(rotated.Body.Bytes(), &rotation)
	if rotation.Credential == "" || rotation.Credential == result.Credential {
		t.Fatal("rotation secret missing")
	}
	if w := v2Call(h, "POST", "/api/events", result.Credential, "0123456789abcdef-old", body); w.Code != 401 {
		t.Fatal("rotated credential still valid", w.Code)
	}
	if w := call(h, "GET", "/api/collector-credentials", a, nil); strings.Contains(w.Body.String(), result.Credential) || strings.Contains(w.Body.String(), rotation.Credential) {
		t.Fatal("collector secret listed")
	}
	if w := call(h, "GET", "/api/workload-identities", b, nil); strings.Contains(w.Body.String(), result.Identity.ID) {
		t.Fatal("cross-org workload read")
	}
}

func TestExpiryProxyQuotaAuditAndMigrationBoundaries(t *testing.T) {
	s, h, a, _ := fixture(t)
	created := call(h, "POST", "/api/keys", a, map[string]any{"name": "expiring", "role": "viewer", "expires": time.Now().Add(time.Hour).UnixMilli()})
	var keyResult struct {
		Metadata Key    `json:"metadata"`
		Key      string `json:"key"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &keyResult)
	if _, err := s.db.Exec(`UPDATE platform_credentials SET expires=? WHERE id=?`, time.Now().Add(-time.Second).UnixMilli(), keyResult.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if w := call(h, "GET", "/api/events", keyResult.Key, nil); w.Code != 401 {
		t.Fatal("expired identity accepted", w.Code)
	}
	trusted, err := ParseTrustedProxies("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.8:10"
	r.Header.Set("X-Forwarded-For", "198.51.100.9")
	if got := trusted.ClientIP(r); got != "203.0.113.8" {
		t.Fatal("trusted arbitrary forwarding header", got)
	}
	r.RemoteAddr = "10.0.0.2:10"
	r.Header.Set("X-Forwarded-For", "198.51.100.9, 10.0.0.3")
	if got := trusted.ClientIP(r); got != "198.51.100.9" {
		t.Fatal("proxy chain resolution", got)
	}
	q1, q2 := NewSQLQuota(s), NewSQLQuota(s)
	ctx := context.Background()
	if ok, _ := q1.Allow(ctx, "shared", 2, time.Minute, 1); !ok {
		t.Fatal("first quota denied")
	}
	if ok, _ := q2.Allow(ctx, "shared", 2, time.Minute, 1); !ok {
		t.Fatal("second quota denied")
	}
	if ok, _ := q1.Allow(ctx, "shared", 2, time.Minute, 1); ok {
		t.Fatal("distributed quota exceeded")
	}
	p, _ := s.Authenticate(ctx, a)
	tx, _ := s.db.BeginTx(ctx, nil)
	if err := s.auditTx(ctx, tx, p, "test.redaction", "test", "one", map[string]any{"api_key": "secret-value", "safe": "visible"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	page, err := s.AuditEvents(ctx, p, 1, "")
	if err != nil || len(page.Events) != 1 || page.Cursor == "" {
		t.Fatal("audit pagination", err, page)
	}
	raw, _ := json.Marshal(page)
	if strings.Contains(string(raw), "secret-value") || !strings.Contains(string(raw), "[REDACTED]") {
		t.Fatal("audit redaction", string(raw))
	}
	if _, err = s.db.Exec(`UPDATE platform_audit_events SET action='tampered'`); err == nil {
		t.Fatal("audit history mutable")
	}
	legacy, err := Open(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	if _, err = legacy.db.Exec(`INSERT INTO platform_events(tenant,id,time_ms,signal,service,trace_id,payload) VALUES('orphan','e',1,'trace','svc','','{}')`); err != nil {
		t.Fatal(err)
	}
	legacyToken := "sk_legacyyyyyyyyyyyyyyyyyyyyyyyyyyy"
	if _, err = legacy.db.Exec(`INSERT INTO platform_keys(id,hash,tenant,name,role,prefix,created,expires,revoked) VALUES('k',?,'orphan','legacy','admin','sk_legac',1,0,0)`, digest(legacyToken)); err != nil {
		t.Fatal(err)
	}
	report, err := legacy.MigrateLegacy(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Quarantined != 2 {
		t.Fatalf("legacy records not quarantined: %+v", report)
	}
	var active int
	if err = legacy.db.QueryRow(`SELECT COUNT(*) FROM platform_events`).Scan(&active); err != nil || active != 0 {
		t.Fatal("orphan remained active", active, err)
	}
	if _, err = legacy.Authenticate(ctx, legacyToken); err == nil {
		t.Fatal("orphan credential globally assigned")
	}
	mapped, err := Open(filepath.Join(t.TempDir(), "mapped.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer mapped.Close()
	mappedToken := "sk_mappedyyyyyyyyyyyyyyyyyyyyyyyyyy"
	if _, err = mapped.db.Exec(`INSERT INTO platform_keys(id,hash,tenant,name,role,prefix,created,expires,revoked) VALUES('k',?,'old','legacy','reader','sk_mappe',1,0,0)`, digest(mappedToken)); err != nil {
		t.Fatal(err)
	}
	if _, err = mapped.db.Exec(`INSERT INTO platform_events(tenant,id,time_ms,signal,service,trace_id,payload) VALUES('old','e',1,'trace','svc','','{}')`); err != nil {
		t.Fatal(err)
	}
	mappedReport, err := mapped.MigrateLegacy(ctx, []OwnershipMapping{{LegacyTenant: "old", OrganizationID: "org-explicit", OrganizationName: "Explicit", WorkspaceID: "ws-explicit", WorkspaceName: "Production"}})
	if err != nil {
		t.Fatal(err)
	}
	if mappedReport.CredentialsConverted != 1 || mappedReport.Mapped != 1 {
		t.Fatalf("explicit migration incomplete: %+v", mappedReport)
	}
	mappedPrincipal, err := mapped.Authenticate(ctx, mappedToken)
	if err != nil || mappedPrincipal.OrganizationID != "org-explicit" || mappedPrincipal.WorkspaceID != "ws-explicit" {
		t.Fatal("mapped credential ownership", mappedPrincipal, err)
	}
}
