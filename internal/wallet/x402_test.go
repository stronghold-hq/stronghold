package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateNonce(t *testing.T) {
	nonce, err := generateNonce()
	require.NoError(t, err)

	// Nonce should be 64 hex characters (32 bytes = 256 bits)
	assert.Len(t, nonce, 64, "nonce should be 64 hex characters (256 bits)")

	// Verify it's valid hex
	for _, c := range nonce {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"nonce should only contain hex characters")
	}
}

func TestGenerateNonce_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		nonce, err := generateNonce()
		require.NoError(t, err)

		assert.False(t, seen[nonce], "nonce collision detected")
		seen[nonce] = true
	}
}

func TestGetChainID(t *testing.T) {
	testCases := []struct {
		network  string
		expected int
	}{
		{"base", 8453},
		{"base-sepolia", 84532},
		{"unknown", 8453}, // Defaults to base mainnet
	}

	for _, tc := range testCases {
		t.Run(tc.network, func(t *testing.T) {
			result := getChainID(tc.network)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParseX402Payment_InvalidFormat(t *testing.T) {
	testCases := []struct {
		name    string
		header  string
		wantErr bool
	}{
		{"empty string", "", true},
		{"no scheme", "base64payload", true},
		{"wrong scheme", "x401;base64payload", true},
		{"invalid base64", "x402;not-valid-base64!!!", true},
		{"valid format but invalid json", "x402;aW52YWxpZCBqc29u", true}, // "invalid json" in base64
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseX402Payment(tc.header)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetTokenDecimals(t *testing.T) {
	// USDC has 6 decimals
	assert.Equal(t, 6, GetTokenDecimals(USDCBaseAddress))
	assert.Equal(t, 6, GetTokenDecimals("0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"))

	// Unknown tokens default to 18
	assert.Equal(t, 18, GetTokenDecimals("0x0000000000000000000000000000000000000000"))
}
