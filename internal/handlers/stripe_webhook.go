package handlers

import (
	"log/slog"
	"time"

	"stronghold/internal/config"
	"stronghold/internal/db"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82/webhook"
)

// webhookTimestampTolerance is the maximum age of a webhook before it's rejected
// to prevent replay attacks
const webhookTimestampTolerance = 5 * time.Minute

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

	// Validate webhook timestamp to prevent replay attacks
	eventTime := time.Unix(event.Created, 0)
	if time.Since(eventTime) > webhookTimestampTolerance {
		slog.Warn("stripe webhook rejected: timestamp too old",
			"event_id", event.ID,
			"event_time", eventTime,
			"age", time.Since(eventTime),
		)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Webhook timestamp too old",
		})
	}

	slog.Info("stripe webhook received", "type", event.Type, "id", event.ID)

	// Check event ID idempotency - reject duplicates
	alreadyProcessed, err := h.db.CheckAndRecordWebhookEvent(c.Context(), event.ID, string(event.Type))
	if err != nil {
		slog.Error("failed to check webhook event idempotency", "event_id", event.ID, "error", err)
		// Return 500 to trigger Stripe retry
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal error",
		})
	}
	if alreadyProcessed {
		slog.Info("duplicate stripe webhook event, skipping", "event_id", event.ID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received":  true,
			"duplicate": true,
		})
	}

	// Route to event-specific handlers
	switch event.Type {
	case "crypto.onramp_session.updated":
		return h.handleOnrampSessionUpdated(c, event.Data.Object)
	case "checkout.session.completed":
		return h.handleCheckoutSessionCompleted(c, event.Data.Object)
	case "checkout.session.expired":
		return h.handleCheckoutSessionExpired(c, event.Data.Object)
	case "invoice.paid":
		return h.handleInvoicePaid(c, event.Data.Object)
	case "invoice.payment_failed":
		return h.handleInvoicePaymentFailed(c, event.Data.Object)
	default:
		// Return 200 for unhandled events to prevent Stripe retries
		slog.Debug("unhandled stripe webhook event", "type", event.Type)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
		})
	}
}

// handleOnrampSessionUpdated processes crypto.onramp_session.updated events
func (h *StripeWebhookHandler) handleOnrampSessionUpdated(c fiber.Ctx, obj map[string]interface{}) error {
	// Extract fields from the parsed object map
	sessionID, _ := obj["id"].(string)
	status, _ := obj["status"].(string)

	var depositID string
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		depositID, _ = metadata["deposit_id"].(string)
	}

	slog.Info("processing onramp session update",
		"session_id", sessionID,
		"status", status,
		"deposit_id", depositID,
	)

	// Extract deposit ID from metadata
	if depositID == "" {
		slog.Warn("onramp session missing deposit_id in metadata", "session_id", sessionID)
		// Return 200 to prevent retries - this session wasn't created by us
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"warning":  "missing deposit_id in metadata",
		})
	}

	parsedDepositID, err := uuid.Parse(depositID)
	if err != nil {
		slog.Error("invalid deposit_id in metadata", "deposit_id", depositID, "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid deposit_id format",
		})
	}

	ctx := c.Context()

	// Handle based on session status
	switch status {
	case "fulfillment_complete":
		// Get the deposit to check current status (idempotency)
		deposit, err := h.db.GetDepositByID(ctx, parsedDepositID)
		if err != nil {
			slog.Error("failed to get deposit", "deposit_id", parsedDepositID, "error", err)
			// Return 500 to trigger Stripe retry
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get deposit",
			})
		}

		// Skip if already completed (idempotent)
		if deposit.Status == db.DepositStatusCompleted {
			slog.Info("deposit already completed, skipping", "deposit_id", parsedDepositID)
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"received": true,
				"status":   "already_completed",
			})
		}

		// Log the network from deposit metadata if available
		depositNetwork := "unknown"
		if deposit.Metadata != nil {
			if n, ok := deposit.Metadata["network"].(string); ok {
				depositNetwork = n
			}
		}

		// Complete the deposit and credit the account
		if err := h.db.CompleteDeposit(ctx, parsedDepositID); err != nil {
			slog.Error("failed to complete deposit", "deposit_id", parsedDepositID, "error", err)
			// Return 500 to trigger Stripe retry
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to complete deposit",
			})
		}

		slog.Info("deposit completed successfully",
			"deposit_id", parsedDepositID,
			"network", depositNetwork,
		)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"status":   "completed",
		})

	case "rejected":
		// Log the network from deposit metadata if available
		depositNetwork := "unknown"
		if rejDeposit, err := h.db.GetDepositByID(ctx, parsedDepositID); err == nil && rejDeposit.Metadata != nil {
			if n, ok := rejDeposit.Metadata["network"].(string); ok {
				depositNetwork = n
			}
		}

		// Mark deposit as failed
		if err := h.db.FailDeposit(ctx, parsedDepositID, "Stripe onramp session rejected"); err != nil {
			slog.Error("failed to mark deposit as failed", "deposit_id", parsedDepositID, "error", err)
			// Return 500 to trigger Stripe retry
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update deposit status",
			})
		}

		slog.Info("deposit marked as failed",
			"deposit_id", parsedDepositID,
			"network", depositNetwork,
		)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"status":   "failed",
		})

	default:
		// Ignore intermediate states (requires_payment, fulfillment_processing, etc.)
		slog.Debug("ignoring intermediate onramp session status", "status", status, "deposit_id", parsedDepositID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"status":   "ignored",
		})
	}
}

// handleCheckoutSessionCompleted processes checkout.session.completed events (B2B credit purchases)
func (h *StripeWebhookHandler) handleCheckoutSessionCompleted(c fiber.Ctx, obj map[string]interface{}) error {
	sessionID, _ := obj["id"].(string)
	paymentStatus, _ := obj["payment_status"].(string)

	var depositID string
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		depositID, _ = metadata["deposit_id"].(string)
	}

	slog.Info("processing checkout session completed",
		"session_id", sessionID,
		"payment_status", paymentStatus,
		"deposit_id", depositID,
	)

	if paymentStatus != "paid" {
		slog.Info("checkout session not paid, ignoring", "session_id", sessionID, "status", paymentStatus)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"status":   "ignored",
		})
	}

	if depositID == "" {
		slog.Warn("checkout session missing deposit_id in metadata", "session_id", sessionID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"warning":  "missing deposit_id in metadata",
		})
	}

	parsedDepositID, err := uuid.Parse(depositID)
	if err != nil {
		slog.Error("invalid deposit_id in checkout metadata", "deposit_id", depositID, "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid deposit_id format",
		})
	}

	ctx := c.Context()

	// Check deposit status (idempotency)
	deposit, err := h.db.GetDepositByID(ctx, parsedDepositID)
	if err != nil {
		slog.Error("failed to get deposit for checkout", "deposit_id", parsedDepositID, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get deposit",
		})
	}

	if deposit.Status == db.DepositStatusCompleted {
		slog.Info("deposit already completed, skipping", "deposit_id", parsedDepositID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"status":   "already_completed",
		})
	}

	// Complete the deposit (credits the account balance via DB trigger)
	if err := h.db.CompleteDeposit(ctx, parsedDepositID); err != nil {
		slog.Error("failed to complete checkout deposit", "deposit_id", parsedDepositID, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to complete deposit",
		})
	}

	slog.Info("B2B credit purchase completed",
		"deposit_id", parsedDepositID,
		"session_id", sessionID,
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"received": true,
		"status":   "completed",
	})
}

// handleCheckoutSessionExpired processes checkout.session.expired events.
// When a user abandons a Stripe Checkout session, the pending deposit must be failed
// to prevent orphan records in billing history.
func (h *StripeWebhookHandler) handleCheckoutSessionExpired(c fiber.Ctx, obj map[string]interface{}) error {
	sessionID, _ := obj["id"].(string)

	var depositID string
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		depositID, _ = metadata["deposit_id"].(string)
	}

	slog.Info("checkout session expired", "session_id", sessionID, "deposit_id", depositID)

	if depositID == "" {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"received": true,
			"warning":  "missing deposit_id in metadata",
		})
	}

	parsedDepositID, err := uuid.Parse(depositID)
	if err != nil {
		slog.Error("invalid deposit_id in expired checkout metadata", "deposit_id", depositID, "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid deposit_id format",
		})
	}

	if err := h.db.FailDeposit(c.Context(), parsedDepositID, "checkout session expired"); err != nil {
		slog.Error("failed to mark expired checkout deposit as failed",
			"deposit_id", parsedDepositID, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update deposit status",
		})
	}

	slog.Info("expired checkout deposit marked as failed", "deposit_id", parsedDepositID)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"received": true,
		"status":   "failed",
	})
}

// handleInvoicePaid processes invoice.paid events (metered billing invoices)
func (h *StripeWebhookHandler) handleInvoicePaid(c fiber.Ctx, obj map[string]interface{}) error {
	invoiceID, _ := obj["id"].(string)
	customerID, _ := obj["customer"].(string)

	slog.Info("metered invoice paid",
		"invoice_id", invoiceID,
		"customer_id", customerID,
	)

	// No balance change needed - usage was already served when metered
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"received": true,
		"status":   "logged",
	})
}

// handleInvoicePaymentFailed processes invoice.payment_failed events
func (h *StripeWebhookHandler) handleInvoicePaymentFailed(c fiber.Ctx, obj map[string]interface{}) error {
	invoiceID, _ := obj["id"].(string)
	customerID, _ := obj["customer"].(string)

	slog.Warn("metered invoice payment failed",
		"invoice_id", invoiceID,
		"customer_id", customerID,
	)

	// V1: log only. V2 could suspend B2B account after repeated failures.
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"received": true,
		"status":   "logged",
	})
}
