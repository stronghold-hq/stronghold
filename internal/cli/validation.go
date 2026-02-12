package cli

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// ValidationError represents a validation error with field context
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// ValidatePrivateKeyHex validates a private key hex string.
// Returns the cleaned key (without 0x prefix) or an error.
// The key must be exactly 64 hex characters (32 bytes) after removing the optional 0x prefix.
func ValidatePrivateKeyHex(privateKey string) (string, error) {
	cleaned := strings.TrimPrefix(privateKey, "0x")

	if len(cleaned) != PrivateKeyHexLength {
		return "", &ValidationError{
			Field:   "private_key",
			Message: fmt.Sprintf("invalid length: expected %d hex characters, got %d", PrivateKeyHexLength, len(cleaned)),
		}
	}

	if _, err := hex.DecodeString(cleaned); err != nil {
		return "", &ValidationError{
			Field:   "private_key",
			Message: "invalid hex format: must contain only 0-9, a-f characters",
		}
	}

	return cleaned, nil
}

// ValidatePrivateKeyBase58 validates a Solana private key in base58 format.
// Returns the cleaned key or an error.
// Solana Ed25519 private keys are 64 bytes (base58 encoded, typically 87-88 chars).
func ValidatePrivateKeyBase58(privateKey string) (string, error) {
	cleaned := strings.TrimSpace(privateKey)

	if len(cleaned) == 0 {
		return "", &ValidationError{
			Field:   "solana_private_key",
			Message: "private key is empty",
		}
	}

	// Base58 decode to validate format
	decoded, err := base58Decode(cleaned)
	if err != nil {
		return "", &ValidationError{
			Field:   "solana_private_key",
			Message: "invalid base58 format",
		}
	}

	// Ed25519 private key is 64 bytes
	if len(decoded) != 64 {
		return "", &ValidationError{
			Field:   "solana_private_key",
			Message: fmt.Sprintf("invalid key length: expected 64 bytes, got %d", len(decoded)),
		}
	}

	return cleaned, nil
}

// base58Decode decodes a base58-encoded string (Bitcoin alphabet).
func base58Decode(input string) ([]byte, error) {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	result := make([]byte, 0, len(input))
	for i := 0; i < len(input); i++ {
		idx := strings.IndexByte(alphabet, input[i])
		if idx < 0 {
			return nil, fmt.Errorf("invalid base58 character: %c", input[i])
		}
		carry := idx
		for j := 0; j < len(result); j++ {
			carry += int(result[j]) * 58
			result[j] = byte(carry & 0xff)
			carry >>= 8
		}
		for carry > 0 {
			result = append(result, byte(carry&0xff))
			carry >>= 8
		}
	}

	// Add leading zeros
	for i := 0; i < len(input) && input[i] == '1'; i++ {
		result = append(result, 0)
	}

	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}
