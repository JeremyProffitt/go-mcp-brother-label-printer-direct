package oauth

import "github.com/gofiber/fiber/v2"

func (h *Handler) Metadata(c *fiber.Ctx) error {
	base := h.Config.PublicURL

	return c.JSON(fiber.Map{
		"issuer":                                base,
		"authorization_endpoint":                base + "/authorize",
		"token_endpoint":                        base + "/token",
		"registration_endpoint":                 base + "/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"code_challenge_methods_supported":      []string{"S256"},
		"scopes_supported":                      []string{"mcp:read", "mcp:write"},
	})
}

func (h *Handler) ProtectedResourceMetadata(c *fiber.Ctx) error {
	base := h.Config.PublicURL

	return c.JSON(fiber.Map{
		"resource":                base,
		"authorization_servers":   []string{base},
		"scopes_supported":       []string{"mcp:read", "mcp:write"},
		"bearer_methods_supported": []string{"header"},
	})
}
