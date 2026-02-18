package usdc

// ChainDecimals defines the USDC token decimal places per chain.
// This is the single source of truth — never hardcode decimals elsewhere.
var ChainDecimals = map[string]int{
	"base":          6,
	"base-sepolia":  6,
	"solana":        6,
	"solana-devnet": 6,
}
