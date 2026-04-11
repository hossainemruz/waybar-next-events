package auth

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateState(t *testing.T) {
	// Test that GenerateState returns a non-empty string
	state1, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState() error = %v", err)
	}
	if state1 == "" {
		t.Error("GenerateState() returned empty string")
	}

	// Decode and check entropy
	decoded, err := base64.RawURLEncoding.DecodeString(state1)
	if err != nil {
		t.Fatalf("GenerateState() returned invalid base64: %v", err)
	}
	if len(decoded) != stateEntropyBytes {
		t.Errorf("GenerateState() returned %d bytes, want %d", len(decoded), stateEntropyBytes)
	}

	// Test that multiple calls return different values
	state2, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState() second call error = %v", err)
	}
	if state1 == state2 {
		t.Error("GenerateState() returned same value twice")
	}
}

func TestGeneratePKCE(t *testing.T) {
	pkce1, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error = %v", err)
	}

	// Check verifier
	if pkce1.Verifier == "" {
		t.Error("GeneratePKCE() returned empty verifier")
	}

	// Decode verifier
	verifierBytes, err := base64.RawURLEncoding.DecodeString(pkce1.Verifier)
	if err != nil {
		t.Fatalf("GeneratePKCE() verifier is invalid base64: %v", err)
	}
	if len(verifierBytes) != verifierEntropyBytes {
		t.Errorf("GeneratePKCE() verifier has %d bytes, want %d", len(verifierBytes), verifierEntropyBytes)
	}

	// Check challenge
	if pkce1.Challenge == "" {
		t.Error("GeneratePKCE() returned empty challenge")
	}

	// Check method
	if pkce1.Method != "S256" {
		t.Errorf("GeneratePKCE() method = %s, want S256", pkce1.Method)
	}

	// Verify challenge is correct base64
	_, err = base64.RawURLEncoding.DecodeString(pkce1.Challenge)
	if err != nil {
		t.Errorf("GeneratePKCE() challenge is invalid base64: %v", err)
	}

	// Test that multiple calls return different values
	pkce2, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() second call error = %v", err)
	}
	if pkce1.Verifier == pkce2.Verifier {
		t.Error("GeneratePKCE() returned same verifier twice")
	}
	if pkce1.Challenge == pkce2.Challenge {
		t.Error("GeneratePKCE() returned same challenge twice")
	}
}

func TestPKCEChallengeDerivation(t *testing.T) {
	// Test that S256 challenge is correctly derived using RFC 7636 Appendix B test vector.
	// verifier: "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	// S256 challenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	testVerifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	expectedChallenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	actualChallenge := generateS256Challenge(testVerifier)
	if actualChallenge != expectedChallenge {
		t.Errorf("S256 challenge mismatch: got %s, want %s", actualChallenge, expectedChallenge)
	}
}

func TestPKCEVerifierLength(t *testing.T) {
	// Test that verifier is the correct length (base64url encoded)
	pkce, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error = %v", err)
	}

	// Base64url encoding of 32 bytes should be 43 characters (no padding)
	expectedLength := base64.RawURLEncoding.EncodedLen(verifierEntropyBytes)
	if len(pkce.Verifier) != expectedLength {
		t.Errorf("verifier length = %d, want %d", len(pkce.Verifier), expectedLength)
	}

	// Challenge is SHA256 hash (32 bytes), base64url encoded = 43 chars
	expectedChallengeLength := base64.RawURLEncoding.EncodedLen(32)
	if len(pkce.Challenge) != expectedChallengeLength {
		t.Errorf("challenge length = %d, want %d", len(pkce.Challenge), expectedChallengeLength)
	}
}

func TestPKCENoPadding(t *testing.T) {
	// Ensure base64url encoding without padding is used
	pkce, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error = %v", err)
	}

	// Raw URL encoding should not contain padding characters
	if strings.Contains(pkce.Verifier, "=") {
		t.Error("verifier contains padding character '='")
	}
	if strings.Contains(pkce.Verifier, "+") {
		t.Error("verifier contains '+' character, should use '-' for URL encoding")
	}
	if strings.Contains(pkce.Verifier, "/") {
		t.Error("verifier contains '/' character, should use '_' for URL encoding")
	}

	if strings.Contains(pkce.Challenge, "=") {
		t.Error("challenge contains padding character '='")
	}
}
