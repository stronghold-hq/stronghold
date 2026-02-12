package cli

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"stronghold/internal/wallet"

	"golang.org/x/term"
)

// WalletReplace replaces the current wallet with a new private key
// chainFlag: which chain wallet to replace ("base" or "solana", empty = "base")
// fileFlag: path to file containing private key
// yesFlag: skip confirmation warnings
func WalletReplace(chainFlag, fileFlag string, yesFlag bool) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !config.Auth.LoggedIn {
		return fmt.Errorf("not logged in. Run 'stronghold init' first")
	}

	// Default to base if no chain specified
	if chainFlag == "" {
		chainFlag = "base"
	}

	isSolana := chainFlag == "solana"

	// Read private key from various sources (in order of precedence)
	privateKey, err := readPrivateKey(fileFlag, isSolana)
	if err != nil {
		return err
	}
	defer privateKey.Zero() // CRITICAL: Always zero the key when done

	userID := config.Auth.UserID
	if userID == "" {
		userID = generateUserID()
		config.Auth.UserID = userID
	}

	if isSolana {
		// Validate and import Solana key
		cleanedKey, err := ValidatePrivateKeyBase58(privateKey.String())
		if err != nil {
			return fmt.Errorf("invalid Solana private key: %w", err)
		}

		// Warn about existing Solana wallet
		if config.Wallet.SolanaAddress != "" && !yesFlag {
			fmt.Printf("\n%s Existing Solana wallet detected: %s\n", warningStyle.Render("WARNING:"), config.Wallet.SolanaAddress)
			fmt.Println("Any funds not backed up will be lost.")
			fmt.Print("\nType 'yes' to continue or 'no' to cancel: ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil || strings.TrimSpace(strings.ToLower(response)) != "yes" {
				return fmt.Errorf("wallet replacement cancelled")
			}
		}

		solanaNetwork := config.Wallet.SolanaNetwork
		if solanaNetwork == "" {
			solanaNetwork = DefaultSolanaNetwork
		}
		address, err := ImportSolanaWallet(userID, solanaNetwork, cleanedKey)
		if err != nil {
			return fmt.Errorf("failed to import Solana wallet: %w", err)
		}

		config.Wallet.SolanaAddress = address
		config.Wallet.SolanaNetwork = solanaNetwork
		if err := config.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("%s Solana wallet updated: %s\n", successStyle.Render("✓"), address)
	} else {
		// Validate EVM key
		cleanedKey, err := ValidatePrivateKeyHex(privateKey.String())
		if err != nil {
			return fmt.Errorf("invalid EVM private key: %w", err)
		}

		// Check BOTH config AND keyring for existing EVM wallet
		walletExistsInConfig := config.Wallet.Address != ""
		walletExistsInKeyring := false

		if config.Auth.UserID != "" {
			w, err := wallet.New(wallet.Config{
				UserID:  config.Auth.UserID,
				Network: config.Wallet.Network,
			})
			if err == nil {
				walletExistsInKeyring = w.Exists()
			}
		}

		// Warn if wallet exists in EITHER location
		if (walletExistsInConfig || walletExistsInKeyring) && !yesFlag {
			address := config.Wallet.Address
			if address == "" {
				address = "(wallet found in system keyring)"
			}
			fmt.Printf("\n%s Existing EVM wallet detected: %s\n", warningStyle.Render("WARNING:"), address)
			fmt.Println("Any funds not backed up will be lost.")
			fmt.Print("\nType 'yes' to continue or 'no' to cancel: ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil || strings.TrimSpace(strings.ToLower(response)) != "yes" {
				return fmt.Errorf("wallet replacement cancelled")
			}
		}

		address, err := ImportWallet(userID, DefaultBlockchain, cleanedKey)
		if err != nil {
			return fmt.Errorf("failed to import wallet: %w", err)
		}

		uploadToServer := false
		if term.IsTerminal(int(os.Stdin.Fd())) {
			uploadToServer = Confirm("Upload wallet to server for multi-device setup? [y/N]")
		}

		if uploadToServer {
			if config.Auth.AccountNumber == "" {
				return fmt.Errorf("account number missing. Run 'stronghold init' first")
			}

			apiClient := NewAPIClient(config.API.Endpoint, config.Auth.DeviceToken)
			if _, err := apiClient.Login(config.Auth.AccountNumber); err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			if setup, err := apiClient.SetupTOTP(); err == nil {
				printTOTPSetup(setup)
			} else {
				var apiErr *APIError
				if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
					fmt.Println(warningStyle.Render("TOTP already enabled. Enter your existing TOTP or recovery code."))
				} else {
					fmt.Printf("%s TOTP setup: %v\n", warningStyle.Render("WARNING:"), err)
				}
			}

			fmt.Println(warningStyle.Render("WARNING:"))
			fmt.Println("Uploading your private key enables multi-device setup.")
			fmt.Println("If you lose your old key, funds are not recoverable. Back up your key now.")

			code, isRecovery, err := promptTOTPCode()
			if err != nil {
				return fmt.Errorf("failed to read TOTP: %w", err)
			}
			ttlDays := promptDeviceTTL()
			verifyReq := &TOTPVerifyRequest{
				DeviceLabel:   defaultDeviceLabel(),
				DeviceTTLDays: ttlDays,
			}
			if isRecovery {
				verifyReq.RecoveryCode = code
			} else {
				verifyReq.Code = code
			}

			verifyResp, err := apiClient.VerifyTOTP(verifyReq)
			if err != nil {
				return fmt.Errorf("TOTP verification failed: %w", err)
			}

			config.Auth.DeviceToken = verifyResp.DeviceToken
			apiClient.SetDeviceToken(verifyResp.DeviceToken)

			if err := apiClient.UpdateWallet(cleanedKey); err != nil {
				return fmt.Errorf("failed to upload wallet to server: %w", err)
			}
			fmt.Println(successStyle.Render("✓ Wallet updated on server"))
		} else {
			fmt.Println(warningStyle.Render("WARNING:"))
			fmt.Println("Wallet updated locally only. Server sync was skipped.")
			fmt.Println("To upload later, rerun 'stronghold wallet replace' and choose upload when prompted.")
		}

		config.Wallet.Address = address
		config.Wallet.Network = DefaultBlockchain
		if err := config.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("%s Base (EVM) wallet updated: %s\n", successStyle.Render("✓"), address)
	}

	return nil
}

// readPrivateKey reads the private key from various sources in order of precedence:
// 1. stdin (if piped)
// 2. STRONGHOLD_PRIVATE_KEY or STRONGHOLD_SOLANA_PRIVATE_KEY environment variable
// 3. --file flag
// 4. interactive prompt (if terminal)
//
// Returns a SecureBytes that must be zeroed when done (caller should use defer key.Zero()).
func readPrivateKey(fileFlag string, isSolana bool) (*SecureBytes, error) {
	envVar := "STRONGHOLD_PRIVATE_KEY"
	promptLabel := "Enter EVM private key (hex): "
	if isSolana {
		envVar = "STRONGHOLD_SOLANA_PRIVATE_KEY"
		promptLabel = "Enter Solana private key (base58): "
	}

	// 1. Check stdin (if piped)
	stdinInfo, _ := os.Stdin.Stat()
	if (stdinInfo.Mode() & os.ModeCharDevice) == 0 {
		// stdin has data piped to it
		reader := bufio.NewReader(os.Stdin)
		key, err := reader.ReadString('\n')
		if err != nil && key == "" {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
		return NewSecureBytes([]byte(strings.TrimSpace(key))), nil
	}

	// 2. Check environment variable
	if envKey := os.Getenv(envVar); envKey != "" {
		return NewSecureBytes([]byte(strings.TrimSpace(envKey))), nil
	}

	// 3. Check file flag
	if fileFlag != "" {
		info, err := os.Stat(fileFlag)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("key file not found: %s", fileFlag)
			}
			if os.IsPermission(err) {
				return nil, fmt.Errorf("permission denied reading key file: %s", fileFlag)
			}
			return nil, fmt.Errorf("failed to stat key file: %w", err)
		}
		if info.Size() == 0 {
			return nil, fmt.Errorf("key file is empty: %s", fileFlag)
		}
		if info.Size() > MaxKeyFileSize {
			return nil, fmt.Errorf("key file too large: %d bytes (max %d)", info.Size(), MaxKeyFileSize)
		}
		data, err := os.ReadFile(fileFlag)
		if err != nil {
			return nil, fmt.Errorf("failed to read key file: %w", err)
		}
		// Create SecureBytes from file data (trimmed)
		trimmed := strings.TrimSpace(string(data))
		// Zero the original data slice
		for i := range data {
			data[i] = 0
		}
		return NewSecureBytes([]byte(trimmed)), nil
	}

	// 4. Interactive prompt (only if terminal)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Print(promptLabel)
		key, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // newline after password input
		if err != nil {
			return nil, fmt.Errorf("failed to read private key: %w", err)
		}
		// Create SecureBytes and zero original
		trimmed := strings.TrimSpace(string(key))
		for i := range key {
			key[i] = 0
		}
		return NewSecureBytes([]byte(trimmed)), nil
	}

	return nil, fmt.Errorf("no private key provided. Use stdin, %s env var, --file flag, or run interactively", envVar)
}
