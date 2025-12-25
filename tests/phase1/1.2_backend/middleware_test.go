package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAuthMiddleware_ValidKey tests that valid API key passes through
func TestAuthMiddleware_ValidKey(t *testing.T) {
	t.Skip("Middleware not implemented yet")

	/*
		db := setupTestDB(t)
		db.CreateAPIKey("sk_test_valid_key", "Test Key")

		handler := middleware.AuthRequired(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/sentinel.v1.SentinelService/Heartbeat", nil)
		req.Header.Set("Authorization", "Bearer sk_test_valid_key")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got: %d", rec.Code)
		}
	*/
}

// TestAuthMiddleware_MissingKey tests that missing key returns 401
func TestAuthMiddleware_MissingKey(t *testing.T) {
	t.Skip("Middleware not implemented yet")

	/*
		handler := middleware.AuthRequired(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/sentinel.v1.SentinelService/Heartbeat", nil)
		// No Authorization header
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized, got: %d", rec.Code)
		}
	*/
}

// TestAuthMiddleware_InvalidKey tests that invalid key returns 401
func TestAuthMiddleware_InvalidKey(t *testing.T) {
	t.Skip("Middleware not implemented yet")

	/*
		handler := middleware.AuthRequired(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/sentinel.v1.SentinelService/Heartbeat", nil)
		req.Header.Set("Authorization", "Bearer sk_invalid_key_12345")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized, got: %d", rec.Code)
		}
	*/
}

// TestAuthMiddleware_MalformedHeader tests malformed Authorization header
func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	t.Skip("Middleware not implemented yet")

	/*
		handler := middleware.AuthRequired(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/sentinel.v1.SentinelService/Heartbeat", nil)
		req.Header.Set("Authorization", "NotBearer sk_test_key") // Wrong format
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized, got: %d", rec.Code)
		}
	*/
}

// Placeholders
var _ = http.StatusOK
var _ = httptest.NewRequest
var _ = testing.T{}
