// Package middleware provides HTTP/gRPC middleware for Sennet
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/sennet/sennet/backend/db"
)

// AuthInterceptor validates API keys on incoming requests
type AuthInterceptor struct {
	db *db.DB
}

// NewAuthInterceptor creates a new auth interceptor
func NewAuthInterceptor(database *db.DB) *AuthInterceptor {
	return &AuthInterceptor{db: database}
}

// WrapUnary implements connect.Interceptor for unary RPCs
func (a *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Extract API key from Authorization header
		authHeader := req.Header().Get("Authorization")
		apiKey, err := extractBearerToken(authHeader)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}

		// Validate key against database
		valid, err := a.db.ValidateAPIKey(apiKey)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to validate API key"))
		}
		if !valid {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid API key"))
		}

		// Key is valid, proceed with request
		return next(ctx, req)
	}
}

// WrapStreamingClient implements connect.Interceptor (not used for server)
func (a *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements connect.Interceptor for streaming RPCs
func (a *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		// Extract API key from Authorization header
		authHeader := conn.RequestHeader().Get("Authorization")
		apiKey, err := extractBearerToken(authHeader)
		if err != nil {
			return connect.NewError(connect.CodeUnauthenticated, err)
		}

		// Validate key against database
		valid, err := a.db.ValidateAPIKey(apiKey)
		if err != nil {
			return connect.NewError(connect.CodeInternal, errors.New("failed to validate API key"))
		}
		if !valid {
			return connect.NewError(connect.CodeUnauthenticated, errors.New("invalid API key"))
		}

		return next(ctx, conn)
	}
}

// extractBearerToken extracts the token from "Bearer <token>" format
func extractBearerToken(header string) (string, error) {
	if header == "" {
		return "", errors.New("missing Authorization header")
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return "", errors.New("malformed Authorization header")
	}

	if !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("Authorization header must use Bearer scheme")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("empty bearer token")
	}

	return token, nil
}

// NewHTTPAuthMiddleware creates an HTTP middleware wrapper that validates API keys
func NewHTTPAuthMiddleware(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			apiKey, err := extractBearerToken(authHeader)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			valid, err := database.ValidateAPIKey(apiKey)
			if err != nil {
				http.Error(w, "failed to validate API key", http.StatusInternalServerError)
				return
			}
			if !valid {
				http.Error(w, "invalid API key", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
