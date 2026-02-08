package cli

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
)

func defaultDeviceLabel() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return fmt.Sprintf("%s (%s)", host, runtime.GOOS)
}

func promptDeviceTTL() int {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Trust this device for:")
	fmt.Println("  1) 30 days")
	fmt.Println("  2) 90 days")
	fmt.Println("  3) Indefinitely (default)")
	fmt.Print("Select [1-3]: ")
	raw, _ := reader.ReadString('\n')
	switch strings.TrimSpace(raw) {
	case "1":
		return 30
	case "2":
		return 90
	default:
		return 0
	}
}

func promptTOTPCode() (string, bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Use recovery code instead? [y/N]: ")
	resp, _ := reader.ReadString('\n')
	resp = strings.ToLower(strings.TrimSpace(resp))
	if resp == "y" || resp == "yes" {
		fmt.Print("Enter recovery code: ")
		code, err := reader.ReadString('\n')
		return strings.TrimSpace(code), true, err
	}
	for {
		fmt.Print("Enter TOTP code: ")
		code, err := reader.ReadString('\n')
		if err != nil {
			return "", false, err
		}
		code = strings.TrimSpace(code)
		if isValidTOTPCode(code) {
			return code, false, nil
		}
		fmt.Println("Invalid TOTP format. Enter a 6-digit code.")
	}
}

func isValidTOTPCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func printTOTPSetup(setup *TOTPSetupResponse) {
	fmt.Println()
	fmt.Println(successStyle.Render("✓ TOTP setup required for server wallet storage"))
	fmt.Println("Secret:", setup.Secret)
	fmt.Println("OTPAuth URL:", setup.OTPAuthURL)
	if len(setup.RecoveryCodes) > 0 {
		fmt.Println()
		fmt.Println(warningStyle.Render("IMPORTANT: Save your recovery codes. They are shown once."))
		for _, code := range setup.RecoveryCodes {
			fmt.Println(" -", code)
		}
	}
	fmt.Println()
}

func ensureTrustedDevice(apiClient *APIClient, config *CLIConfig, totpRequired bool) error {
	if !totpRequired {
		return nil
	}
	fmt.Println(warningStyle.Render("TOTP required to trust this device"))
	code, isRecovery, err := promptTOTPCode()
	if err != nil {
		return fmt.Errorf("failed to read TOTP: %w", err)
	}
	ttlDays := promptDeviceTTL()
	req := &TOTPVerifyRequest{
		DeviceLabel:   defaultDeviceLabel(),
		DeviceTTLDays: ttlDays,
	}
	if isRecovery {
		req.RecoveryCode = code
	} else {
		req.Code = code
	}

	resp, err := apiClient.VerifyTOTP(req)
	if err != nil {
		return err
	}
	config.Auth.DeviceToken = resp.DeviceToken
	apiClient.SetDeviceToken(resp.DeviceToken)
	return nil
}
