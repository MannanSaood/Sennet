// Package auth provides Firebase Authentication integration
package auth

import (
	"context"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// FirebaseAuth wraps the Firebase Admin SDK auth client
type FirebaseAuth struct {
	client *auth.Client
}

// NewFirebaseAuth creates a new Firebase Auth client
// It reads credentials from:
// 1. FIREBASE_SERVICE_ACCOUNT_JSON env var (base64 encoded JSON)
// 2. FIREBASE_SERVICE_ACCOUNT_PATH env var (path to JSON file)
// 3. GOOGLE_APPLICATION_CREDENTIALS env var (GCP default)
func NewFirebaseAuth() (*FirebaseAuth, error) {
	ctx := context.Background()
	var app *firebase.App
	var err error

	// Try FIREBASE_SERVICE_ACCOUNT_JSON first (for deployments like Render)
	if saJSON := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"); saJSON != "" {
		opt := option.WithCredentialsJSON([]byte(saJSON))
		app, err = firebase.NewApp(ctx, nil, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Firebase with JSON: %w", err)
		}
	} else if saPath := os.Getenv("FIREBASE_SERVICE_ACCOUNT_PATH"); saPath != "" {
		// Try file path
		opt := option.WithCredentialsFile(saPath)
		app, err = firebase.NewApp(ctx, nil, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Firebase with file: %w", err)
		}
	} else {
		// Fall back to Application Default Credentials
		app, err = firebase.NewApp(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Firebase with default credentials: %w", err)
		}
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Firebase Auth client: %w", err)
	}

	return &FirebaseAuth{client: client}, nil
}

// VerifyToken verifies a Firebase ID token and returns the decoded token
func (fa *FirebaseAuth) VerifyToken(ctx context.Context, idToken string) (*auth.Token, error) {
	token, err := fa.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("invalid ID token: %w", err)
	}
	return token, nil
}

// GetUser retrieves a user by their UID
func (fa *FirebaseAuth) GetUser(ctx context.Context, uid string) (*auth.UserRecord, error) {
	return fa.client.GetUser(ctx, uid)
}

// GetUserByEmail retrieves a user by their email
func (fa *FirebaseAuth) GetUserByEmail(ctx context.Context, email string) (*auth.UserRecord, error) {
	return fa.client.GetUserByEmail(ctx, email)
}

// CreateUser creates a new user in Firebase Auth
func (fa *FirebaseAuth) CreateUser(ctx context.Context, email, password, displayName string) (*auth.UserRecord, error) {
	params := (&auth.UserToCreate{}).
		Email(email).
		Password(password).
		DisplayName(displayName).
		EmailVerified(false)

	return fa.client.CreateUser(ctx, params)
}

// SetCustomClaims sets custom claims on a user (e.g., role)
func (fa *FirebaseAuth) SetCustomClaims(ctx context.Context, uid string, claims map[string]interface{}) error {
	return fa.client.SetCustomUserClaims(ctx, uid, claims)
}

// RevokeTokens revokes all refresh tokens for a user
func (fa *FirebaseAuth) RevokeTokens(ctx context.Context, uid string) error {
	return fa.client.RevokeRefreshTokens(ctx, uid)
}
