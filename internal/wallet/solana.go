package wallet

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/99designs/keyring"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/memo"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"
)

const (
	// USDC SPL token mint on Solana mainnet
	USDCSolanaMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	// USDC SPL token mint on Solana devnet
	USDCSolanaDevnetMint = "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"
	// Solana mainnet RPC
	SolanaMainnetRPC = "https://api.mainnet-beta.solana.com"
	// Solana devnet RPC
	SolanaDevnetRPC = "https://api.devnet.solana.com"
	// USDC has 6 decimals on Solana (same as EVM)
	USDCSolanaDecimals = 6
)

// SolanaWallet represents a Solana wallet using Ed25519 keypair.
// Private keys are stored in the OS keyring, same as the EVM wallet.
type SolanaWallet struct {
	PublicKey solana.PublicKey
	userID    string
	keyring   keyring.Keyring
	network   string
	rpcURL    string
}

// SolanaConfig holds Solana wallet configuration
type SolanaConfig struct {
	UserID  string
	Network string // "solana" or "solana-devnet"
}

// NewSolana creates or loads a Solana wallet for the given user
func NewSolana(cfg SolanaConfig) (*SolanaWallet, error) {
	rpcURL := SolanaMainnetRPC
	if cfg.Network == "solana-devnet" {
		rpcURL = SolanaDevnetRPC
	}

	ring, err := openKeyring()
	if err != nil {
		return nil, fmt.Errorf("failed to open keyring: %w", err)
	}

	w := &SolanaWallet{
		userID:  cfg.UserID,
		keyring: ring,
		network: cfg.Network,
		rpcURL:  rpcURL,
	}

	// Try to load existing wallet
	if err := w.load(); err != nil {
		// No existing wallet, will need to be created
		return w, nil
	}

	return w, nil
}

// Create generates a new Solana Ed25519 keypair
func (w *SolanaWallet) Create() (*SolanaWallet, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key: %w", err)
	}

	w.PublicKey = solana.PublicKeyFromBytes(pub)

	// Store private key as base58 in keyring (Solana convention)
	privKeyBase58 := base58.Encode(priv)
	if err := w.keyring.Set(keyring.Item{
		Key:  w.keyID(),
		Data: []byte(privKeyBase58),
	}); err != nil {
		return nil, fmt.Errorf("failed to store key: %w", err)
	}

	return w, nil
}

// Import creates a Solana wallet from an existing base58-encoded private key
func (w *SolanaWallet) Import(privateKeyBase58 string) (*SolanaWallet, error) {
	privKeyBytes, err := base58.Decode(privateKeyBase58)
	if err != nil {
		return nil, fmt.Errorf("invalid base58 private key: %w", err)
	}

	if len(privKeyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: expected %d bytes, got %d", ed25519.PrivateKeySize, len(privKeyBytes))
	}

	priv := ed25519.PrivateKey(privKeyBytes)
	pub := priv.Public().(ed25519.PublicKey)
	w.PublicKey = solana.PublicKeyFromBytes(pub)

	if err := w.keyring.Set(keyring.Item{
		Key:  w.keyID(),
		Data: []byte(privateKeyBase58),
	}); err != nil {
		return nil, fmt.Errorf("failed to store key: %w", err)
	}

	return w, nil
}

// Export returns the private key as a base58 string
func (w *SolanaWallet) Export() (string, error) {
	item, err := w.keyring.Get(w.keyID())
	if err != nil {
		return "", fmt.Errorf("wallet not found: %w", err)
	}
	return string(item.Data), nil
}

// AddressString returns the base58-encoded public key
func (w *SolanaWallet) AddressString() string {
	return w.PublicKey.String()
}

// Exists returns true if a Solana wallet exists for this user
func (w *SolanaWallet) Exists() bool {
	_, err := w.keyring.Get(w.keyID())
	return err == nil
}

// Delete removes the Solana wallet from the keyring
func (w *SolanaWallet) Delete() error {
	return w.keyring.Remove(w.keyID())
}

// GetNetwork returns the network this wallet is configured for
func (w *SolanaWallet) GetNetwork() string {
	return w.network
}

// GetBalance returns the USDC SPL token balance for this wallet
func (w *SolanaWallet) GetBalance(ctx context.Context) (*big.Int, error) {
	if w.PublicKey.IsZero() {
		return nil, fmt.Errorf("wallet not initialized")
	}

	client := rpc.New(w.rpcURL)

	mintPubkey := solana.MustPublicKeyFromBase58(w.usdcMint())

	// Find the associated token account for USDC
	ata, _, err := solana.FindAssociatedTokenAddress(w.PublicKey, mintPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive ATA: %w", err)
	}

	// Get token account balance
	result, err := client.GetTokenAccountBalance(ctx, ata, rpc.CommitmentFinalized)
	if err != nil {
		// If account doesn't exist, balance is 0
		if strings.Contains(err.Error(), "could not find account") ||
			strings.Contains(err.Error(), "Invalid param") {
			return big.NewInt(0), nil
		}
		return nil, fmt.Errorf("failed to get token balance: %w", err)
	}

	balance := new(big.Int)
	if _, ok := balance.SetString(result.Value.Amount, 10); !ok {
		return nil, fmt.Errorf("failed to parse balance: %s", result.Value.Amount)
	}

	return balance, nil
}

// GetBalanceHuman returns the USDC balance as a human-readable float
func (w *SolanaWallet) GetBalanceHuman(ctx context.Context) (float64, error) {
	balance, err := w.GetBalance(ctx)
	if err != nil {
		return 0, err
	}

	balanceFloat := new(big.Float).SetInt(balance)
	divisor := big.NewFloat(1_000_000) // USDC has 6 decimals
	result, _ := new(big.Float).Quo(balanceFloat, divisor).Float64()

	return result, nil
}

// CreateX402Payment creates a signed x402 payment for Solana.
// Builds a Solana transaction with SPL TransferChecked, partially signs it,
// and encodes it in the x402 payment header format.
func (w *SolanaWallet) CreateX402Payment(req *PaymentRequirements) (string, error) {
	if !w.Exists() {
		return "", fmt.Errorf("wallet not initialized")
	}

	x402Config, ok := x402NetworkConfigs[req.Network]
	if !ok {
		return "", fmt.Errorf("unsupported network: %s", req.Network)
	}

	// Parse amount
	amount := new(big.Int)
	if _, ok := amount.SetString(req.Amount, 10); !ok {
		return "", fmt.Errorf("invalid amount: %s", req.Amount)
	}

	// Get private key for signing
	privKey, err := w.getPrivateKey()
	if err != nil {
		return "", fmt.Errorf("failed to get private key: %w", err)
	}

	// Build the Solana transaction
	txBase64, err := w.buildTransferTransaction(req, x402Config, amount, privKey)
	if err != nil {
		return "", fmt.Errorf("failed to build transaction: %w", err)
	}

	// Generate nonce for uniqueness
	nonce, err := generateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Build the payload with Transaction field (Solana-specific)
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

// buildTransferTransaction constructs a Solana SPL Token transfer transaction
func (w *SolanaWallet) buildTransferTransaction(req *PaymentRequirements, x402Config X402Config, amount *big.Int, privKey ed25519.PrivateKey) (string, error) {
	ctx := context.Background()
	client := rpc.New(w.rpcURL)

	mintPubkey := solana.MustPublicKeyFromBase58(x402Config.TokenAddress)
	recipientPubkey := solana.MustPublicKeyFromBase58(req.Recipient)

	// Derive source ATA (sender's USDC token account)
	sourceATA, _, err := solana.FindAssociatedTokenAddress(w.PublicKey, mintPubkey)
	if err != nil {
		return "", fmt.Errorf("failed to derive source ATA: %w", err)
	}

	// Derive destination ATA (recipient's USDC token account)
	destATA, _, err := solana.FindAssociatedTokenAddress(recipientPubkey, mintPubkey)
	if err != nil {
		return "", fmt.Errorf("failed to derive destination ATA: %w", err)
	}

	// Get recent blockhash
	recentBlockhash, err := client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("failed to get blockhash: %w", err)
	}

	// Build instructions
	instructions := []solana.Instruction{
		// 1. Set compute unit limit
		computebudget.NewSetComputeUnitLimitInstruction(200_000).Build(),
		// 2. Set compute unit price (minimal, facilitator pays fees)
		computebudget.NewSetComputeUnitPriceInstruction(1).Build(),
	}

	// 3. Create destination ATA if needed
	createATAInstruction := associatedtokenaccount.NewCreateInstruction(
		w.PublicKey,     // payer
		recipientPubkey, // wallet address
		mintPubkey,      // mint
	).Build()
	instructions = append(instructions, createATAInstruction)

	// 4. SPL Token TransferChecked
	transferInstruction := token.NewTransferCheckedInstruction(
		amount.Uint64(),     // amount
		USDCSolanaDecimals,  // decimals
		sourceATA,           // source
		mintPubkey,          // mint
		destATA,             // destination
		w.PublicKey,         // owner/authority
		[]solana.PublicKey{}, // no multisig signers
	).Build()
	instructions = append(instructions, transferInstruction)

	// 5. Memo for uniqueness (anti-replay)
	memoNonce := make([]byte, 16)
	if _, err := rand.Read(memoNonce); err != nil {
		return "", fmt.Errorf("failed to generate memo nonce: %w", err)
	}
	memoInstruction := memo.NewMemoInstruction(
		[]byte(fmt.Sprintf("x402:%x", memoNonce)),
		w.PublicKey,
	).Build()
	instructions = append(instructions, memoInstruction)

	// Determine fee payer: use facilitator's pubkey if provided, otherwise self
	feePayer := w.PublicKey
	if req.FeePayer != "" {
		feePayer = solana.MustPublicKeyFromBase58(req.FeePayer)
	}

	// Build the transaction
	tx, err := solana.NewTransaction(
		instructions,
		recentBlockhash.Value.Blockhash,
		solana.TransactionPayer(feePayer),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	// Partially sign with our key
	solanaPrivKey := solana.PrivateKey(privKey)
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(w.PublicKey) {
			return &solanaPrivKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Serialize to base64
	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to serialize transaction: %w", err)
	}

	return base64.StdEncoding.EncodeToString(txBytes), nil
}

// Private helpers

func (w *SolanaWallet) keyID() string {
	return fmt.Sprintf("solana-wallet-%s", w.userID)
}

func (w *SolanaWallet) load() error {
	item, err := w.keyring.Get(w.keyID())
	if err != nil {
		return err
	}

	privKeyBytes, err := base58.Decode(string(item.Data))
	if err != nil {
		return fmt.Errorf("failed to decode stored key: %w", err)
	}

	if len(privKeyBytes) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid stored key length")
	}

	priv := ed25519.PrivateKey(privKeyBytes)
	pub := priv.Public().(ed25519.PublicKey)
	w.PublicKey = solana.PublicKeyFromBytes(pub)

	return nil
}

func (w *SolanaWallet) getPrivateKey() (ed25519.PrivateKey, error) {
	item, err := w.keyring.Get(w.keyID())
	if err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}

	privKeyBytes, err := base58.Decode(string(item.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %w", err)
	}

	if len(privKeyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid key length")
	}

	return ed25519.PrivateKey(privKeyBytes), nil
}

func (w *SolanaWallet) usdcMint() string {
	if w.network == "solana-devnet" {
		return USDCSolanaDevnetMint
	}
	return USDCSolanaMint
}
