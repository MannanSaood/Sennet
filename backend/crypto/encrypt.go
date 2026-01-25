// Package crypto provides encryption utilities for sensitive data
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

var (
	// ErrInvalidCiphertext is returned when decryption fails
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	// ErrNoEncryptionKey is returned when encryption key is not configured
	ErrNoEncryptionKey = errors.New("ENCRYPTION_KEY environment variable not set")
)

// GetEncryptionKey retrieves the 32-byte encryption key from environment
// The key should be 32 bytes for AES-256
func GetEncryptionKey() ([]byte, error) {
	keyStr := os.Getenv("ENCRYPTION_KEY")
	if keyStr == "" {
		return nil, ErrNoEncryptionKey
	}

	// Decode from base64
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		// Try using as raw bytes if not base64
		key = []byte(keyStr)
	}

	// Pad or truncate to 32 bytes
	if len(key) < 32 {
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	} else if len(key) > 32 {
		key = key[:32]
	}

	return key, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
// Returns base64-encoded ciphertext
func Encrypt(plaintext []byte) (string, error) {
	key, err := GetEncryptionKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-256-GCM
func Decrypt(ciphertextB64 string) ([]byte, error) {
	key, err := GetEncryptionKey()
	if err != nil {
		return nil, err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, ErrInvalidCiphertext
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64-encoded ciphertext
func EncryptString(plaintext string) (string, error) {
	return Encrypt([]byte(plaintext))
}

// DecryptString decrypts base64-encoded ciphertext and returns a string
func DecryptString(ciphertextB64 string) (string, error) {
	plaintext, err := Decrypt(ciphertextB64)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// GenerateKey generates a new random 32-byte key for AES-256
// Returns the key as base64 for easy storage in environment variables
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
