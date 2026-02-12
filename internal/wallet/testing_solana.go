package wallet

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/token"
)

// TestSolanaWallet is a Solana wallet that doesn't require OS keyring or RPC access.
// Use this for unit and integration tests.
type TestSolanaWallet struct {
	privateKey ed25519.PrivateKey
	PublicKey  solana.PublicKey
	network    string
}

// NewTestSolanaWallet creates a new test Solana wallet with a random keypair
func NewTestSolanaWallet() (*TestSolanaWallet, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	return &TestSolanaWallet{
		privateKey: priv,
		PublicKey:  solana.PublicKeyFromBytes(pub),
		network:    "solana-devnet",
	}, nil
}

// AddressString returns the base58-encoded public key
func (w *TestSolanaWallet) AddressString() string {
	return w.PublicKey.String()
}

// Exists returns true since a test wallet always exists once created
func (w *TestSolanaWallet) Exists() bool {
	return w.privateKey != nil
}

// SetNetwork sets the network for this wallet
func (w *TestSolanaWallet) SetNetwork(network string) {
	w.network = network
}

// CreateX402Payment creates a signed x402 payment for Solana testing.
// Builds a Solana transaction with a dummy blockhash (no RPC needed).
func (w *TestSolanaWallet) CreateX402Payment(req *PaymentRequirements) (string, error) {
	x402Config, ok := x402NetworkConfigs[req.Network]
	if !ok {
		return "", fmt.Errorf("unsupported network: %s", req.Network)
	}

	amount := new(big.Int)
	if _, ok := amount.SetString(req.Amount, 10); !ok {
		return "", fmt.Errorf("invalid amount: %s", req.Amount)
	}

	txBase64, err := w.buildTestTransaction(req, x402Config, amount)
	if err != nil {
		return "", fmt.Errorf("failed to build transaction: %w", err)
	}

	nonce, err := generateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	payload := X402Payload{
		Network:      req.Network,
		Scheme:       "x402",
		Payer:        w.AddressString(),
		Receiver:     req.Recipient,
		TokenAddress: x402Config.TokenAddress,
		Amount:       req.Amount,
		Timestamp:    timeNow().Unix(),
		Nonce:        nonce,
		Transaction:  txBase64,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	payment := fmt.Sprintf("x402;%s", base64.StdEncoding.EncodeToString(payloadJSON))
	return payment, nil
}

// buildTestTransaction constructs a Solana transaction with a dummy blockhash for testing
func (w *TestSolanaWallet) buildTestTransaction(req *PaymentRequirements, x402Config X402Config, amount *big.Int) (string, error) {
	mintPubkey := solana.MustPublicKeyFromBase58(x402Config.TokenAddress)
	recipientPubkey := solana.MustPublicKeyFromBase58(req.Recipient)

	sourceATA, _, err := solana.FindAssociatedTokenAddress(w.PublicKey, mintPubkey)
	if err != nil {
		return "", fmt.Errorf("failed to derive source ATA: %w", err)
	}

	destATA, _, err := solana.FindAssociatedTokenAddress(recipientPubkey, mintPubkey)
	if err != nil {
		return "", fmt.Errorf("failed to derive destination ATA: %w", err)
	}

	instructions := []solana.Instruction{
		computebudget.NewSetComputeUnitLimitInstruction(200_000).Build(),
		computebudget.NewSetComputeUnitPriceInstruction(1).Build(),
		associatedtokenaccount.NewCreateInstruction(w.PublicKey, recipientPubkey, mintPubkey).Build(),
		token.NewTransferCheckedInstruction(
			amount.Uint64(), USDCSolanaDecimals,
			sourceATA, mintPubkey, destATA, w.PublicKey,
			[]solana.PublicKey{},
		).Build(),
	}

	feePayer := w.PublicKey
	if req.FeePayer != "" {
		feePayer = solana.MustPublicKeyFromBase58(req.FeePayer)
	}

	tx, err := solana.NewTransaction(
		instructions,
		solana.Hash{}, // Dummy blockhash for testing
		solana.TransactionPayer(feePayer),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	solanaPrivKey := solana.PrivateKey(w.privateKey)
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(w.PublicKey) {
			return &solanaPrivKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to serialize transaction: %w", err)
	}

	return base64.StdEncoding.EncodeToString(txBytes), nil
}

// CreateTestPaymentHeader creates a Solana payment header for testing with the given parameters
func (w *TestSolanaWallet) CreateTestPaymentHeader(recipient string, amount string, network string) (string, error) {
	req := &PaymentRequirements{
		Scheme:    "x402",
		Network:   network,
		Recipient: recipient,
		Amount:    amount,
		Currency:  "USDC",
	}
	return w.CreateX402Payment(req)
}
