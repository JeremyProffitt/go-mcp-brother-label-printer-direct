package oauth

import "github.com/gofiber/fiber/v2"

type OAuthError struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

func oauthError(c *fiber.Ctx, status int, errCode, description string) error {
	return c.Status(status).JSON(OAuthError{
		Error:       errCode,
		Description: description,
	})
}

func invalidRequest(c *fiber.Ctx, desc string) error {
	return oauthError(c, fiber.StatusBadRequest, "invalid_request", desc)
}

func invalidClient(c *fiber.Ctx, desc string) error {
	return oauthError(c, fiber.StatusUnauthorized, "invalid_client", desc)
}

func invalidGrant(c *fiber.Ctx, desc string) error {
	return oauthError(c, fiber.StatusBadRequest, "invalid_grant", desc)
}

func unsupportedGrantType(c *fiber.Ctx) error {
	return oauthError(c, fiber.StatusBadRequest, "unsupported_grant_type", "only authorization_code and refresh_token are supported")
}

func serverError(c *fiber.Ctx, desc string) error {
	return oauthError(c, fiber.StatusInternalServerError, "server_error", desc)
}
