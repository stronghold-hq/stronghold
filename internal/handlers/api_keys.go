package handlers

import (
	"errors"
	"stronghold/internal/db"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// APIKeyHandler handles API key management endpoints
type APIKeyHandler struct {
	db *db.DB
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(database *db.DB) *APIKeyHandler {
	return &APIKeyHandler{db: database}
}

// RegisterRoutes registers API key management routes
func (h *APIKeyHandler) RegisterRoutes(app *fiber.App, authHandler *AuthHandler) {
	group := app.Group("/v1/account/api-keys")
	group.Post("/", authHandler.AuthMiddleware(), authHandler.RequireTrustedDevice(), h.CreateAPIKey)
	group.Get("/", authHandler.AuthMiddleware(), authHandler.RequireTrustedDevice(), h.ListAPIKeys)
	group.Delete("/:id", authHandler.AuthMiddleware(), authHandler.RequireTrustedDevice(), h.RevokeAPIKey)
}

// CreateAPIKeyRequest represents a request to create an API key
type CreateAPIKeyRequest struct {
	Label string `json:"label"`
}

// CreateAPIKey creates a new API key for the authenticated account
func (h *APIKeyHandler) CreateAPIKey(c fiber.Ctx) error {
	accountIDStr, _ := c.Locals("account_id").(string)
	if accountIDStr == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Not authenticated",
		})
	}

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid account ID",
		})
	}

	var req CreateAPIKeyRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	apiKey, rawKey, err := h.db.CreateAPIKey(c.Context(), accountID, req.Label)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create API key",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"key":        rawKey,
		"id":         apiKey.ID,
		"key_prefix": apiKey.KeyPrefix,
		"label":      apiKey.Label,
		"created_at": apiKey.CreatedAt,
	})
}

// ListAPIKeys returns all API keys for the authenticated account
func (h *APIKeyHandler) ListAPIKeys(c fiber.Ctx) error {
	accountIDStr, _ := c.Locals("account_id").(string)
	if accountIDStr == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Not authenticated",
		})
	}

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid account ID",
		})
	}

	keys, err := h.db.ListAPIKeys(c.Context(), accountID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list API keys",
		})
	}

	// Return empty array instead of null
	if keys == nil {
		keys = []*db.APIKey{}
	}

	return c.JSON(fiber.Map{
		"api_keys": keys,
	})
}

// RevokeAPIKey revokes an API key by ID
func (h *APIKeyHandler) RevokeAPIKey(c fiber.Ctx) error {
	accountIDStr, _ := c.Locals("account_id").(string)
	if accountIDStr == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Not authenticated",
		})
	}

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid account ID",
		})
	}

	keyIDStr := c.Params("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid API key ID",
		})
	}

	if err := h.db.RevokeAPIKey(c.Context(), accountID, keyID); err != nil {
		if errors.Is(err, db.ErrAPIKeyNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "API key not found or already revoked",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to revoke API key",
		})
	}

	return c.JSON(fiber.Map{
		"message": "API key revoked",
	})
}
