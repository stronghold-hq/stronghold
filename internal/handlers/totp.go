package handlers

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	deviceTokenCookie = "stronghold_device"
	deviceTokenHeader = "X-Stronghold-Device"
)

// TOTPSetupResponse contains enrollment details.
type TOTPSetupResponse struct {
	Secret        string   `json:"secret"`
	OTPAuthURL    string   `json:"otpauth_url"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// TOTPVerifyRequest verifies a TOTP or recovery code and trusts the device.
type TOTPVerifyRequest struct {
	Code          string `json:"code,omitempty"`
	RecoveryCode  string `json:"recovery_code,omitempty"`
	DeviceLabel   string `json:"device_label,omitempty"`
	DeviceTTLDays int    `json:"device_ttl_days,omitempty"`
}

// TOTPVerifyResponse returns the trusted device token.
type TOTPVerifyResponse struct {
	DeviceToken      string     `json:"device_token"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	RecoveryCodeUsed bool       `json:"recovery_code_used"`
}

// DeviceRevokeRequest revokes device trust.
type DeviceRevokeRequest struct {
	DeviceID string `json:"device_id,omitempty"`
	All      bool   `json:"all,omitempty"`
}

// SetupTOTP enrolls TOTP for an account and returns the secret and recovery codes.
func (h *AuthHandler) SetupTOTP(c fiber.Ctx) error {
	accountID, err := h.requireAccountID(c)
	if err != nil {
		return err
	}
	if h.kmsClient == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "KMS not configured",
		})
	}

	ctx := c.Context()
	account, err := h.db.GetAccountByID(ctx, accountID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Account not found",
		})
	}
	if account.TOTPEnabled {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "TOTP already enabled",
		})
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Stronghold",
		AccountName: account.AccountNumber,
		Digits:      otp.DigitsSix,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate TOTP secret",
		})
	}

	encrypted, err := h.kmsClient.Encrypt(ctx, key.Secret())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to secure TOTP secret",
		})
	}

	if err := h.db.SetTOTPSecret(ctx, accountID, encrypted); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to store TOTP secret",
		})
	}
	// Replace any existing recovery codes on setup.
	_ = h.db.ClearRecoveryCodes(ctx, accountID)

	recoveryCodes, err := generateRecoveryCodes(10)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate recovery codes",
		})
	}
	if err := h.db.StoreRecoveryCodes(ctx, accountID, normalizeRecoveryCodes(recoveryCodes)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to store recovery codes",
		})
	}

	return c.JSON(TOTPSetupResponse{
		Secret:        key.Secret(),
		OTPAuthURL:    key.URL(),
		RecoveryCodes: recoveryCodes,
	})
}

// VerifyTOTP verifies a TOTP code or recovery code and trusts the device.
func (h *AuthHandler) VerifyTOTP(c fiber.Ctx) error {
	accountID, err := h.requireAccountID(c)
	if err != nil {
		return err
	}

	var req TOTPVerifyRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	req.Code = strings.TrimSpace(req.Code)
	req.RecoveryCode = strings.TrimSpace(req.RecoveryCode)
	if req.Code == "" && req.RecoveryCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "TOTP code or recovery code is required",
		})
	}

	ctx := c.Context()
	encrypted, err := h.db.GetTOTPSecret(ctx, accountID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "TOTP not configured",
		})
	}

	secret, err := h.kmsClient.Decrypt(ctx, encrypted)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to decrypt TOTP secret",
		})
	}

	recoveryUsed := false
	if req.RecoveryCode != "" {
		ok, err := h.db.UseRecoveryCode(ctx, accountID, normalizeRecoveryCode(req.RecoveryCode))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to verify recovery code",
			})
		}
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid recovery code",
			})
		}
		recoveryUsed = true
	} else {
		if !totp.Validate(req.Code, secret) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid TOTP code",
			})
		}
	}

	// Enable TOTP on successful verification.
	if err := h.db.SetTOTPEnabled(ctx, accountID, true); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to enable TOTP",
		})
	}

	deviceToken, err := generateDeviceToken()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate device token",
		})
	}

	expiresAt, err := parseDeviceTTL(req.DeviceTTLDays)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	label := strings.TrimSpace(req.DeviceLabel)
	if label == "" {
		label = defaultDeviceLabel(c)
	}

	if _, err := h.db.CreateDeviceToken(ctx, accountID, deviceToken, label, expiresAt); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to trust device",
		})
	}

	h.setDeviceCookie(c, deviceToken, expiresAt)

	return c.JSON(TOTPVerifyResponse{
		DeviceToken:      deviceToken,
		ExpiresAt:        expiresAt,
		RecoveryCodeUsed: recoveryUsed,
	})
}

// ListDevices lists trusted devices for the account.
func (h *AuthHandler) ListDevices(c fiber.Ctx) error {
	accountID, err := h.requireAccountID(c)
	if err != nil {
		return err
	}

	devices, err := h.db.ListDevices(c.Context(), accountID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list devices",
		})
	}

	return c.JSON(fiber.Map{
		"devices": devices,
	})
}

// RevokeDevice revokes a single device or all devices.
func (h *AuthHandler) RevokeDevice(c fiber.Ctx) error {
	accountID, err := h.requireAccountID(c)
	if err != nil {
		return err
	}

	var req DeviceRevokeRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx := c.Context()
	if req.All {
		if err := h.db.RevokeAllDevices(ctx, accountID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to revoke devices",
			})
		}
		return c.JSON(fiber.Map{"revoked": "all"})
	}

	if req.DeviceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "device_id is required",
		})
	}
	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid device_id",
		})
	}
	if err := h.db.RevokeDevice(ctx, accountID, deviceID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to revoke device",
		})
	}
	return c.JSON(fiber.Map{"revoked": req.DeviceID})
}

// RequireTrustedDevice enforces device trust when wallet escrow is enabled.
func (h *AuthHandler) RequireTrustedDevice() fiber.Handler {
	return func(c fiber.Ctx) error {
		accountID, err := h.requireAccountID(c)
		if err != nil {
			return err
		}

		ctx := c.Context()
		account, err := h.db.GetAccountByID(ctx, accountID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Account not found",
			})
		}

		if !account.WalletEscrow {
			return c.Next()
		}

		deviceToken := getDeviceToken(c)
		if deviceToken == "" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":         "TOTP required",
				"totp_required": true,
			})
		}

		if _, err := h.db.GetDeviceByToken(ctx, accountID, deviceToken); err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":         "TOTP required",
				"totp_required": true,
			})
		}

		_ = h.db.TouchDevice(ctx, accountID, deviceToken)
		return c.Next()
	}
}

func (h *AuthHandler) setDeviceCookie(c fiber.Ctx, token string, expiresAt *time.Time) {
	cookie := &fiber.Cookie{
		Name:     deviceTokenCookie,
		Value:    token,
		HTTPOnly: true,
		Secure:   h.config.Cookie.Secure,
		SameSite: h.parseSameSite(),
		Path:     "/",
		Domain:   h.config.Cookie.Domain,
	}
	if expiresAt != nil {
		cookie.Expires = *expiresAt
	} else {
		cookie.Expires = time.Now().UTC().AddDate(10, 0, 0)
	}
	c.Cookie(cookie)
}

func getDeviceToken(c fiber.Ctx) string {
	if token := c.Get(deviceTokenHeader); token != "" {
		return token
	}
	return c.Cookies(deviceTokenCookie)
}

func (h *AuthHandler) requireAccountID(c fiber.Ctx) (uuid.UUID, error) {
	accountIDStr := c.Locals("account_id")
	if accountIDStr == nil {
		return uuid.UUID{}, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Not authenticated",
		})
	}
	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		return uuid.UUID{}, c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid account ID",
		})
	}
	return accountID, nil
}

func parseDeviceTTL(days int) (*time.Time, error) {
	if days == 0 {
		return nil, nil
	}
	if days != 30 && days != 90 {
		return nil, fmt.Errorf("device_ttl_days must be 30, 90, or 0 for indefinite")
	}
	exp := time.Now().UTC().AddDate(0, 0, days)
	return &exp, nil
}

func generateDeviceToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateRecoveryCodes(count int) ([]string, error) {
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	codes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		b := make([]byte, 10)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		raw := strings.ToUpper(encoder.EncodeToString(b))
		codes = append(codes, formatRecoveryCode(raw))
	}
	return codes, nil
}

func formatRecoveryCode(code string) string {
	if len(code) < 16 {
		return code
	}
	return fmt.Sprintf("%s-%s-%s-%s", code[0:4], code[4:8], code[8:12], code[12:16])
}

func normalizeRecoveryCode(code string) string {
	code = strings.ToUpper(code)
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func normalizeRecoveryCodes(codes []string) []string {
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		out = append(out, normalizeRecoveryCode(code))
	}
	return out
}

func defaultDeviceLabel(c fiber.Ctx) string {
	ua := strings.TrimSpace(string(c.Request().Header.UserAgent()))
	if ua == "" {
		return "unknown device"
	}
	if len(ua) > 80 {
		return ua[:80]
	}
	return ua
}
