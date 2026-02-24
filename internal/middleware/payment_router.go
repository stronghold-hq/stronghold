package middleware

import (
	"log/slog"

	"stronghold/internal/billing"
	"stronghold/internal/db"
	"stronghold/internal/usdc"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// PaymentRouter routes payment handling between x402 (B2C) and API key (B2B) authentication.
// For scan endpoints, it checks:
// 1. X-PAYMENT header → delegate to x402 AtomicPayment
// 2. Authorization: Bearer sk_live_... → API key auth with credit deduction or metered billing
// 3. Neither → return 402 Payment Required
type PaymentRouter struct {
	x402   *X402Middleware
	apiKey *APIKeyMiddleware
	meter  *billing.MeterReporter
	db     *db.DB
}

// NewPaymentRouter creates a new payment router
func NewPaymentRouter(x402 *X402Middleware, apiKey *APIKeyMiddleware, meter *billing.MeterReporter, database *db.DB) *PaymentRouter {
	return &PaymentRouter{
		x402:   x402,
		apiKey: apiKey,
		meter:  meter,
		db:     database,
	}
}

// Route returns middleware that handles payment for the given price.
// It accepts either x402 crypto payment OR B2B API key authentication.
func (pr *PaymentRouter) Route(price usdc.MicroUSDC) fiber.Handler {
	// Pre-build the x402 handler for this price
	x402Handler := pr.x402.AtomicPayment(price)

	return func(c fiber.Ctx) error {
		// Path 1: x402 crypto payment (X-PAYMENT header present)
		if c.Get("X-Payment") != "" {
			return x402Handler(c)
		}

		// Path 2: API key authentication (Bearer sk_live_...)
		authHeader := string(c.Request().Header.Peek("Authorization"))
		if authHeader != "" && len(authHeader) > 7 {
			// Check if it looks like an API key (not a JWT)
			token := authHeader
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				token = authHeader[7:]
			}
			if len(token) > 8 && token[:8] == "sk_live_" {
				return pr.handleAPIKeyPayment(c, price)
			}
		}

		// Path 3: Neither header present → 402 Payment Required
		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"error":   "Payment required",
			"message": "Include an X-PAYMENT header (x402 crypto) or Authorization: Bearer sk_live_... (API key)",
		})
	}
}

// handleAPIKeyPayment authenticates via API key and handles billing (credits or metered)
func (pr *PaymentRouter) handleAPIKeyPayment(c fiber.Ctx, price usdc.MicroUSDC) error {
	// Authenticate API key
	account, _, err := pr.apiKey.Authenticate(c)
	if err != nil {
		return err
	}

	// Try deducting from credit balance (atomic SQL)
	deducted, err := pr.db.DeductBalance(c.Context(), account.ID, price)
	if err != nil {
		slog.Error("failed to deduct balance", "account_id", account.ID, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Payment processing error",
		})
	}

	if deducted {
		// Credits used - log usage and proceed
		pr.logUsage(c, account.ID, price, "credits")
		return c.Next()
	}

	// Insufficient credits - fall back to metered billing
	if account.StripeCustomerID == nil || *account.StripeCustomerID == "" {
		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"error":   "Insufficient credits",
			"message": "Your credit balance is insufficient. Purchase credits at /v1/billing/credits.",
		})
	}

	// Report to Stripe Meter
	if pr.meter != nil {
		if err := pr.meter.ReportUsage(c.Context(), account.ID, *account.StripeCustomerID, c.Path(), price); err != nil {
			slog.Error("failed to report metered usage", "account_id", account.ID, "error", err)
			// Still allow the request - usage is logged locally, Stripe can be reconciled
		}
	}

	pr.logUsage(c, account.ID, price, "metered")
	return c.Next()
}

// logUsage creates a usage log entry for a B2B API request
func (pr *PaymentRouter) logUsage(c fiber.Ctx, accountID uuid.UUID, price usdc.MicroUSDC, paymentMethod string) {
	requestID := GetRequestID(c)
	usageLog := &db.UsageLog{
		AccountID: accountID,
		RequestID: requestID,
		Endpoint:  c.Path(),
		Method:    c.Method(),
		CostUSDC:  price,
		Status:    "success",
		Metadata: map[string]any{
			"payment_method": paymentMethod,
			"account_type":   "b2b",
		},
	}

	if err := pr.db.CreateUsageLog(c.Context(), usageLog); err != nil {
		slog.Error("failed to create usage log", "account_id", accountID, "error", err)
	}
}
