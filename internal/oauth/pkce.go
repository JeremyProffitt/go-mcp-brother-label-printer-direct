package oauth

import (
	"crypto/sha256"
	"encoding/base64"
)

// VerifyPKCE verifies a PKCE code verifier against the stored code challenge (S256 method).
func VerifyPKCE(codeVerifier, codeChallenge string) bool {
	if codeVerifier == "" || codeChallenge == "" {
		return false
	}
	h := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])
	return computed == codeChallenge
}
