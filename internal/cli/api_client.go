// Package cli provides the API client for CLI operations
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIClient handles communication with the Stronghold API
type APIClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
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
	PrivateKey    *string `json:"private_key,omitempty"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// CreateAccount creates a new account via the API
func (c *APIClient) CreateAccount(req *CreateAccountRequest) (*CreateAccountResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/v1/auth/account",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result CreateAccountResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// Login authenticates with the API and returns account info including decrypted wallet key
func (c *APIClient) Login(accountNumber string) (*LoginResponse, error) {
	req := LoginRequest{AccountNumber: accountNumber}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/v1/auth/login",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to login: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result LoginResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}
