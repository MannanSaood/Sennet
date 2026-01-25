package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/sennet/sennet/backend/db"
)

const (
	// SignatureHeader is the header containing the HMAC signature
	SignatureHeader = "X-Sennet-Signature"
	// TimestampHeader is the header containing the request timestamp
	TimestampHeader = "X-Sennet-Timestamp"
	// MaxTimestampAge is the maximum age of a request before it's rejected (5 minutes)
	MaxTimestampAge = 5 * 60
)

// SignatureMiddleware creates middleware that verifies HMAC signatures on requests
// This provides protection against:
// - Request tampering (HMAC verification)
// - Replay attacks (timestamp validation)
func SignatureMiddleware(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract headers
			signature := r.Header.Get(SignatureHeader)
			timestampStr := r.Header.Get(TimestampHeader)

			// Signature is optional for backward compatibility
			// If not present, skip verification but log a warning
			if signature == "" || timestampStr == "" {
				// Allow request but could log for monitoring
				next.ServeHTTP(w, r)
				return
			}

			// Parse timestamp
			timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid timestamp format", http.StatusBadRequest)
				return
			}

			// Check timestamp is within acceptable range (prevent replay attacks)
			now := time.Now().Unix()
			if abs(now-timestamp) > MaxTimestampAge {
				http.Error(w, "Request expired", http.StatusUnauthorized)
				return
			}

			// Read body for verification
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}
			// Restore body for downstream handlers
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			// Get API key from Authorization header
			apiKey := extractAPIKey(r)
			if apiKey == "" {
				http.Error(w, "Authorization required", http.StatusUnauthorized)
				return
			}

			// Verify the API key exists
			exists, err := database.APIKeyExists(apiKey)
			if err != nil || !exists {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Verify signature
			expectedSig := signRequest(apiKey, timestamp, body)
			if !verifySignature(expectedSig, signature) {
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// signRequest computes HMAC-SHA256 signature matching the Rust agent implementation
func signRequest(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))

	// Write timestamp as little-endian bytes (matching Rust i64::to_le_bytes())
	tsBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(tsBytes, uint64(timestamp))
	mac.Write(tsBytes)

	// Write body
	mac.Write(body)

	return hex.EncodeToString(mac.Sum(nil))
}

// verifySignature compares signatures using constant-time comparison
func verifySignature(expected, actual string) bool {
	expectedBytes, err1 := hex.DecodeString(expected)
	actualBytes, err2 := hex.DecodeString(actual)

	if err1 != nil || err2 != nil {
		return false
	}

	return subtle.ConstantTimeCompare(expectedBytes, actualBytes) == 1
}

// extractAPIKey extracts the API key from the Authorization header
func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

// abs returns the absolute value of an int64
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// RequireSignature creates a stricter middleware that requires signatures
// Use this for sensitive endpoints
func RequireSignature(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			signature := r.Header.Get(SignatureHeader)
			timestampStr := r.Header.Get(TimestampHeader)

			if signature == "" {
				http.Error(w, "Signature required", http.StatusUnauthorized)
				return
			}
			if timestampStr == "" {
				http.Error(w, "Timestamp required", http.StatusUnauthorized)
				return
			}

			// Delegate to the standard middleware
			SignatureMiddleware(database)(next).ServeHTTP(w, r)
		})
	}
}
