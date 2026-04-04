package oauth

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"

	"go-mcp-brother-label-printer-direct/internal/store"
)

type registrationRequest struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	GrantTypes              []string `json:"grant_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope"`
}

func (h *Handler) Register(c *fiber.Ctx) error {
	var req registrationRequest
	if err := c.BodyParser(&req); err != nil {
		return invalidRequest(c, "invalid request body")
	}

	if len(req.RedirectURIs) == 0 {
		return invalidRequest(c, "redirect_uris is required")
	}

	if len(req.GrantTypes) == 0 {
		req.GrantTypes = []string{"authorization_code"}
	}

	clientIDBytes := make([]byte, 16)
	if _, err := rand.Read(clientIDBytes); err != nil {
		slog.Error("failed to generate client ID", "error", err)
		return serverError(c, "failed to generate client ID")
	}
	clientID := hex.EncodeToString(clientIDBytes)

	client := &store.OAuthClient{
		ClientID:     clientID,
		ClientName:   req.ClientName,
		RedirectURIs: req.RedirectURIs,
		GrantTypes:   req.GrantTypes,
		Scope:        req.Scope,
		IssuedAt:     time.Now().Unix(),
	}

	if err := h.Store.SaveClient(c.Context(), client); err != nil {
		slog.Error("failed to save client", "error", err)
		return serverError(c, "failed to register client")
	}

	slog.Info("client registered", "client_id", clientID, "client_name", req.ClientName, "redirect_uris", req.RedirectURIs)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"client_id":                    clientID,
		"client_name":                  req.ClientName,
		"redirect_uris":               req.RedirectURIs,
		"grant_types":                 req.GrantTypes,
		"token_endpoint_auth_method":   "none",
		"client_id_issued_at":         client.IssuedAt,
	})
}
