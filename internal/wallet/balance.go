package wallet

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// QueryEVMBalance returns the USDC balance (as a human-readable float) for an
// EVM address on the given network.  No private key is needed -- only a public
// address and an RPC call.
//
// Supported networks: "base", "base-sepolia".
func QueryEVMBalance(ctx context.Context, address string, network string) (float64, error) {
	rpcURL, usdcAddr, err := evmNetworkParams(network)
	if err != nil {
		return 0, err
	}

	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to %s RPC: %w", network, err)
	}
	defer client.Close()

	addr := common.HexToAddress(address)

	// Build balanceOf(address) call data
	data := append(usdcBalanceOfSelector, common.LeftPadBytes(addr.Bytes(), 32)...)

	msg := map[string]interface{}{
		"to":   usdcAddr,
		"data": hex.EncodeToString(data),
	}

	var result string
	if err := client.Client().CallContext(ctx, &result, "eth_call", msg, "latest"); err != nil {
		return 0, fmt.Errorf("eth_call balanceOf failed: %w", err)
	}

	balance := new(big.Int)
	balance.SetString(strings.TrimPrefix(result, "0x"), 16)

	// USDC has 6 decimals
	balanceFloat := new(big.Float).SetInt(balance)
	divisor := big.NewFloat(1_000_000)
	human, _ := new(big.Float).Quo(balanceFloat, divisor).Float64()

	return human, nil
}

// QuerySolanaBalance returns the USDC balance (as a human-readable float) for
// a Solana address on the given network.  No private key is needed.
//
// Supported networks: "solana", "solana-devnet".
func QuerySolanaBalance(ctx context.Context, address string, network string) (float64, error) {
	rpcURL, usdcMint, err := solanaNetworkParams(network)
	if err != nil {
		return 0, err
	}

	client := rpc.New(rpcURL)

	ownerPubkey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return 0, fmt.Errorf("invalid Solana address %q: %w", address, err)
	}
	mintPubkey, err := solana.PublicKeyFromBase58(usdcMint)
	if err != nil {
		return 0, fmt.Errorf("invalid USDC mint %q: %w", usdcMint, err)
	}

	// Derive the associated token account (ATA) for USDC
	ata, _, err := solana.FindAssociatedTokenAddress(ownerPubkey, mintPubkey)
	if err != nil {
		return 0, fmt.Errorf("failed to derive ATA: %w", err)
	}

	result, err := client.GetTokenAccountBalance(ctx, ata, rpc.CommitmentFinalized)
	if err != nil {
		// If the ATA doesn't exist yet the balance is zero.
		if strings.Contains(err.Error(), "could not find account") ||
			strings.Contains(err.Error(), "Invalid param") {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get token balance: %w", err)
	}

	balance := new(big.Int)
	if _, ok := balance.SetString(result.Value.Amount, 10); !ok {
		return 0, fmt.Errorf("failed to parse balance: %s", result.Value.Amount)
	}

	// USDC has 6 decimals on Solana
	balanceFloat := new(big.Float).SetInt(balance)
	divisor := big.NewFloat(1_000_000)
	human, _ := new(big.Float).Quo(balanceFloat, divisor).Float64()

	return human, nil
}

// evmNetworkParams returns the RPC URL and USDC contract address for the given
// EVM network name.
func evmNetworkParams(network string) (rpcURL string, usdcAddr string, err error) {
	switch network {
	case "base":
		return BaseMainnetRPC, USDCBaseAddress, nil
	case "base-sepolia":
		return BaseSepoliaRPC, x402NetworkConfigs["base-sepolia"].TokenAddress, nil
	default:
		return "", "", fmt.Errorf("unsupported EVM network: %s", network)
	}
}

// solanaNetworkParams returns the RPC URL and USDC mint for the given Solana
// network name.
func solanaNetworkParams(network string) (rpcURL string, usdcMint string, err error) {
	switch network {
	case "solana":
		return SolanaMainnetRPC, USDCSolanaMint, nil
	case "solana-devnet":
		return SolanaDevnetRPC, USDCSolanaDevnetMint, nil
	default:
		return "", "", fmt.Errorf("unsupported Solana network: %s", network)
	}
}
