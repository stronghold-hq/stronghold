package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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

// ScannerClient is a client for the Stronghold scanning API
type ScannerClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewScannerClient creates a new scanner client
func NewScannerClient(baseURL, token string) *ScannerClient {
	return &ScannerClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ScanContent scans content for prompt injection attacks
func (c *ScannerClient) ScanContent(ctx context.Context, content []byte, sourceURL, contentType string) (*ScanResult, error) {
	req := ScanRequest{
		Text:        string(content),
		SourceURL:   sourceURL,
		SourceType:  "http_proxy",
		ContentType: contentType,
	}

	return c.scan(ctx, "/v1/scan/content", req)
}

// Scan scans content using the unified endpoint
func (c *ScannerClient) Scan(ctx context.Context, content []byte, mode string) (*ScanResult, error) {
	reqBody := map[string]interface{}{
		"text": content,
		"mode": mode,
	}

	return c.scan(ctx, "/v1/scan", reqBody)
}

// scan performs the actual scan request
func (c *ScannerClient) scan(ctx context.Context, endpoint string, reqBody interface{}) (*ScanResult, error) {
	url := c.baseURL + endpoint

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scan failed: %s - %s", resp.Status, string(body))
	}

	var result ScanResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
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
