// Package cli provides the API client for CLI operations
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// APIClient handles communication with the Stronghold API
type APIClient struct {
	baseURL     string
	httpClient  *http.Client
	deviceToken string
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL, deviceToken string) *APIClient {
	jar, _ := cookiejar.New(nil)
	return &APIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
		deviceToken: deviceToken,
	}
}

// SetDeviceToken updates the device token used for trusted device access.
func (c *APIClient) SetDeviceToken(token string) {
	c.deviceToken = token
}

// CreateAccountRequest represents a request to create an account
type CreateAccountRequest struct {
	WalletAddress *string `json:"wallet_address,omitempty"`
}

// CreateAccountResponse represents the response from creating an account
type CreateAccountResponse struct {
	AccountNumber string `json:"account_number"`
	ExpiresAt     string `json:"expires_at"`
	RecoveryFile  string `json:"recovery_file"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	AccountNumber string `json:"account_number"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	AccountNumber string  `json:"account_number"`
	ExpiresAt     string  `json:"expires_at"`
	WalletAddress *string `json:"wallet_address,omitempty"`
	TOTPRequired  bool    `json:"totp_required"`
	DeviceTrusted bool    `json:"device_trusted"`
	EscrowEnabled bool    `json:"wallet_escrow_enabled"`
}

// GetWalletKeyResponse represents the response from the wallet-key endpoint
type GetWalletKeyResponse struct {
	PrivateKey string `json:"private_key"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// APIError captures HTTP errors with status codes for callers that need branching.
type APIError struct {
	StatusCode int
	Method     string
	Endpoint   string
	Message    string
	Kind       string
}

func (e *APIError) Error() string {
	if e.Kind == "api" {
		return fmt.Sprintf("API error (%d %s %s): %s", e.StatusCode, e.Method, e.Endpoint, e.Message)
	}
	return fmt.Sprintf("unexpected status %d from %s %s: %s", e.StatusCode, e.Method, e.Endpoint, e.Message)
}

// doRequest performs an HTTP request with JSON marshaling/unmarshaling
func (c *APIClient) doRequest(method, endpoint string, expectedStatus int, reqBody interface{}, respBody interface{}) error {
	var body io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.deviceToken != "" {
		req.Header.Set("X-Stronghold-Device", c.deviceToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != expectedStatus {
		var errResp ErrorResponse
		if json.Unmarshal(respData, &errResp) == nil && errResp.Error != "" {
			return &APIError{
				StatusCode: resp.StatusCode,
				Method:     method,
				Endpoint:   endpoint,
				Message:    errResp.Error,
				Kind:       "api",
			}
		}
		// Include truncated response body for debugging
		bodyPreview := string(respData)
		if len(bodyPreview) > 200 {
			bodyPreview = bodyPreview[:200] + "..."
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Method:     method,
			Endpoint:   endpoint,
			Message:    bodyPreview,
			Kind:       "status",
		}
	}

	if respBody != nil {
		if err := json.Unmarshal(respData, respBody); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}
	return nil
}

// CreateAccount creates a new account via the API
func (c *APIClient) CreateAccount(req *CreateAccountRequest) (*CreateAccountResponse, error) {
	var result CreateAccountResponse
	if err := c.doRequest(http.MethodPost, "/v1/auth/account", http.StatusCreated, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Login authenticates with the API and returns account info including decrypted wallet key
func (c *APIClient) Login(accountNumber string) (*LoginResponse, error) {
	req := LoginRequest{AccountNumber: accountNumber}
	var result LoginResponse
	if err := c.doRequest(http.MethodPost, "/v1/auth/login", http.StatusOK, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetWalletKey retrieves the decrypted wallet private key from the server
func (c *APIClient) GetWalletKey() (string, error) {
	var result GetWalletKeyResponse
	if err := c.doRequest(http.MethodGet, "/v1/auth/wallet-key", http.StatusOK, nil, &result); err != nil {
		return "", err
	}
	return result.PrivateKey, nil
}

// TOTPSetupResponse represents the response from TOTP setup.
type TOTPSetupResponse struct {
	Secret        string   `json:"secret"`
	OTPAuthURL    string   `json:"otpauth_url"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// TOTPVerifyRequest represents a TOTP verification request.
type TOTPVerifyRequest struct {
	Code          string `json:"code,omitempty"`
	RecoveryCode  string `json:"recovery_code,omitempty"`
	DeviceLabel   string `json:"device_label,omitempty"`
	DeviceTTLDays int    `json:"device_ttl_days,omitempty"`
}

// TOTPVerifyResponse represents the response from TOTP verification.
type TOTPVerifyResponse struct {
	DeviceToken      string  `json:"device_token"`
	ExpiresAt        *string `json:"expires_at,omitempty"`
	RecoveryCodeUsed bool    `json:"recovery_code_used"`
}

// SetupTOTP enrolls TOTP for the account.
func (c *APIClient) SetupTOTP() (*TOTPSetupResponse, error) {
	var result TOTPSetupResponse
	if err := c.doRequest(http.MethodPost, "/v1/auth/totp/setup", http.StatusOK, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// VerifyTOTP verifies a TOTP or recovery code and trusts the device.
func (c *APIClient) VerifyTOTP(req *TOTPVerifyRequest) (*TOTPVerifyResponse, error) {
	var result TOTPVerifyResponse
	if err := c.doRequest(http.MethodPost, "/v1/auth/totp/verify", http.StatusOK, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateWalletRequest represents a request to update wallet
type UpdateWalletRequest struct {
	PrivateKey string `json:"private_key"`
}

// UpdateWallet updates the wallet for the current account
func (c *APIClient) UpdateWallet(privateKey string) error {
	req := UpdateWalletRequest{PrivateKey: privateKey}
	return c.doRequest(http.MethodPut, "/v1/auth/wallet", http.StatusOK, req, nil)
}
