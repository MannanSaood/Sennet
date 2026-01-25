package auth

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
)

// ContextKey type for context values
type ContextKey string

const (
	// FirebaseUIDKey is the context key for Firebase UID
	FirebaseUIDKey ContextKey = "firebase_uid"
	// FirebaseEmailKey is the context key for Firebase email
	FirebaseEmailKey ContextKey = "firebase_email"
	// FirebaseTokenKey is the context key for the full Firebase token
	FirebaseTokenKey ContextKey = "firebase_token"
)

// FirebaseMiddleware creates HTTP middleware that verifies Firebase ID tokens
func FirebaseMiddleware(fa *FirebaseAuth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "Invalid authorization format, expected 'Bearer <token>'", http.StatusUnauthorized)
				return
			}

			idToken := strings.TrimPrefix(authHeader, "Bearer ")
			if idToken == "" {
				http.Error(w, "Empty token", http.StatusUnauthorized)
				return
			}

			// Verify the token
			token, err := fa.VerifyToken(r.Context(), idToken)
			if err != nil {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Add user info to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, FirebaseUIDKey, token.UID)
			if email, ok := token.Claims["email"].(string); ok {
				ctx = context.WithValue(ctx, FirebaseEmailKey, email)
			}
			ctx = context.WithValue(ctx, FirebaseTokenKey, token)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetFirebaseUID extracts the Firebase UID from the request context
func GetFirebaseUID(ctx context.Context) string {
	if uid, ok := ctx.Value(FirebaseUIDKey).(string); ok {
		return uid
	}
	return ""
}

// GetFirebaseEmail extracts the Firebase email from the request context
func GetFirebaseEmail(ctx context.Context) string {
	if email, ok := ctx.Value(FirebaseEmailKey).(string); ok {
		return email
	}
	return ""
}

// GetFirebaseToken extracts the full Firebase token from the request context
func GetFirebaseToken(ctx context.Context) *auth.Token {
	if token, ok := ctx.Value(FirebaseTokenKey).(*auth.Token); ok {
		return token
	}
	return nil
}

// RequireRole creates middleware that checks for a specific custom claim role
func RequireRole(fa *FirebaseAuth, role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := GetFirebaseToken(r.Context())
			if token == nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			// Check custom claims for role
			if claims, ok := token.Claims["role"].(string); ok {
				if claims == role || claims == "admin" {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Insufficient permissions", http.StatusForbidden)
		})
	}
}
