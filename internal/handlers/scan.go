package handlers

import (
	"stronghold/internal/middleware"
	"stronghold/internal/stronghold"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// ScanHandler handles scan-related endpoints
type ScanHandler struct {
	scanner *stronghold.Scanner
	x402    *middleware.X402Middleware
}

// NewScanHandler creates a new scan handler
func NewScanHandler(scanner *stronghold.Scanner, x402 *middleware.X402Middleware) *ScanHandler {
	return &ScanHandler{
		scanner: scanner,
		x402:    x402,
	}
}

// ScanContentRequest represents a request to scan external content for prompt injection
type ScanContentRequest struct {
	Text        string `json:"text"`
	SourceURL   string `json:"source_url,omitempty"`   // Where content came from (e.g., https://github.com/...)
	SourceType  string `json:"source_type,omitempty"`  // "web_page", "file", "api_response", "code_repo"
	ContentType string `json:"content_type,omitempty"` // "html", "markdown", "json", "text", "code"
	FilePath    string `json:"file_path,omitempty"`    // For file reads, e.g., "README.md"
	SessionID   string `json:"session_id,omitempty"`   // For multi-turn context
}

// ScanOutputRequest represents a request to scan LLM/agent output for credential leaks
type ScanOutputRequest struct {
	Text string `json:"text"`
}

// ScanUnifiedRequest represents a unified scan request
type ScanUnifiedRequest struct {
	Text string `json:"text"`
	Mode string `json:"mode"` // "input", "output", or "both"
}

// ScanMultiturnRequest represents a multi-turn scan request
type ScanMultiturnRequest struct {
	SessionID string           `json:"session_id"`
	Turns     []stronghold.Turn `json:"turns"`
}

// RegisterRoutes registers all scan routes
func (h *ScanHandler) RegisterRoutes(app *fiber.App) {
	group := app.Group("/v1/scan")

	// Content scanning - for external content (websites, files, APIs) - $0.001
	group.Post("/content", h.x402.RequirePayment(0.001), h.ScanContent)

	// Output scanning - for LLM/agent output credential leak detection - $0.001
	group.Post("/output", h.x402.RequirePayment(0.001), h.ScanOutput)

	// Unified scanning - $0.002
	group.Post("/", h.x402.RequirePayment(0.002), h.ScanUnified)

	// Multi-turn scanning - $0.005
	group.Post("/multiturn", h.x402.RequirePayment(0.005), h.ScanMultiturn)

	// Deprecated: /input endpoint - redirects to /content for backward compatibility
	group.Post("/input", h.x402.RequirePayment(0.001), h.ScanContent)
}

// ScanContent handles external content scanning for prompt injection
// @Summary Scan external content for prompt injection
// @Description Scans content from external sources (websites, files, APIs) for prompt injection attacks before passing to LLM
// @Tags scan
// @Accept json
// @Produce json
// @Param request body ScanContentRequest true "Content scan request"
// @Success 200 {object} stronghold.ScanResult
// @Failure 400 {object} map[string]string
// @Failure 402 {object} map[string]interface{}
// @Router /v1/scan/content [post]
func (h *ScanHandler) ScanContent(c fiber.Ctx) error {
	var req ScanContentRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Text == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Text is required",
		})
	}

	result, err := h.scanner.ScanContent(c.Context(), req.Text, req.SourceURL, req.SourceType, req.ContentType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Scan failed: " + err.Error(),
		})
	}

	// Add source metadata to result
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["source_url"] = req.SourceURL
	result.Metadata["source_type"] = req.SourceType
	result.Metadata["content_type"] = req.ContentType
	result.Metadata["file_path"] = req.FilePath

	result.RequestID = uuid.New().String()
	h.x402.PaymentResponse(c, result.RequestID)

	return c.JSON(result)
}

// ScanOutput handles output scanning
// @Summary Scan LLM output for credential leaks
// @Description Scans LLM output text for credential leaks and sensitive data exposure
// @Tags scan
// @Accept json
// @Produce json
// @Param request body ScanOutputRequest true "Output scan request"
// @Success 200 {object} stronghold.ScanResult
// @Failure 400 {object} map[string]string
// @Failure 402 {object} map[string]interface{}
// @Router /v1/scan/output [post]
func (h *ScanHandler) ScanOutput(c fiber.Ctx) error {
	var req ScanOutputRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Text == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Text is required",
		})
	}

	result, err := h.scanner.ScanOutput(c.Context(), req.Text)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Scan failed: " + err.Error(),
		})
	}

	result.RequestID = uuid.New().String()
	h.x402.PaymentResponse(c, result.RequestID)

	return c.JSON(result)
}

// ScanUnified handles unified scanning
// @Summary Unified content scanning
// @Description Scans content for both input and output threats based on mode
// @Tags scan
// @Accept json
// @Produce json
// @Param request body ScanUnifiedRequest true "Unified scan request"
// @Success 200 {object} stronghold.ScanResult
// @Failure 400 {object} map[string]string
// @Failure 402 {object} map[string]interface{}
// @Router /v1/scan [post]
func (h *ScanHandler) ScanUnified(c fiber.Ctx) error {
	var req ScanUnifiedRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Text == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Text is required",
		})
	}

	mode := req.Mode
	if mode == "" {
		mode = "both"
	}

	if mode != "input" && mode != "output" && mode != "both" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid mode. Must be 'input', 'output', or 'both'",
		})
	}

	result, err := h.scanner.ScanUnified(c.Context(), req.Text, mode)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Scan failed: " + err.Error(),
		})
	}

	result.RequestID = uuid.New().String()
	h.x402.PaymentResponse(c, result.RequestID)

	return c.JSON(result)
}

// ScanMultiturn handles multi-turn conversation scanning
// @Summary Scan multi-turn conversations
// @Description Scans conversation history for context-aware attacks
// @Tags scan
// @Accept json
// @Produce json
// @Param request body ScanMultiturnRequest true "Multi-turn scan request"
// @Success 200 {object} stronghold.ScanResult
// @Failure 400 {object} map[string]string
// @Failure 402 {object} map[string]interface{}
// @Router /v1/scan/multiturn [post]
func (h *ScanHandler) ScanMultiturn(c fiber.Ctx) error {
	var req ScanMultiturnRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.SessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Session ID is required",
		})
	}

	if len(req.Turns) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least one turn is required",
		})
	}

	result, err := h.scanner.ScanMultiturn(c.Context(), req.SessionID, req.Turns)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Scan failed: " + err.Error(),
		})
	}

	result.RequestID = uuid.New().String()
	h.x402.PaymentResponse(c, result.RequestID)

	return c.JSON(result)
}
