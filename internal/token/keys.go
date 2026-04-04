package token

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

type jwtKeySecret struct {
	Seed string `json:"seed"`
}

func LoadKeyPair(ctx context.Context, smClient *secretsmanager.Client, secretARN string) (*KeyPair, error) {
	var seed []byte

	if secretARN == "" {
		slog.Warn("JWT_SIGNING_KEY_ARN not set, using deterministic development key")
		h := sha256.Sum256([]byte("mcp-brother-label-printer-direct-dev-key"))
		seed = h[:]
	} else {
		result, err := smClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secretARN),
		})
		if err != nil {
			return nil, fmt.Errorf("get JWT signing key secret: %w", err)
		}

		var secret jwtKeySecret
		if err := json.Unmarshal([]byte(*result.SecretString), &secret); err != nil {
			h := sha256.Sum256([]byte(*result.SecretString))
			seed = h[:]
		} else {
			h := sha256.Sum256([]byte(secret.Seed))
			seed = h[:]
		}
	}

	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}
