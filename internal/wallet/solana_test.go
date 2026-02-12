package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsSolanaNetwork(t *testing.T) {
	assert.True(t, IsSolanaNetwork("solana"))
	assert.True(t, IsSolanaNetwork("solana-devnet"))
	assert.True(t, IsSolanaNetwork("solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp"))
	assert.False(t, IsSolanaNetwork("base"))
	assert.False(t, IsSolanaNetwork("base-sepolia"))
	assert.False(t, IsSolanaNetwork(""))
}

func TestNetworkToCAIP2_Solana(t *testing.T) {
	assert.Equal(t, "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp", networkToCAIP2("solana"))
	assert.Equal(t, "solana:EtWTRABZaYq6iMfeYKouRu166VU2xqa1", networkToCAIP2("solana-devnet"))
	// Already CAIP-2 format should pass through
	assert.Equal(t, "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp", networkToCAIP2("solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp"))
}

func TestTestSolanaWallet_Create(t *testing.T) {
	w, err := NewTestSolanaWallet()
	require.NoError(t, err)
	assert.True(t, w.Exists())
	assert.NotEmpty(t, w.AddressString())
	// Solana addresses are base58-encoded, 32-44 characters
	assert.GreaterOrEqual(t, len(w.AddressString()), 32)
}

func TestTestSolanaWallet_CreateX402Payment(t *testing.T) {
	w, err := NewTestSolanaWallet()
	require.NoError(t, err)

	recipient, err := NewTestSolanaWallet()
	require.NoError(t, err)

	req := &PaymentRequirements{
		Scheme:    "x402",
		Network:   "solana-devnet",
		Recipient: recipient.AddressString(),
		Amount:    "1000",
		Currency:  "USDC",
	}

	payment, err := w.CreateX402Payment(req)
	require.NoError(t, err)
	assert.NotEmpty(t, payment)

	// Parse and verify roundtrip
	payload, err := ParseX402Payment(payment)
	require.NoError(t, err)
	assert.Equal(t, "solana-devnet", payload.Network)
	assert.Equal(t, "x402", payload.Scheme)
	assert.Equal(t, w.AddressString(), payload.Payer)
	assert.Equal(t, recipient.AddressString(), payload.Receiver)
	assert.Equal(t, "1000", payload.Amount)
	assert.Equal(t, USDCSolanaDevnetMint, payload.TokenAddress)
	assert.NotEmpty(t, payload.Transaction)
	assert.Empty(t, payload.Signature) // Solana uses Transaction, not Signature
	assert.NotZero(t, payload.Timestamp)
	assert.NotEmpty(t, payload.Nonce)
}

func TestTestSolanaWallet_CreateX402Payment_Mainnet(t *testing.T) {
	w, err := NewTestSolanaWallet()
	require.NoError(t, err)

	recipient, err := NewTestSolanaWallet()
	require.NoError(t, err)

	req := &PaymentRequirements{
		Scheme:    "x402",
		Network:   "solana",
		Recipient: recipient.AddressString(),
		Amount:    "500000",
		Currency:  "USDC",
	}

	payment, err := w.CreateX402Payment(req)
	require.NoError(t, err)

	payload, err := ParseX402Payment(payment)
	require.NoError(t, err)
	assert.Equal(t, "solana", payload.Network)
	assert.Equal(t, USDCSolanaMint, payload.TokenAddress)
}

func TestTestSolanaWallet_UnsupportedNetwork(t *testing.T) {
	w, err := NewTestSolanaWallet()
	require.NoError(t, err)

	req := &PaymentRequirements{
		Scheme:    "x402",
		Network:   "unsupported-chain",
		Recipient: w.AddressString(),
		Amount:    "1000",
		Currency:  "USDC",
	}

	_, err = w.CreateX402Payment(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported network")
}

func TestBuildSolanaFacilitatorRequest(t *testing.T) {
	payload := &X402Payload{
		Network:      "solana",
		Scheme:       "x402",
		Payer:        "SomeBase58PublicKey",
		Receiver:     "RecipientBase58PublicKey",
		TokenAddress: USDCSolanaMint,
		Amount:       "1000",
		Timestamp:    1700000000,
		Nonce:        "abc123",
		Transaction:  "base64encodedtransaction",
	}

	req := BuildFacilitatorRequest(payload, nil)
	assert.Equal(t, 2, req.PaymentPayload.X402Version)
	assert.Equal(t, "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp", req.PaymentRequirements.Network)
	assert.Equal(t, "exact", req.PaymentRequirements.Scheme)
	assert.Equal(t, USDCSolanaMint, req.PaymentRequirements.Asset)
	assert.Equal(t, "1000", req.PaymentRequirements.Amount)
	assert.Equal(t, "RecipientBase58PublicKey", req.PaymentRequirements.PayTo)

	// Solana payload should have transaction, not signature/authorization
	assert.Equal(t, "base64encodedtransaction", req.PaymentPayload.Payload["transaction"])
	assert.Nil(t, req.PaymentPayload.Payload["signature"])
	assert.Nil(t, req.PaymentPayload.Payload["authorization"])

	// Check asset transfer method
	extra := req.PaymentRequirements.Extra
	assert.Equal(t, "solana-transfer", extra["assetTransferMethod"])
}

func TestBuildSolanaFacilitatorRequest_Devnet(t *testing.T) {
	payload := &X402Payload{
		Network:      "solana-devnet",
		Scheme:       "x402",
		TokenAddress: USDCSolanaDevnetMint,
		Amount:       "500",
		Receiver:     "Recipient",
		Transaction:  "txdata",
	}

	req := BuildFacilitatorRequest(payload, nil)
	assert.Equal(t, "solana:EtWTRABZaYq6iMfeYKouRu166VU2xqa1", req.PaymentRequirements.Network)
	assert.Equal(t, USDCSolanaDevnetMint, req.PaymentRequirements.Asset)
}

func TestGetTokenDecimals_Solana(t *testing.T) {
	assert.Equal(t, 6, GetTokenDecimals(USDCSolanaMint))
	assert.Equal(t, 6, GetTokenDecimals(USDCSolanaDevnetMint))
}

func TestSolanaX402Configs(t *testing.T) {
	assert.Equal(t, "solana", X402Solana.Network)
	assert.Equal(t, USDCSolanaMint, X402Solana.TokenAddress)
	assert.Equal(t, "solana-devnet", X402SolanaDevnet.Network)
	assert.Equal(t, USDCSolanaDevnetMint, X402SolanaDevnet.TokenAddress)
}
