package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// Uninstall removes Stronghold from the system
func Uninstall(preserveConfig bool) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !config.Installed {
		fmt.Println("Stronghold is not installed.")
		return nil
	}

	// Confirm uninstallation
	fmt.Println()
	fmt.Println("This will remove Stronghold proxy and configuration.")
	if config.Auth.LoggedIn {
		fmt.Printf("Your wallet balance will remain for future use.\n")
	}
	fmt.Println()

	if !Confirm("Remove Stronghold? [y/N]:") {
		fmt.Println("Uninstallation cancelled.")
		return nil
	}

	fmt.Println()
	fmt.Println("Uninstalling Stronghold...")

	serviceManager := NewServiceManager(config)

	// Stop the proxy
	if status, _ := serviceManager.IsRunning(); status.Running {
		fmt.Println("  → Stopping proxy...")
		if err := serviceManager.Stop(); err != nil {
			fmt.Printf("    Warning: failed to stop proxy: %v\n", err)
		} else {
			fmt.Println("    ✓ Proxy stopped")
		}
	}

	// Disable transparent proxy
	fmt.Println("  → Disabling transparent proxy...")
	tp := NewTransparentProxy(config)
	if err := tp.Disable(); err != nil {
		fmt.Printf("    Warning: failed to disable transparent proxy: %v\n", err)
	} else {
		fmt.Println("    ✓ Transparent proxy disabled")
	}

	// Uninstall service
	fmt.Println("  → Removing system service...")
	if err := serviceManager.UninstallService(); err != nil {
		fmt.Printf("    Warning: failed to uninstall service: %v\n", err)
	} else {
		fmt.Println("    ✓ Service removed")
	}

	// Remove binaries
	fmt.Println("  → Removing binaries...")
	binaries := []string{
		"/usr/local/bin/stronghold",
		"/usr/local/bin/stronghold-proxy",
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "stronghold"),
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "stronghold-proxy"),
	}
	for _, binary := range binaries {
		if err := os.Remove(binary); err == nil {
			fmt.Printf("    ✓ Removed %s\n", filepath.Base(binary))
		}
	}

	// Handle configuration
	if preserveConfig {
		fmt.Println("  → Preserving configuration at ~/.stronghold/")
		// Just mark as uninstalled
		config.Installed = false
		config.Save()
	} else {
		fmt.Println("  → Removing configuration...")
		configDir := ConfigDir()
		if err := os.RemoveAll(configDir); err != nil {
			fmt.Printf("    Warning: failed to remove config directory: %v\n", err)
		} else {
			fmt.Println("    ✓ Configuration removed")
		}
	}

	fmt.Println()
	fmt.Println("✓ Stronghold has been uninstalled.")

	if config.Auth.LoggedIn && preserveConfig {
		fmt.Println()
		fmt.Println("Your account and wallet balance are preserved.")
		fmt.Println("To delete your account: stronghold account delete")
	}

	return nil
}
