package oauth

import (
	"go-mcp-brother-label-printer-direct/internal/config"
	"go-mcp-brother-label-printer-direct/internal/store"
	"go-mcp-brother-label-printer-direct/internal/token"

	"github.com/gofiber/fiber/v2"
)

// Handler holds dependencies for all OAuth endpoints.
type Handler struct {
	Config  *config.Config
	Store   store.Store
	KeyPair *token.KeyPair
}

// RegisterRoutes sets up all OAuth-related routes on the Fiber app.
func (h *Handler) RegisterRoutes(app *fiber.App) {
	app.Get("/.well-known/oauth-authorization-server", h.Metadata)
	app.Get("/.well-known/oauth-protected-resource", h.ProtectedResourceMetadata)
	app.Post("/register", h.Register)
	app.Get("/authorize", h.AuthorizeGet)
	app.Post("/authorize", h.AuthorizePost)
	app.Post("/token", h.Token)
}
