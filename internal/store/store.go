package store

import "context"

// Store defines the interface for OAuth state persistence.
type Store interface {
	SaveClient(ctx context.Context, client *OAuthClient) error
	GetClient(ctx context.Context, clientID string) (*OAuthClient, error)

	SaveAuthCode(ctx context.Context, code *AuthCode) error
	GetAuthCode(ctx context.Context, code string) (*AuthCode, error)
	MarkAuthCodeUsed(ctx context.Context, code string) error

	SaveRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, token string) error
}
