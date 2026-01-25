package middleware

import (
	"context"
	"log"
	"net/http"
	"time"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	Timestamp  time.Time
	UserID     string
	Email      string
	Method     string
	Path       string
	StatusCode int
	Duration   time.Duration
	IP         string
	UserAgent  string
}

// AuditLogger is a function type for logging audit events
type AuditLogger func(log AuditLog)

// DefaultAuditLogger logs to standard logger
func DefaultAuditLogger() AuditLogger {
	return func(entry AuditLog) {
		log.Printf("AUDIT user=%s email=%s method=%s path=%s status=%d duration=%s ip=%s",
			entry.UserID,
			entry.Email,
			entry.Method,
			entry.Path,
			entry.StatusCode,
			entry.Duration,
			entry.IP,
		)
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type auditResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *auditResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// AuditMiddleware creates middleware that logs all requests
func AuditMiddleware(logger AuditLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status
			wrapped := &auditResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Extract user info from context (set by auth middleware)
			userID := ""
			email := ""
			if uid := r.Context().Value("firebase_uid"); uid != nil {
				userID = uid.(string)
			}
			if e := r.Context().Value("firebase_email"); e != nil {
				email = e.(string)
			}

			// Log the audit entry
			logger(AuditLog{
				Timestamp:  start,
				UserID:     userID,
				Email:      email,
				Method:     r.Method,
				Path:       r.URL.Path,
				StatusCode: wrapped.statusCode,
				Duration:   time.Since(start),
				IP:         getClientIP(r),
				UserAgent:  r.UserAgent(),
			})
		})
	}
}

// AuditMiddlewareWithContext extracts user info using provided context keys
func AuditMiddlewareWithContext(logger AuditLogger, uidKey, emailKey interface{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &auditResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			userID := getContextString(r.Context(), uidKey)
			email := getContextString(r.Context(), emailKey)

			logger(AuditLog{
				Timestamp:  start,
				UserID:     userID,
				Email:      email,
				Method:     r.Method,
				Path:       r.URL.Path,
				StatusCode: wrapped.statusCode,
				Duration:   time.Since(start),
				IP:         getClientIP(r),
				UserAgent:  r.UserAgent(),
			})
		})
	}
}

func getContextString(ctx context.Context, key interface{}) string {
	if val := ctx.Value(key); val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}
