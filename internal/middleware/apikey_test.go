package middleware

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"stronghold/internal/db"
	"stronghold/internal/db/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	database := db.NewFromPool(testDB.Pool)
	ctx := context.Background()

	// Create account and API key
	account, err := database.CreateAccount(ctx, nil, nil)
	require.NoError(t, err)

	_, rawKey, err := database.CreateAPIKey(ctx, account.ID, "test key")
	require.NoError(t, err)

	m := NewAPIKeyMiddleware(database)

	var capturedAccountID string
	var capturedAuthMethod string

	app := fiber.New()
	app.Post("/test", m.Authenticate(), func(c fiber.Ctx) error {
		capturedAccountID, _ = c.Locals("account_id").(string)
		capturedAuthMethod, _ = c.Locals("auth_method").(string)
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-API-Key", rawKey)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, account.ID.String(), capturedAccountID, "account_id should be set in locals")
	assert.Equal(t, "api_key", capturedAuthMethod, "auth_method should be api_key")
}

func TestAPIKeyMiddleware_MissingHeader(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	database := db.NewFromPool(testDB.Pool)

	m := NewAPIKeyMiddleware(database)

	app := fiber.New()
	app.Post("/test", m.Authenticate(), func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	// No X-API-Key header

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 401, resp.StatusCode)

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)
	assert.Contains(t, body["error"], "API key required")
}

func TestAPIKeyMiddleware_InvalidKey(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	database := db.NewFromPool(testDB.Pool)

	m := NewAPIKeyMiddleware(database)

	app := fiber.New()
	app.Post("/test", m.Authenticate(), func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-API-Key", "sh_live_invalid_key_that_does_not_exist")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 401, resp.StatusCode)

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)
	assert.Contains(t, body["error"], "Invalid or revoked API key")
}

func TestAPIKeyMiddleware_RevokedKey(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	database := db.NewFromPool(testDB.Pool)
	ctx := context.Background()

	// Create account and API key
	account, err := database.CreateAccount(ctx, nil, nil)
	require.NoError(t, err)

	apiKey, rawKey, err := database.CreateAPIKey(ctx, account.ID, "revoked key")
	require.NoError(t, err)

	// Revoke the key
	err = database.RevokeAPIKey(ctx, account.ID, apiKey.ID)
	require.NoError(t, err)

	m := NewAPIKeyMiddleware(database)

	app := fiber.New()
	app.Post("/test", m.Authenticate(), func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-API-Key", rawKey)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 401, resp.StatusCode)

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)
	assert.Contains(t, body["error"], "Invalid or revoked API key")
}

func TestAPIKeyMiddleware_EmptyKey(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	database := db.NewFromPool(testDB.Pool)

	m := NewAPIKeyMiddleware(database)

	app := fiber.New()
	app.Post("/test", m.Authenticate(), func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-API-Key", "")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 401, resp.StatusCode)
}
