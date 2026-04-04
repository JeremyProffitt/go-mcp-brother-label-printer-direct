package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"go-mcp-brother-label-printer-direct/internal/store"
)

const loginPageHTML = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>MCP Printer - Login</title>
    <style>
        body { font-family: -apple-system, system-ui, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #f5f5f5; }
        .card { background: white; padding: 2rem; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); max-width: 400px; width: 90%; }
        h1 { margin-top: 0; font-size: 1.5rem; color: #333; }
        label { display: block; margin-bottom: 0.5rem; color: #555; font-weight: 500; }
        input { width: 100%; padding: 0.75rem; border: 1px solid #ddd; border-radius: 4px; font-size: 1rem; box-sizing: border-box; margin-bottom: 1rem; }
        .password-wrap { position: relative; }
        .password-wrap input { padding-right: 3rem; margin-bottom: 1rem; }
        .toggle-pw { position: absolute; right: 0.5rem; top: 0.6rem; background: none; border: none; color: #888; cursor: pointer; font-size: 1.1rem; padding: 0.2rem; width: auto; line-height: 1; }
        .toggle-pw:hover { color: #333; }
        button[type="submit"] { width: 100%; padding: 0.75rem; background: #2563eb; color: white; border: none; border-radius: 4px; font-size: 1rem; cursor: pointer; }
        button[type="submit"]:hover { background: #1d4ed8; }
        .error { color: #dc2626; margin-bottom: 1rem; }
        .info { color: #666; font-size: 0.875rem; margin-bottom: 1rem; }
    </style>
</head>
<body>
    <div class="card">
        <h1>MCP Printer Direct</h1>
        {{ERROR}}
        <p class="info">Authorize <strong>{{CLIENT_NAME}}</strong> to access printer tools.</p>
        <form method="POST" action="/authorize">
            <input type="hidden" name="client_id" value="{{CLIENT_ID}}">
            <input type="hidden" name="redirect_uri" value="{{REDIRECT_URI}}">
            <input type="hidden" name="state" value="{{STATE}}">
            <input type="hidden" name="code_challenge" value="{{CODE_CHALLENGE}}">
            <input type="hidden" name="scope" value="{{SCOPE}}">
            <label for="username">Username</label>
            <input type="text" id="username" name="username" required autocomplete="username">
            <label for="password">Password</label>
            <div class="password-wrap">
                <input type="password" id="password" name="password" required autocomplete="current-password">
                <button type="button" class="toggle-pw" onclick="var p=document.getElementById('password'),s=this.querySelector('.eye-show'),h=this.querySelector('.eye-hide');if(p.type==='password'){p.type='text';s.style.display='none';h.style.display='inline';}else{p.type='password';s.style.display='inline';h.style.display='none';}"><svg class="eye-show" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg><svg class="eye-hide" style="display:none" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg></button>
            </div>
            <button type="submit">Authorize</button>
        </form>
    </div>
</body>
</html>`

func (h *Handler) AuthorizeGet(c *fiber.Ctx) error {
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	state := c.Query("state")
	codeChallenge := c.Query("code_challenge")
	codeChallengeMethod := c.Query("code_challenge_method")
	scope := c.Query("scope")

	if clientID == "" || redirectURI == "" {
		return invalidRequest(c, "client_id and redirect_uri are required")
	}

	if codeChallengeMethod != "" && codeChallengeMethod != "S256" {
		return invalidRequest(c, "only S256 code_challenge_method is supported")
	}

	client, err := h.Store.GetClient(c.Context(), clientID)
	if err != nil {
		slog.Error("failed to get client", "error", err)
		return serverError(c, "internal error")
	}
	if client == nil {
		return invalidClient(c, "client not found")
	}

	if !store.ValidRedirectURI(client, redirectURI) {
		slog.Warn("invalid redirect_uri", "client_id", clientID, "redirect_uri", redirectURI)
		return invalidRequest(c, "redirect_uri not registered for this client")
	}

	clientName := client.ClientName
	if clientName == "" {
		clientName = clientID
	}

	return renderLoginPage(c, clientName, clientID, redirectURI, state, codeChallenge, scope, "")
}

func (h *Handler) AuthorizePost(c *fiber.Ctx) error {
	clientID := c.FormValue("client_id")
	redirectURI := c.FormValue("redirect_uri")
	state := c.FormValue("state")
	codeChallenge := c.FormValue("code_challenge")
	scope := c.FormValue("scope")
	username := c.FormValue("username")
	password := c.FormValue("password")

	if clientID == "" || redirectURI == "" {
		return invalidRequest(c, "client_id and redirect_uri are required")
	}

	client, err := h.Store.GetClient(c.Context(), clientID)
	if err != nil {
		slog.Error("failed to get client", "error", err)
		return serverError(c, "internal error")
	}
	if client == nil {
		return invalidClient(c, "client not found")
	}

	if !store.ValidRedirectURI(client, redirectURI) {
		slog.Warn("invalid redirect_uri in POST", "client_id", clientID, "redirect_uri", redirectURI)
		return invalidRequest(c, "redirect_uri not registered for this client")
	}

	if username != h.Config.AdminUser {
		clientName := client.ClientName
		if clientName == "" {
			clientName = clientID
		}
		slog.Warn("login attempt with wrong username", "username", username)
		return renderLoginPage(c, clientName, clientID, redirectURI, state, codeChallenge, scope, "Invalid credentials")
	}

	if !checkPassword(h.Config.AdminPassword, password) {
		clientName := client.ClientName
		if clientName == "" {
			clientName = clientID
		}
		slog.Warn("login attempt with wrong password", "username", username)
		return renderLoginPage(c, clientName, clientID, redirectURI, state, codeChallenge, scope, "Invalid credentials")
	}

	codeBytes := make([]byte, 32)
	if _, err := rand.Read(codeBytes); err != nil {
		return serverError(c, "failed to generate authorization code")
	}
	code := hex.EncodeToString(codeBytes)

	authCode := &store.AuthCode{
		Code:          code,
		ClientID:      clientID,
		RedirectURI:   redirectURI,
		CodeChallenge: codeChallenge,
		Scope:         scope,
		UserID:        username,
		ExpiresAt:     time.Now().Add(h.Config.AuthCodeTTL),
	}

	if err := h.Store.SaveAuthCode(c.Context(), authCode); err != nil {
		slog.Error("failed to save auth code", "error", err)
		return serverError(c, "failed to create authorization code")
	}

	slog.Info("authorization code issued", "client_id", clientID, "user_id", username, "scope", scope)

	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		return invalidRequest(c, "invalid redirect_uri")
	}
	q := redirectURL.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	redirectURL.RawQuery = q.Encode()

	return c.Redirect(redirectURL.String(), fiber.StatusFound)
}

func renderLoginPage(c *fiber.Ctx, clientName, clientID, redirectURI, state, codeChallenge, scope, errorMsg string) error {
	page := loginPageHTML
	errorHTML := ""
	if errorMsg != "" {
		errorHTML = fmt.Sprintf(`<p class="error">%s</p>`, html.EscapeString(errorMsg))
	}

	replacer := map[string]string{
		"{{ERROR}}":          errorHTML,
		"{{CLIENT_NAME}}":    html.EscapeString(clientName),
		"{{CLIENT_ID}}":      html.EscapeString(clientID),
		"{{REDIRECT_URI}}":   html.EscapeString(redirectURI),
		"{{STATE}}":          html.EscapeString(state),
		"{{CODE_CHALLENGE}}": html.EscapeString(codeChallenge),
		"{{SCOPE}}":          html.EscapeString(scope),
	}

	for k, v := range replacer {
		page = replaceAll(page, k, v)
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(page)
}

func replaceAll(s, old, new string) string {
	for {
		i := indexOf(s, old)
		if i < 0 {
			return s
		}
		s = s[:i] + new + s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func checkPassword(stored, provided string) bool {
	if len(stored) > 0 && stored[0] == '$' {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(provided)) == nil
	}
	slog.Warn("ADMIN_PASSWORD is not bcrypt-hashed; set a bcrypt hash for production security")
	if len(stored) != len(provided) {
		return false
	}
	var mismatch byte
	for i := 0; i < len(stored); i++ {
		mismatch |= stored[i] ^ provided[i]
	}
	return mismatch == 0
}

func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
