package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"

	"stronghold/internal/db"

	"github.com/gofiber/fiber/v3"
)

// APIKeyMiddleware handles API key authentication for B2B accounts
type APIKeyMiddleware struct {
	db *db.DB
}

// NewAPIKeyMiddleware creates a new API key middleware
func NewAPIKeyMiddleware(database *db.DB) *APIKeyMiddleware {
	return &APIKeyMiddleware{db: database}
}

// Authenticate validates an API key from the Authorization header and loads the associated account.
// Returns the account and API key on success, or an error fiber response.
func (m *APIKeyMiddleware) Authenticate(c fiber.Ctx) (*db.Account, *db.APIKey, error) {
	// Extract Bearer token
	authHeader := string(c.Request().Header.Peek("Authorization"))
	if authHeader == "" {
		return nil, nil, fiber.NewError(fiber.StatusUnauthorized, "Missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid authorization header")
	}

	apiKeyStr := parts[1]
	if !strings.HasPrefix(apiKeyStr, "sk_live_") {
		return nil, nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid API key format")
	}

	// Hash the key
	hash := sha256.Sum256([]byte(apiKeyStr))
	keyHash := hex.EncodeToString(hash[:])

	// Look up key
	apiKey, err := m.db.GetAPIKeyByHash(c.Context(), keyHash)
	if err != nil {
		return nil, nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid API key")
	}

	// Load associated account
	account, err := m.db.GetAccountByID(c.Context(), apiKey.AccountID)
	if err != nil {
		return nil, nil, fiber.NewError(fiber.StatusUnauthorized, "Account not found")
	}

	// Verify account is B2B and active
	if account.AccountType != db.AccountTypeB2B {
		return nil, nil, fiber.NewError(fiber.StatusForbidden, "API keys require a business account")
	}
	if account.Status != db.AccountStatusActive {
		return nil, nil, fiber.NewError(fiber.StatusForbidden, "Account is not active")
	}

	// Store in context
	c.Locals("account_id", account.ID.String())
	c.Locals("api_key_id", apiKey.ID.String())

	// Async update last_used_at (use background context since Fiber's
	// request context is recycled after the handler returns)
	keyID := apiKey.ID
	go func() {
		if err := m.db.UpdateAPIKeyLastUsed(context.Background(), keyID); err != nil {
			slog.Debug("failed to update API key last_used_at", "error", err)
		}
	}()

	return account, apiKey, nil
}
