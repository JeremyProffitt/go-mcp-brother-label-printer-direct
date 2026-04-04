package token

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"
)

func testKeyPair() *KeyPair {
	h := sha256.Sum256([]byte("test-key-seed"))
	priv := ed25519.NewKeyFromSeed(h[:])
	pub := priv.Public().(ed25519.PublicKey)
	return &KeyPair{PrivateKey: priv, PublicKey: pub}
}

func TestIssueAndValidateAccessToken(t *testing.T) {
	kp := testKeyPair()

	tokenStr, err := IssueAccessToken(kp, "https://test.example.com", "user1", "client1", "mcp:read", time.Hour)
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	if tokenStr == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := ValidateAccessToken(kp, tokenStr)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.Subject != "user1" {
		t.Errorf("expected subject 'user1', got %q", claims.Subject)
	}
	if claims.ClientID != "client1" {
		t.Errorf("expected client_id 'client1', got %q", claims.ClientID)
	}
	if claims.Scope != "mcp:read" {
		t.Errorf("expected scope 'mcp:read', got %q", claims.Scope)
	}
	if claims.Issuer != "https://test.example.com" {
		t.Errorf("expected issuer 'https://test.example.com', got %q", claims.Issuer)
	}
}

func TestExpiredToken(t *testing.T) {
	kp := testKeyPair()

	// Issue token with negative TTL (already expired)
	tokenStr, err := IssueAccessToken(kp, "https://test.example.com", "user1", "client1", "", -time.Hour)
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	_, err = ValidateAccessToken(kp, tokenStr)
	if err == nil {
		t.Error("expected validation to fail for expired token")
	}
}

func TestInvalidToken(t *testing.T) {
	kp := testKeyPair()

	_, err := ValidateAccessToken(kp, "invalid-token-string")
	if err == nil {
		t.Error("expected validation to fail for invalid token")
	}
}

func TestWrongKey(t *testing.T) {
	kp1 := testKeyPair()

	// Create a different key pair
	h := sha256.Sum256([]byte("different-key-seed"))
	priv := ed25519.NewKeyFromSeed(h[:])
	pub := priv.Public().(ed25519.PublicKey)
	kp2 := &KeyPair{PrivateKey: priv, PublicKey: pub}

	tokenStr, err := IssueAccessToken(kp1, "https://test.example.com", "user1", "client1", "", time.Hour)
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	_, err = ValidateAccessToken(kp2, tokenStr)
	if err == nil {
		t.Error("expected validation to fail with wrong key")
	}
}
