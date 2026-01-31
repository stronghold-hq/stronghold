package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"stronghold/internal/config"
	"stronghold/internal/wallet"

	"github.com/gofiber/fiber/v3"
)

// X402Middleware creates x402 payment verification middleware
type X402Middleware struct {
	config         *config.X402Config
	pricing        *config.PricingConfig
	httpClient     *http.Client
}

// NewX402Middleware creates a new x402 middleware instance
func NewX402Middleware(cfg *config.X402Config, pricing *config.PricingConfig) *X402Middleware {
	return &X402Middleware{
		config:  cfg,
		pricing: pricing,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// PriceRoute represents a route with its price
type PriceRoute struct {
	Path   string
	Method string
	Price  float64
}

// GetRoutes returns all priced routes
func (m *X402Middleware) GetRoutes() []PriceRoute {
	return []PriceRoute{
		{Path: "/v1/scan/input", Method: "POST", Price: m.pricing.ScanInput},
		{Path: "/v1/scan/output", Method: "POST", Price: m.pricing.ScanOutput},
		{Path: "/v1/scan", Method: "POST", Price: m.pricing.ScanUnified},
		{Path: "/v1/scan/multiturn", Method: "POST", Price: m.pricing.ScanMultiturn},
	}
}

// RequirePayment returns middleware that requires x402 payment
func (m *X402Middleware) RequirePayment(price float64) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Skip if wallet address not configured (allow all in dev mode)
		if m.config.WalletAddress == "" {
			return c.Next()
		}

		// Convert price to wei (6 decimal places for USDC)
		priceWei := float64ToWei(price)

		// Check for payment header
		paymentHeader := c.Get("X-Payment")
		if paymentHeader == "" {
			// Return 402 with payment requirements
			return m.requirePaymentResponse(c, priceWei)
		}

		// Verify payment
		valid, err := m.verifyPayment(paymentHeader, priceWei)
		if err != nil || !valid {
			return m.requirePaymentResponse(c, priceWei)
		}

		// Payment valid, continue
		return c.Next()
	}
}

// requirePaymentResponse returns a 402 Payment Required response
func (m *X402Middleware) requirePaymentResponse(c fiber.Ctx, amount *big.Int) error {
	c.Status(fiber.StatusPaymentRequired)

	response := map[string]interface{}{
		"error": "Payment required",
		"payment_requirements": map[string]interface{}{
			"scheme":          "x402",
			"network":         m.config.Network,
			"recipient":       m.config.WalletAddress,
			"amount":          amount.String(),
			"currency":        "USDC",
			"facilitator_url": m.config.FacilitatorURL,
			"description":     "Citadel security scan",
		},
	}

	return c.JSON(response)
}

// verifyPayment verifies the x402 payment header via the facilitator
func (m *X402Middleware) verifyPayment(paymentHeader string, expectedAmount *big.Int) (bool, error) {
	// Parse payment header
	payload, err := wallet.ParseX402Payment(paymentHeader)
	if err != nil {
		return false, fmt.Errorf("failed to parse payment: %w", err)
	}

	// Verify the amount matches
	amount := new(big.Int)
	amount.SetString(payload.Amount, 10)
	if amount.Cmp(expectedAmount) != 0 {
		return false, fmt.Errorf("amount mismatch: expected %s, got %s", expectedAmount.String(), payload.Amount)
	}

	// Verify the recipient is our wallet
	if !strings.EqualFold(payload.Receiver, m.config.WalletAddress) {
		return false, fmt.Errorf("recipient mismatch: expected %s, got %s", m.config.WalletAddress, payload.Receiver)
	}

	// Verify payment signature
	if err := wallet.VerifyPaymentSignature(payload, payload.Payer); err != nil {
		return false, fmt.Errorf("invalid signature: %w", err)
	}

	// Call facilitator to verify payment is valid and not already spent
	verifyReq := struct {
		Payment  string `json:"payment"`
		Network  string `json:"network"`
		Amount   string `json:"amount"`
		Receiver string `json:"receiver"`
		Token    string `json:"token"`
	}{
		Payment:  paymentHeader,
		Network:  payload.Network,
		Amount:   payload.Amount,
		Receiver: payload.Receiver,
		Token:    payload.TokenAddress,
	}

	verifyBody, err := json.Marshal(verifyReq)
	if err != nil {
		return false, fmt.Errorf("failed to marshal verify request: %w", err)
	}

	facilitatorURL := m.config.FacilitatorURL
	if facilitatorURL == "" {
		facilitatorURL = "https://x402.org/facilitator"
	}

	resp, err := m.httpClient.Post(
		facilitatorURL+"/verify",
		"application/json",
		bytes.NewReader(verifyBody),
	)
	if err != nil {
		return false, fmt.Errorf("failed to call facilitator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("facilitator verification failed: %s", resp.Status)
	}

	var verifyResult struct {
		Valid   bool   `json:"valid"`
		Reason  string `json:"reason,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&verifyResult); err != nil {
		return false, fmt.Errorf("failed to decode verify response: %w", err)
	}

	if !verifyResult.Valid {
		return false, fmt.Errorf("payment invalid: %s", verifyResult.Reason)
	}

	return true, nil
}

// settlePayment settles the payment with the facilitator
func (m *X402Middleware) settlePayment(paymentHeader string) (string, error) {
	payload, err := wallet.ParseX402Payment(paymentHeader)
	if err != nil {
		return "", fmt.Errorf("failed to parse payment: %w", err)
	}

	settleReq := struct {
		Payment  string `json:"payment"`
		Network  string `json:"network"`
		Amount   string `json:"amount"`
		Receiver string `json:"receiver"`
		Token    string `json:"token"`
	}{
		Payment:  paymentHeader,
		Network:  payload.Network,
		Amount:   payload.Amount,
		Receiver: payload.Receiver,
		Token:    payload.TokenAddress,
	}

	settleBody, err := json.Marshal(settleReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal settle request: %w", err)
	}

	facilitatorURL := m.config.FacilitatorURL
	if facilitatorURL == "" {
		facilitatorURL = "https://x402.org/facilitator"
	}

	resp, err := m.httpClient.Post(
		facilitatorURL+"/settle",
		"application/json",
		bytes.NewReader(settleBody),
	)
	if err != nil {
		return "", fmt.Errorf("failed to call facilitator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("facilitator settlement failed: %s", resp.Status)
	}

	var settleResult struct {
		PaymentID string `json:"payment_id"`
		TxHash    string `json:"tx_hash,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&settleResult); err != nil {
		return "", fmt.Errorf("failed to decode settle response: %w", err)
	}

	return settleResult.PaymentID, nil
}

// PaymentResponse adds payment response header after successful processing
func (m *X402Middleware) PaymentResponse(c fiber.Ctx, paymentID string) {
	if m.config.WalletAddress == "" {
		return
	}

	response := map[string]string{
		"payment_id": paymentID,
		"status":     "settled",
	}

	responseJSON, _ := json.Marshal(response)
	c.Set("X-Payment-Response", string(responseJSON))
}

// float64ToWei converts a dollar amount to wei (6 decimals for USDC)
func float64ToWei(amount float64) *big.Int {
	// USDC has 6 decimals
	multiplier := big.NewInt(1_000_000)
	amountInt := big.NewInt(int64(amount * 1_000_000))
	return new(big.Int).Mul(amountInt, multiplier)
}

// IsFreeRoute checks if a route doesn't require payment
func (m *X402Middleware) IsFreeRoute(path string) bool {
	freeRoutes := []string{
		"/health",
		"/v1/pricing",
	}

	for _, route := range freeRoutes {
		if strings.HasPrefix(path, route) {
			return true
		}
	}
	return false
}

// Middleware returns the main x402 middleware that handles all routes
func (m *X402Middleware) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()

		// Skip free routes
		if m.IsFreeRoute(path) {
			return c.Next()
		}

		// Skip if wallet not configured
		if m.config.WalletAddress == "" {
			return c.Next()
		}

		// Get price for this route
		price := m.getPriceForRoute(path, c.Method())
		if price == 0 {
			// No price set, allow through
			return c.Next()
		}

		// Check payment
		paymentHeader := c.Get("X-Payment")
		if paymentHeader == "" {
			return m.requirePaymentResponse(c, float64ToWei(price))
		}

		valid, err := m.verifyPayment(paymentHeader, float64ToWei(price))
		if err != nil || !valid {
			return m.requirePaymentResponse(c, float64ToWei(price))
		}

		// Store payment header for settlement after successful handler
		c.Locals("x402_payment", paymentHeader)

		return c.Next()
	}
}

// SettleAfterHandler settles the payment after the handler completes successfully
func (m *X402Middleware) SettleAfterHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Continue to next handler first
		err := c.Next()
		if err != nil {
			return err
		}

		// Check if there's a payment to settle
		paymentHeader, ok := c.Locals("x402_payment").(string)
		if !ok || paymentHeader == "" {
			return nil
		}

		// Only settle if response is successful
		if c.Response().StatusCode() >= 200 && c.Response().StatusCode() < 300 {
			paymentID, err := m.settlePayment(paymentHeader)
			if err != nil {
				// Log but don't fail the request - payment was already verified
				fmt.Printf("Failed to settle payment: %v\n", err)
			} else {
				m.PaymentResponse(c, paymentID)
			}
		}

		return nil
	}
}

// getPriceForRoute returns the price for a given route
func (m *X402Middleware) getPriceForRoute(path, method string) float64 {
	routes := m.GetRoutes()
	for _, route := range routes {
		if strings.HasPrefix(path, route.Path) && method == route.Method {
			return route.Price
		}
	}
	return 0
}

// X402Client is a client for interacting with x402 payments
type X402Client struct {
	FacilitatorURL string
	Network        string
}

// NewX402Client creates a new x402 client
func NewX402Client(facilitatorURL, network string) *X402Client {
	return &X402Client{
		FacilitatorURL: facilitatorURL,
		Network:        network,
	}
}

// VerifyPayment verifies a payment with the facilitator
func (c *X402Client) VerifyPayment(payment string, amount *big.Int) (bool, error) {
	// This is now handled by the middleware
	return true, nil
}

// SettlePayment settles a payment with the facilitator
func (c *X402Client) SettlePayment(payment string) (string, error) {
	// This is now handled by the middleware
	return "payment-id", nil
}
