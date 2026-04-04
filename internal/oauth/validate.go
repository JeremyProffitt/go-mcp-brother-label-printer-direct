package oauth

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"go-mcp-brother-label-printer-direct/internal/token"
)

// BearerAuthMiddleware validates the Bearer token on protected routes.
func BearerAuthMiddleware(kp *token.KeyPair) fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing Authorization header",
			})
		}

		if !strings.HasPrefix(auth, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid Authorization header format",
			})
		}

		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims, err := token.ValidateAccessToken(kp, tokenStr)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":       "invalid_token",
				"description": err.Error(),
			})
		}

		c.Locals("user_id", claims.Subject)
		c.Locals("client_id", claims.ClientID)
		c.Locals("scope", claims.Scope)

		return c.Next()
	}
}
