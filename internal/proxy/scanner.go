package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"stronghold/internal/wallet"
)

// Decision represents the scan decision
type Decision string

const (
	DecisionAllow Decision = "ALLOW"
	DecisionWarn  Decision = "WARN"
	DecisionBlock Decision = "BLOCK"
)

// Threat represents a detected threat
type Threat struct {
	Category    string `json:"category"`
	Pattern     string `json:"pattern"`
	Location    string `json:"location"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

// ScanResult represents the result of a security scan
type ScanResult struct {
	Decision          Decision               `json:"decision"`
	Scores            map[string]float64     `json:"scores"`
	Reason            string                 `json:"reason"`
	LatencyMs         int64                  `json:"latency_ms"`
	RequestID         string                 `json:"request_id"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	SanitizedText     string                 `json:"sanitized_text,omitempty"`
	ThreatsFound      []Threat               `json:"threats_found,omitempty"`
	RecommendedAction string                 `json:"recommended_action,omitempty"`
}

// ScanRequest represents a scan request
type ScanRequest struct {
	Text        string `json:"text"`
	SourceURL   string `json:"source_url,omitempty"`
	SourceType  string `json:"source_type,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}

// X402Wallet defines the interface for x402 payment creation
type X402Wallet interface {
	Exists() bool
	CreateX402Payment(req *wallet.PaymentRequirements) (string, error)
}

// ScannerClient is a client for the Stronghold scanning API
type ScannerClient struct {
	baseURL        string
	token          string
	httpClient     *http.Client
	wallet         X402Wallet // EVM wallet (Base)
	solanaWallet   X402Wallet // Solana wallet
	facilitatorURL string
}

// NewScannerClient creates a new scanner client
func NewScannerClient(baseURL, token string) *ScannerClient {
	// Use standard client (no socket marks needed - we use user-based filtering)
	client := &http.Client{
		Timeout: 5 * time.Second,
		// Don't follow redirects to prevent payment headers from being sent
		// to attacker-controlled URLs via redirect chains
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &ScannerClient{
		baseURL:        baseURL,
		token:          token,
		httpClient:     client,
		facilitatorURL: "https://x402.org/facilitator",
	}
}

// SetWallet sets the EVM wallet for x402 payments
func (c *ScannerClient) SetWallet(w X402Wallet) {
	c.wallet = w
}

// SetSolanaWallet sets the Solana wallet for x402 payments
func (c *ScannerClient) SetSolanaWallet(w X402Wallet) {
	c.solanaWallet = w
}

// ScanContent scans external content for prompt injection attacks
func (c *ScannerClient) ScanContent(ctx context.Context, content []byte, sourceURL, contentType string) (*ScanResult, error) {
	req := ScanRequest{
		Text:        string(content),
		SourceURL:   sourceURL,
		SourceType:  "http_proxy",
		ContentType: contentType,
	}

	return c.scanWithPayment(ctx, "/v1/scan/content", req)
}

// scanWithPayment performs a scan request with automatic x402 payment handling
func (c *ScannerClient) scanWithPayment(ctx context.Context, endpoint string, reqBody interface{}) (*ScanResult, error) {
	// Try the request first (might already have credit or in dev mode)
	result, statusCode, paymentReq, err := c.scan(ctx, endpoint, reqBody, "")

	// If successful or error other than 402, return immediately
	if err != nil || statusCode != http.StatusPaymentRequired {
		return result, err
	}

	// Handle 402 Payment Required
	if paymentReq == nil {
		return nil, fmt.Errorf("payment required but no requirements received")
	}

	// Select wallet based on network
	selectedWallet := c.wallet
	if wallet.IsSolanaNetwork(paymentReq.Network) {
		selectedWallet = c.solanaWallet
	}

	if selectedWallet == nil {
		return nil, fmt.Errorf("payment required but no wallet configured for network %s. Run 'stronghold wallet list' or 'stronghold wallet balance' to check wallet status, or visit https://getstronghold.xyz/dashboard to add funds", paymentReq.Network)
	}

	// Create x402 payment
	paymentHeader, err := selectedWallet.CreateX402Payment(paymentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	// Retry with payment
	result, statusCode, _, err = c.scan(ctx, endpoint, reqBody, paymentHeader)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusPaymentRequired {
		return nil, fmt.Errorf("payment was rejected - insufficient funds or invalid payment. Check your balance with 'stronghold wallet balance'")
	}

	return result, nil
}

// scan performs the actual scan request
// Returns: result, statusCode, paymentRequirements (if 402), error
func (c *ScannerClient) scan(ctx context.Context, endpoint string, reqBody interface{}, paymentHeader string) (*ScanResult, int, *wallet.PaymentRequirements, error) {
	url := c.baseURL + endpoint

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if paymentHeader != "" {
		req.Header.Set("X-Payment", paymentHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Handle 402 Payment Required
	if resp.StatusCode == http.StatusPaymentRequired {
		paymentReq, err := c.parsePaymentRequired(resp)
		if err != nil {
			return nil, resp.StatusCode, nil, fmt.Errorf("payment required but failed to parse requirements: %w", err)
		}
		return nil, resp.StatusCode, paymentReq, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, resp.StatusCode, nil, fmt.Errorf("scan failed: %s - %s", resp.Status, string(body))
	}

	var result ScanResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, resp.StatusCode, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, resp.StatusCode, nil, nil
}

// paymentOption represents a single payment option from the 402 response
type paymentOption struct {
	Scheme         string `json:"scheme"`
	Network        string `json:"network"`
	Recipient      string `json:"recipient"`
	Amount         string `json:"amount"`
	Currency       string `json:"currency"`
	FacilitatorURL string `json:"facilitator_url"`
	Description    string `json:"description"`
	FeePayer       string `json:"fee_payer,omitempty"`
}

// parsePaymentRequired parses a 402 response to extract payment requirements.
// It inspects the "accepts" array and selects the first option the client has
// a wallet for. Falls back to "payment_requirements" for older servers.
func (c *ScannerClient) parsePaymentRequired(resp *http.Response) (*wallet.PaymentRequirements, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response struct {
		Error               string          `json:"error"`
		PaymentRequirements paymentOption   `json:"payment_requirements"`
		Accepts             []paymentOption `json:"accepts"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse 402 response: %w", err)
	}

	// If accepts array present, pick the first option we can pay for
	if len(response.Accepts) > 0 {
		for _, opt := range response.Accepts {
			if c.hasWalletForNetwork(opt.Network) {
				return optionToRequirements(&opt), nil
			}
		}
		// No matching wallet found; return the first option so the caller
		// can produce a helpful "no wallet for network X" error
		return optionToRequirements(&response.Accepts[0]), nil
	}

	// Backward compat: older servers only send payment_requirements
	return optionToRequirements(&response.PaymentRequirements), nil
}

// hasWalletForNetwork returns true if the client has a wallet for the given network
func (c *ScannerClient) hasWalletForNetwork(network string) bool {
	if !wallet.IsNetworkSupported(network) {
		return false
	}
	if wallet.IsSolanaNetwork(network) {
		return c.solanaWallet != nil && c.solanaWallet.Exists()
	}
	return c.wallet != nil && c.wallet.Exists()
}

// optionToRequirements converts a paymentOption to wallet.PaymentRequirements
func optionToRequirements(opt *paymentOption) *wallet.PaymentRequirements {
	return &wallet.PaymentRequirements{
		Scheme:         opt.Scheme,
		Network:        opt.Network,
		Recipient:      opt.Recipient,
		Amount:         opt.Amount,
		Currency:       opt.Currency,
		FacilitatorURL: opt.FacilitatorURL,
		Description:    opt.Description,
		FeePayer:       opt.FeePayer,
	}
}

// ShouldScanContentType determines if a content type should be scanned
func ShouldScanContentType(contentType string) bool {
	// Scan these content types
	scannableTypes := []string{
		"text/html",
		"text/plain",
		"text/markdown",
		"application/json",
		"application/xml",
		"text/xml",
		"application/javascript",
		"text/javascript",
		"text/css",
	}

	for _, t := range scannableTypes {
		if contains(contentType, t) {
			return true
		}
	}

	return false
}

// IsBinaryContentType determines if a content type is binary
func IsBinaryContentType(contentType string) bool {
	binaryTypes := []string{
		"image/",
		"video/",
		"audio/",
		"application/octet-stream",
		"application/pdf",
		"application/zip",
		"application/gzip",
		"application/x-",
	}

	for _, t := range binaryTypes {
		if contains(contentType, t) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
