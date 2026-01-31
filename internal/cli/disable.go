package cli

import (
	"fmt"
)

// Disable disables the Stronghold proxy
func Disable() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !config.Installed {
		return fmt.Errorf("stronghold is not installed")
	}

	serviceManager := NewServiceManager(config)

	// Check current status
	status, err := serviceManager.IsRunning()
	if err != nil {
		return fmt.Errorf("failed to check status: %w", err)
	}

	if !status.Running {
		fmt.Println("Stronghold proxy is not running")
		return nil
	}

	fmt.Println("Disabling Stronghold proxy...")

	// Disable transparent proxy
	tp := NewTransparentProxy(config)
	if err := tp.Disable(); err != nil {
		fmt.Printf("Warning: failed to disable transparent proxy: %v\n", err)
	} else {
		fmt.Println("✓ Transparent proxy disabled")
	}

	// Stop the proxy
	if err := serviceManager.Stop(); err != nil {
		return fmt.Errorf("failed to stop proxy: %w", err)
	}
	fmt.Println("✓ Stronghold proxy stopped")

	fmt.Println()
	fmt.Println("Direct internet access restored.")
	fmt.Println("Agents are no longer protected. Run 'stronghold enable' to restore protection.")

	return nil
}
