package store

import "testing"

func TestValidRedirectURI(t *testing.T) {
	client := &OAuthClient{
		RedirectURIs: []string{
			"https://claude.ai/oauth/callback",
			"http://localhost:3000/callback",
		},
	}

	tests := []struct {
		uri      string
		expected bool
	}{
		{"https://claude.ai/oauth/callback", true},
		{"http://localhost:3000/callback", true},
		{"http://localhost:8080/callback", true}, // localhost wildcard
		{"https://evil.com/callback", false},
		{"", false},
	}

	for _, tt := range tests {
		got := ValidRedirectURI(client, tt.uri)
		if got != tt.expected {
			t.Errorf("ValidRedirectURI(%q) = %v, want %v", tt.uri, got, tt.expected)
		}
	}
}
