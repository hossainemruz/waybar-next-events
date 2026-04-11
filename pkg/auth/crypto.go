package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

const (
	// stateEntropyBytes is the number of random bytes for CSRF state.
	// 32 bytes = 256 bits of entropy.
	stateEntropyBytes = 32
	// verifierEntropyBytes is the number of random bytes for PKCE verifier.
	// RFC 7636 recommends at least 256 bits (32 bytes).
	verifierEntropyBytes = 32
)

// PKCE holds the PKCE verifier and its derived challenge.
type PKCE struct {
	Verifier  string
	Challenge string
	Method    string
}

// GeneratePKCE generates a new PKCE verifier and S256 challenge.
// Returns an error if random generation fails.
func GeneratePKCE() (*PKCE, error) {
	verifier, err := generateVerifier()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE verifier: %w", err)
	}

	challenge := generateS256Challenge(verifier)

	return &PKCE{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}

// generateVerifier generates a cryptographically secure random verifier.
// The verifier is base64url-encoded without padding.
func generateVerifier() (string, error) {
	b := make([]byte, verifierEntropyBytes)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generateS256Challenge generates the S256 code challenge from a verifier.
// challenge = BASE64URL(SHA256(verifier))
func generateS256Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// GenerateState generates a cryptographically secure random state parameter
// for CSRF protection. Returns base64url-encoded random bytes.
func GenerateState() (string, error) {
	b := make([]byte, stateEntropyBytes)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
