package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"stronghold/internal/wallet"
)

var (
	walletTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00D4AA"))

	walletAddressStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF"))

	walletBalanceStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00D4AA"))

	walletInfoStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	walletWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFA500"))

	walletErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF4444"))
)

// WalletShow displays the wallet information
func WalletShow() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !config.Auth.LoggedIn {
		fmt.Println(walletErrorStyle.Render("✗ Not logged in"))
		fmt.Println(walletInfoStyle.Render("Run 'stronghold install' to set up your account and wallet"))
		return nil
	}

	if config.Wallet.Address == "" {
		fmt.Println(walletWarningStyle.Render("⚠ No wallet configured"))
		fmt.Println(walletInfoStyle.Render("Your account doesn't have a wallet yet."))
		fmt.Println(walletInfoStyle.Render("This will be set up automatically during account creation."))
		return nil
	}

	// Load wallet to check balance
	w, err := wallet.New(wallet.Config{
		UserID:  config.Auth.UserID,
		Network: config.Wallet.Network,
	})
	if err != nil {
		return fmt.Errorf("failed to load wallet: %w", err)
	}

	fmt.Println(walletTitleStyle.Render("💳 Stronghold Wallet"))
	fmt.Println()

	// Display address
	fmt.Println("Address:")
	fmt.Println(walletAddressStyle.Render("  " + config.Wallet.Address))
	fmt.Println()

	// Display network
	networkDisplay := "Base Mainnet"
	if config.Wallet.Network == "base-sepolia" {
		networkDisplay = "Base Sepolia (Testnet)"
	}
	fmt.Printf("Network: %s\n", walletInfoStyle.Render(networkDisplay))
	fmt.Println()

	// Check balance
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	balance, err := w.GetBalanceHuman(ctx)
	if err != nil {
		fmt.Println(walletWarningStyle.Render("⚠ Could not fetch balance"))
		fmt.Println(walletInfoStyle.Render(fmt.Sprintf("  Error: %v", err)))
	} else {
		fmt.Printf("Balance: %s\n", walletBalanceStyle.Render(fmt.Sprintf("%.6f USDC", balance)))
		if balance < 1.0 {
			fmt.Println()
			fmt.Println(walletWarningStyle.Render("⚠ Low balance"))
			fmt.Println(walletInfoStyle.Render("  Visit https://dashboard.stronghold.security to add funds"))
			fmt.Println(walletInfoStyle.Render("  Or send USDC directly to your wallet address above"))
		}
	}

	fmt.Println()
	fmt.Println(walletInfoStyle.Render("To add funds:"))
	fmt.Println(walletInfoStyle.Render("  1. Visit https://dashboard.stronghold.security"))
	fmt.Println(walletInfoStyle.Render("  2. Sign in with your account"))
	fmt.Println(walletInfoStyle.Render("  3. Use Stripe on-ramp or send USDC directly"))

	return nil
}

// WalletAddress returns just the wallet address (useful for scripting)
func WalletAddress() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if config.Wallet.Address == "" {
		return fmt.Errorf("no wallet configured")
	}

	fmt.Println(config.Wallet.Address)
	return nil
}

// WalletBalance returns just the balance (useful for scripting)
func WalletBalance() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if config.Wallet.Address == "" {
		return fmt.Errorf("no wallet configured")
	}

	w, err := wallet.New(wallet.Config{
		UserID:  config.Auth.UserID,
		Network: config.Wallet.Network,
	})
	if err != nil {
		return fmt.Errorf("failed to load wallet: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	balance, err := w.GetBalanceHuman(ctx)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	fmt.Printf("%.6f\n", balance)
	return nil
}

// SetupWallet creates or loads a wallet for the user during install
// In production, this would call the backend API which creates the wallet
// and returns the address. For now, we create it locally.
func SetupWallet(userID string, network string) (string, error) {
	w, err := wallet.New(wallet.Config{
		UserID:  userID,
		Network: network,
	})
	if err != nil {
		return "", fmt.Errorf("failed to initialize wallet: %w", err)
	}

	// Check if wallet already exists
	if w.Exists() {
		return w.AddressString(), nil
	}

	// Create new wallet
	if _, err := w.Create(); err != nil {
		return "", fmt.Errorf("failed to create wallet: %w", err)
	}

	return w.AddressString(), nil
}
