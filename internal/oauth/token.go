package oauth

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"

	"go-mcp-brother-label-printer-direct/internal/store"
	"go-mcp-brother-label-printer-direct/internal/token"
)

func (h *Handler) Token(c *fiber.Ctx) error {
	grantType := c.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		return h.tokenAuthorizationCode(c)
	case "refresh_token":
		return h.tokenRefreshToken(c)
	default:
		return unsupportedGrantType(c)
	}
}

func (h *Handler) tokenAuthorizationCode(c *fiber.Ctx) error {
	code := c.FormValue("code")
	clientID := c.FormValue("client_id")
	redirectURI := c.FormValue("redirect_uri")
	codeVerifier := c.FormValue("code_verifier")

	if code == "" || clientID == "" {
		return invalidRequest(c, "code and client_id are required")
	}

	authCode, err := h.Store.GetAuthCode(c.Context(), code)
	if err != nil {
		slog.Error("failed to get auth code", "error", err)
		return serverError(c, "internal error")
	}
	if authCode == nil {
		return invalidGrant(c, "authorization code not found")
	}

	if authCode.Used {
		return invalidGrant(c, "authorization code already used")
	}
	if time.Now().After(authCode.ExpiresAt) {
		return invalidGrant(c, "authorization code expired")
	}
	if authCode.ClientID != clientID {
		return invalidGrant(c, "client_id mismatch")
	}
	if authCode.RedirectURI != "" && authCode.RedirectURI != redirectURI {
		return invalidGrant(c, "redirect_uri mismatch")
	}

	if authCode.CodeChallenge != "" {
		if !VerifyPKCE(codeVerifier, authCode.CodeChallenge) {
			return invalidGrant(c, "PKCE verification failed")
		}
	}

	if err := h.Store.MarkAuthCodeUsed(c.Context(), code); err != nil {
		slog.Error("failed to mark auth code used", "error", err)
	}

	return h.issueTokens(c, clientID, authCode.UserID, authCode.Scope)
}

func (h *Handler) tokenRefreshToken(c *fiber.Ctx) error {
	refreshTokenStr := c.FormValue("refresh_token")
	clientID := c.FormValue("client_id")

	if refreshTokenStr == "" {
		return invalidRequest(c, "refresh_token is required")
	}

	rt, err := h.Store.GetRefreshToken(c.Context(), refreshTokenStr)
	if err != nil {
		slog.Error("failed to get refresh token", "error", err)
		return serverError(c, "internal error")
	}
	if rt == nil {
		return invalidGrant(c, "refresh token not found")
	}
	if rt.Revoked {
		return invalidGrant(c, "refresh token revoked")
	}
	if time.Now().After(rt.ExpiresAt) {
		return invalidGrant(c, "refresh token expired")
	}
	if clientID != "" && rt.ClientID != clientID {
		return invalidGrant(c, "client_id mismatch")
	}

	if err := h.Store.RevokeRefreshToken(c.Context(), refreshTokenStr); err != nil {
		slog.Error("failed to revoke old refresh token", "error", err)
	}

	return h.issueTokens(c, rt.ClientID, rt.UserID, rt.Scope)
}

func (h *Handler) issueTokens(c *fiber.Ctx, clientID, userID, scope string) error {
	accessToken, err := token.IssueAccessToken(
		h.KeyPair,
		h.Config.PublicURL,
		userID,
		clientID,
		scope,
		h.Config.AccessTokenTTL,
	)
	if err != nil {
		slog.Error("failed to issue access token", "error", err)
		return serverError(c, "failed to issue token")
	}

	rtBytes := make([]byte, 32)
	if _, err := rand.Read(rtBytes); err != nil {
		return serverError(c, "failed to generate refresh token")
	}
	refreshTokenStr := hex.EncodeToString(rtBytes)

	rt := &store.RefreshToken{
		Token:     refreshTokenStr,
		ClientID:  clientID,
		UserID:    userID,
		Scope:     scope,
		ExpiresAt: time.Now().Add(h.Config.RefreshTokenTTL),
	}

	if err := h.Store.SaveRefreshToken(c.Context(), rt); err != nil {
		slog.Error("failed to save refresh token", "error", err)
		return serverError(c, "failed to save refresh token")
	}

	slog.Info("tokens issued", "client_id", clientID, "user_id", userID, "scope", scope)

	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    int(h.Config.AccessTokenTTL.Seconds()),
		"refresh_token": refreshTokenStr,
		"scope":         scope,
	})
}
