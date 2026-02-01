package handlers

import (
	"encoding/json"
	"io"
	"log/slog"

	"stronghold/internal/config"
	"stronghold/internal/db"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82/webhook"
)

// StripeWebhookHandler handles Stripe webhook events
type StripeWebhookHandler struct {
	db           *db.DB
	stripeConfig *config.StripeConfig
}

// NewStripeWebhookHandler creates a new Stripe webhook handler
func NewStripeWebhookHandler(database *db.DB, stripeConfig *config.StripeConfig) *StripeWebhookHandler {
	return &StripeWebhookHandler{
		db:           database,
		stripeConfig: stripeConfig,
	}
}

// HandleWebhook handles incoming Stripe webhook events
func (h *StripeWebhookHandler) HandleWebhook(c fiber.Ctx) error {
	signature := c.Get("Stripe-Signature")
	if signature == "" {
		slog.Warn("stripe webhook missing signature header")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing Stripe-Signature header",
		})
	}

	body := c.Body()
	event, err := webhook.ConstructEventWithOptions(body, signature, h.stripeConfig.WebhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		slog.Warn("stripe webhook signature verification failed", "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid signature",
		})
	}

	slog.Info("stripe webhook received", "type", event.Type, "id", event.ID)

	// Route to event-specific handlers
	switch event.Type {
	case "crypto.onramp_session.updated":
		return h.handleOnrampSessionUpdated(c, event.Data.Raw)
	default:
		// Return 200 for unhandled events to prevent Stripe retries
		slog.Debug("unhandled stripe webhook event", "type", event.Type)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
		})
	}
}

// onrampSession represents the relevant fields from a Stripe Crypto Onramp session
type onrampSession struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Metadata struct {
		DepositID string `json:"deposit_id"`
	} `json:"metadata"`
}

// handleOnrampSessionUpdated processes crypto.onramp_session.updated events
func (h *StripeWebhookHandler) handleOnrampSessionUpdated(c fiber.Ctx, rawData json.RawMessage) error {
	var data struct {
		Object onrampSession `json:"object"`
	}
	if err := json.Unmarshal(rawData, &data); err != nil {
		slog.Error("failed to parse onramp session data", "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid event data",
		})
	}

	session := data.Object
	slog.Info("processing onramp session update",
		"session_id", session.ID,
		"status", session.Status,
		"deposit_id", session.Metadata.DepositID,
	)

	// Extract deposit ID from metadata
	if session.Metadata.DepositID == "" {
		slog.Warn("onramp session missing deposit_id in metadata", "session_id", session.ID)
		// Return 200 to prevent retries - this session wasn't created by us
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"warning":  "missing deposit_id in metadata",
		})
	}

	depositID, err := uuid.Parse(session.Metadata.DepositID)
	if err != nil {
		slog.Error("invalid deposit_id in metadata", "deposit_id", session.Metadata.DepositID, "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid deposit_id format",
		})
	}

	ctx := c.Context()

	// Handle based on session status
	switch session.Status {
	case "fulfillment_complete":
		// Get the deposit to check current status (idempotency)
		deposit, err := h.db.GetDepositByID(ctx, depositID)
		if err != nil {
			slog.Error("failed to get deposit", "deposit_id", depositID, "error", err)
			// Return 500 to trigger Stripe retry
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get deposit",
			})
		}

		// Skip if already completed (idempotent)
		if deposit.Status == db.DepositStatusCompleted {
			slog.Info("deposit already completed, skipping", "deposit_id", depositID)
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"received": true,
				"status":   "already_completed",
			})
		}

		// Complete the deposit and credit the account
		if err := h.db.CompleteDeposit(ctx, depositID); err != nil {
			slog.Error("failed to complete deposit", "deposit_id", depositID, "error", err)
			// Return 500 to trigger Stripe retry
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to complete deposit",
			})
		}

		slog.Info("deposit completed successfully", "deposit_id", depositID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"status":   "completed",
		})

	case "rejected":
		// Mark deposit as failed
		if err := h.db.FailDeposit(ctx, depositID, "Stripe onramp session rejected"); err != nil {
			slog.Error("failed to mark deposit as failed", "deposit_id", depositID, "error", err)
			// Return 500 to trigger Stripe retry
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update deposit status",
			})
		}

		slog.Info("deposit marked as failed", "deposit_id", depositID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"status":   "failed",
		})

	default:
		// Ignore intermediate states (requires_payment, fulfillment_processing, etc.)
		slog.Debug("ignoring intermediate onramp session status", "status", session.Status, "deposit_id", depositID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"status":   "ignored",
		})
	}
}

// ReadBody is a helper to read the request body for signature verification
func ReadBody(c fiber.Ctx) ([]byte, error) {
	return io.ReadAll(c.Request().BodyStream())
}
