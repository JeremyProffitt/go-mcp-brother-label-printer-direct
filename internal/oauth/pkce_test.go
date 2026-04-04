package oauth

import (
	"testing"
)

func TestVerifyPKCE(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := GenerateCodeChallenge(verifier)

	if !VerifyPKCE(verifier, challenge) {
		t.Error("expected PKCE verification to pass")
	}

	if VerifyPKCE("wrong-verifier", challenge) {
		t.Error("expected PKCE verification to fail with wrong verifier")
	}

	if VerifyPKCE("", challenge) {
		t.Error("expected PKCE verification to fail with empty verifier")
	}

	if VerifyPKCE(verifier, "") {
		t.Error("expected PKCE verification to fail with empty challenge")
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	challenge := GenerateCodeChallenge("test-verifier")
	if challenge == "" {
		t.Error("expected non-empty challenge")
	}

	// Same verifier should produce same challenge
	challenge2 := GenerateCodeChallenge("test-verifier")
	if challenge != challenge2 {
		t.Error("expected same verifier to produce same challenge")
	}

	// Different verifier should produce different challenge
	challenge3 := GenerateCodeChallenge("different-verifier")
	if challenge == challenge3 {
		t.Error("expected different verifier to produce different challenge")
	}
}
