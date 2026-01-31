package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InstallState represents the current state of the installation
type InstallState int

const (
	StateWarning InstallState = iota
	StateChecking
	StateAuth
	StatePayment
	StateConfig
	StateInstalling
	StateComplete
	StateError
)

// InstallModel is the Bubble Tea model for the install command
type InstallModel struct {
	state       InstallState
	config      *CLIConfig
	width       int
	height      int

	// Warning step
	confirmWarning bool

	// Auth step
	authChoice    int // 0 = login, 1 = signup, 2 = skip
	emailInput    textinput.Model
	passwordInput textinput.Model
	authToken     string
	loggedIn      bool
	userEmail     string

	// Payment step
	paymentMethod int // 0 = stripe, 1 = wallet

	// Config step
	portInput   textinput.Model
	apiInput    textinput.Model
	configPort  int
	configAPI   string

	// Progress
	progress    []string
	currentStep int
	errorMsg    string
}

// Styles
var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00D4AA")).
		MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF"))

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00D4AA"))

	warningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFA500"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF4444"))

	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00D4AA")).
		Bold(true)

	unselectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))
)

// NewInstallModel creates a new install model
func NewInstallModel() *InstallModel {
	emailInput := textinput.New()
	emailInput.Placeholder = "user@example.com"
	emailInput.Focus()
	emailInput.CharLimit = 100
	emailInput.Width = 40

	passwordInput := textinput.New()
	passwordInput.Placeholder = "password"
	passwordInput.EchoMode = textinput.EchoPassword
	passwordInput.CharLimit = 100
	passwordInput.Width = 40

	portInput := textinput.New()
	portInput.Placeholder = "8080"
	portInput.CharLimit = 5
	portInput.Width = 10

	apiInput := textinput.New()
	apiInput.Placeholder = "https://api.stronghold.security"
	apiInput.CharLimit = 100
	apiInput.Width = 50

	return &InstallModel{
		state:         StateWarning,
		config:        DefaultConfig(),
		emailInput:    emailInput,
		passwordInput: passwordInput,
		portInput:     portInput,
		apiInput:      apiInput,
		configPort:    8080,
		configAPI:     "https://api.stronghold.security",
		progress:      []string{},
	}
}

// Init initializes the model
func (m *InstallModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *InstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "y", "Y":
			if m.state == StateWarning {
				m.confirmWarning = true
				m.state = StateChecking
				return m, m.runChecks()
			}

		case "n", "N":
			if m.state == StateWarning {
				return m, tea.Quit
			}

		case "up", "k":
			m.handleUp()

		case "down", "j":
			m.handleDown()

		case "tab":
			m.handleTab()
		}
	}

	// Update text inputs
	var cmd tea.Cmd
	switch m.state {
	case StateAuth:
		if m.emailInput.Focused() {
			m.emailInput, cmd = m.emailInput.Update(msg)
		} else {
			m.passwordInput, cmd = m.passwordInput.Update(msg)
		}
	case StateConfig:
		if m.portInput.Focused() {
			m.portInput, cmd = m.portInput.Update(msg)
		} else {
			m.apiInput, cmd = m.apiInput.Update(msg)
		}
	}

	return m, cmd
}

// View renders the UI
func (m *InstallModel) View() string {
	switch m.state {
	case StateWarning:
		return m.viewWarning()
	case StateChecking:
		return m.viewChecking()
	case StateAuth:
		return m.viewAuth()
	case StatePayment:
		return m.viewPayment()
	case StateConfig:
		return m.viewConfig()
	case StateInstalling:
		return m.viewInstalling()
	case StateComplete:
		return m.viewComplete()
	case StateError:
		return m.viewError()
	default:
		return ""
	}
}

// handleEnter processes the enter key
func (m *InstallModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateChecking:
		m.state = StateAuth
		return m, nil

	case StateAuth:
		if m.authChoice == 0 { // Login
			if m.emailInput.Value() != "" && m.passwordInput.Value() != "" {
				// Simulate login
				m.userEmail = m.emailInput.Value()
				m.loggedIn = true
				m.authToken = "simulated-jwt-token"
				m.config.Auth.Token = m.authToken
				m.config.Auth.Email = m.userEmail
				m.config.Auth.LoggedIn = true
				m.state = StatePayment
			}
		} else if m.authChoice == 1 { // Signup
			m.userEmail = m.emailInput.Value()
			m.loggedIn = true
			m.authToken = "simulated-jwt-token"
			m.config.Auth.Token = m.authToken
			m.config.Auth.Email = m.userEmail
			m.config.Auth.LoggedIn = true
			m.state = StatePayment
		} else { // Skip
			m.state = StatePayment
		}

	case StatePayment:
		if m.paymentMethod == 0 {
			m.config.Payments.Method = "stripe"
		} else {
			m.config.Payments.Method = "wallet"
		}
		m.state = StateConfig
		m.portInput.Focus()

	case StateConfig:
		// Parse port
		if m.portInput.Value() != "" {
			fmt.Sscanf(m.portInput.Value(), "%d", &m.configPort)
		}
		m.config.Proxy.Port = m.configPort

		if m.apiInput.Value() != "" {
			m.configAPI = m.apiInput.Value()
		}
		m.config.API.Endpoint = m.configAPI

		m.state = StateInstalling
		return m, m.runInstallation()

	case StateComplete:
		return m, tea.Quit

	case StateError:
		return m, tea.Quit
	}

	return m, nil
}

// handleUp handles up arrow
func (m *InstallModel) handleUp() {
	switch m.state {
	case StateAuth:
		if m.authChoice > 0 {
			m.authChoice--
		}
	case StatePayment:
		if m.paymentMethod > 0 {
			m.paymentMethod--
		}
	}
}

// handleDown handles down arrow
func (m *InstallModel) handleDown() {
	switch m.state {
	case StateAuth:
		if m.authChoice < 2 {
			m.authChoice++
		}
	case StatePayment:
		if m.paymentMethod < 1 {
			m.paymentMethod++
		}
	}
}

// handleTab handles tab key
func (m *InstallModel) handleTab() {
	switch m.state {
	case StateAuth:
		if m.emailInput.Focused() {
			m.emailInput.Blur()
			m.passwordInput.Focus()
		} else {
			m.passwordInput.Blur()
			m.emailInput.Focus()
		}
	case StateConfig:
		if m.portInput.Focused() {
			m.portInput.Blur()
			m.apiInput.Focus()
		} else {
			m.apiInput.Blur()
			m.portInput.Focus()
		}
	}
}

// viewWarning renders the warning screen
func (m *InstallModel) viewWarning() string {
	var b strings.Builder

	b.WriteString(warningStyle.Render("⚠️  WARNING"))
	b.WriteString("\n\n")
	b.WriteString("Stronghold sets up a system-wide proxy.\n")
	b.WriteString("This will route ALL system traffic through our scanning service.\n")
	b.WriteString("This is intended for isolated machines running AI agents.\n")
	b.WriteString(warningStyle.Render("Not recommended for daily-use workstations."))
	b.WriteString("\n\n")
	b.WriteString("Continue? [y/N]: ")

	return b.String()
}

// viewChecking renders the system check screen
func (m *InstallModel) viewChecking() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Checking system..."))
	b.WriteString("\n\n")

	for _, step := range m.progress {
		b.WriteString(step)
		b.WriteString("\n")
	}

	if m.currentStep >= 2 {
		b.WriteString("\n")
		b.WriteString(infoStyle.Render("Press Enter to continue..."))
	}

	return b.String()
}

// viewAuth renders the authentication screen
func (m *InstallModel) viewAuth() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Authentication"))
	b.WriteString("\n\n")

	// Auth choices
	choices := []string{"Login", "Sign Up", "Skip (limited functionality)"}
	for i, choice := range choices {
		if i == m.authChoice {
			b.WriteString(selectedStyle.Render(fmt.Sprintf("▸ %s", choice)))
		} else {
			b.WriteString(unselectedStyle.Render(fmt.Sprintf("  %s", choice)))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if m.authChoice < 2 {
		b.WriteString("Email:\n")
		b.WriteString(m.emailInput.View())
		b.WriteString("\n\n")
		b.WriteString("Password:\n")
		b.WriteString(m.passwordInput.View())
	}

	b.WriteString("\n\n")
	b.WriteString(infoStyle.Render("Use Tab to switch fields, Enter to continue"))

	return b.String()
}

// viewPayment renders the payment setup screen
func (m *InstallModel) viewPayment() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Payment Setup"))
	b.WriteString("\n\n")
	b.WriteString("Choose payment method:\n\n")

	methods := []struct {
		name        string
		description string
	}{
		{"Credit Card (Stripe)", "Recommended - Easy setup, auto top-up"},
		{"Crypto Wallet (x402 direct)", "Connect your own wallet"},
	}

	for i, method := range methods {
		if i == m.paymentMethod {
			b.WriteString(selectedStyle.Render(fmt.Sprintf("▸ [%d] %s", i+1, method.name)))
		} else {
			b.WriteString(unselectedStyle.Render(fmt.Sprintf("  [%d] %s", i+1, method.name)))
		}
		b.WriteString("\n")
		b.WriteString(infoStyle.Render(fmt.Sprintf("    %s", method.description)))
		b.WriteString("\n\n")
	}

	b.WriteString(infoStyle.Render("Press Enter to continue"))

	return b.String()
}

// viewConfig renders the configuration screen
func (m *InstallModel) viewConfig() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Configuration"))
	b.WriteString("\n\n")

	b.WriteString("Proxy port [8080]:\n")
	b.WriteString(m.portInput.View())
	b.WriteString("\n\n")

	b.WriteString("Stronghold API [https://api.stronghold.security]:\n")
	b.WriteString(m.apiInput.View())
	b.WriteString("\n\n")

	b.WriteString(infoStyle.Render("Use Tab to switch fields, Enter to continue"))

	return b.String()
}

// viewInstalling renders the installation progress screen
func (m *InstallModel) viewInstalling() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Installing..."))
	b.WriteString("\n\n")

	for _, step := range m.progress {
		b.WriteString(step)
		b.WriteString("\n")
	}

	return b.String()
}

// viewComplete renders the completion screen
func (m *InstallModel) viewComplete() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("✓ Installation complete!"))
	b.WriteString("\n\n")
	b.WriteString("Your system is now protected. All HTTP/HTTPS traffic is being\n")
	b.WriteString("intercepted at the network level and scanned for prompt injection\n")
	b.WriteString("attacks before reaching your agents.\n\n")
	b.WriteString("This cannot be bypassed by applications.\n\n")

	b.WriteString(headerStyle.Render("Quick Commands:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Status:    stronghold status\n"))
	b.WriteString(fmt.Sprintf("  Disable:   stronghold disable\n"))
	b.WriteString(fmt.Sprintf("  Dashboard: https://dashboard.stronghold.security\n"))
	b.WriteString(fmt.Sprintf("  Usage:     ~$0.001 per scanned request\n\n"))

	b.WriteString(infoStyle.Render("Press Enter to exit"))

	return b.String()
}

// viewError renders the error screen
func (m *InstallModel) viewError() string {
	var b strings.Builder

	b.WriteString(errorStyle.Render("✗ Installation failed"))
	b.WriteString("\n\n")
	b.WriteString(m.errorMsg)
	b.WriteString("\n\n")
	b.WriteString(infoStyle.Render("Press Enter to exit"))

	return b.String()
}

// runChecks runs system checks
func (m *InstallModel) runChecks() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(500 * time.Millisecond)

		// Check platform
		if !IsSupportedPlatform() {
			m.progress = append(m.progress, errorStyle.Render("✗ "+runtime.GOOS+" is not supported (Linux/macOS only)"))
			m.state = StateError
			m.errorMsg = "Unsupported operating system"
			return nil
		}
		m.progress = append(m.progress, successStyle.Render("✓ "+runtime.GOOS+" detected"))

		time.Sleep(300 * time.Millisecond)

		// Check port availability
		if !m.config.IsPortAvailable() {
			// Try to find an available port
			newPort := FindAvailablePort(m.config.Proxy.Port + 1)
			if newPort == 0 {
				m.progress = append(m.progress, errorStyle.Render("✗ No available ports found"))
				m.state = StateError
				m.errorMsg = "Could not find an available port"
				return nil
			}
			m.config.Proxy.Port = newPort
			m.progress = append(m.progress, warningStyle.Render(fmt.Sprintf("⚠ Port 8080 in use, using port %d", newPort)))
		} else {
			m.progress = append(m.progress, successStyle.Render(fmt.Sprintf("✓ Port %d available", m.config.Proxy.Port)))
		}

		m.currentStep = 2
		return nil
	}
}

// runInstallation performs the installation
func (m *InstallModel) runInstallation() tea.Cmd {
	return func() tea.Msg {
		steps := []struct {
			name string
			fn   func() error
		}{
			{"Saving configuration", m.saveConfig},
			{"Installing proxy binary", m.installProxyBinary},
			{"Installing CLI binary", m.installCLIBinary},
			{"Configuring system service", m.configureService},
			{"Starting proxy", m.startProxy},
			{"Enabling transparent proxy", m.enableTransparentProxy},
		}

		for _, step := range steps {
			m.progress = append(m.progress, fmt.Sprintf("  → %s...", step.name))
			if err := step.fn(); err != nil {
				m.progress = append(m.progress, errorStyle.Render(fmt.Sprintf("    ✗ %s", err.Error())))
				m.state = StateError
				m.errorMsg = err.Error()
				return nil
			}
			// Replace the "→" with "✓"
			m.progress[len(m.progress)-1] = successStyle.Render(fmt.Sprintf("    ✓ %s", step.name))
			time.Sleep(200 * time.Millisecond)
		}

		m.config.Installed = true
		m.config.InstallDate = time.Now().Format(time.RFC3339)
		m.config.Save()

		m.state = StateComplete
		return nil
	}
}

// saveConfig saves the configuration
func (m *InstallModel) saveConfig() error {
	return m.config.Save()
}

// installProxyBinary installs the proxy binary
func (m *InstallModel) installProxyBinary() error {
	// In a real implementation, this would download or copy the binary
	// For now, we'll just check if it exists or create a placeholder

	destPath := "/usr/local/bin/stronghold-proxy"

	// Check if we're running from source
	if _, err := os.Stat("./cmd/proxy/main.go"); err == nil {
		// Build the proxy
		cmd := exec.Command("go", "build", "-o", destPath, "./cmd/proxy")
		if _, err := cmd.CombinedOutput(); err != nil {
			// Try user-local bin
			userBin := filepath.Join(os.Getenv("HOME"), ".local", "bin")
			os.MkdirAll(userBin, 0755)
			destPath = filepath.Join(userBin, "stronghold-proxy")
			cmd = exec.Command("go", "build", "-o", destPath, "./cmd/proxy")
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to build proxy: %s", string(output))
			}
		}
	} else {
		// Check if binary already exists
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			// Create a placeholder script for development
			return fmt.Errorf("proxy binary not found - please build from source")
		}
	}

	return nil
}

// installCLIBinary installs the CLI binary
func (m *InstallModel) installCLIBinary() error {
	// Similar to proxy binary
	destPath := "/usr/local/bin/stronghold"

	if _, err := os.Stat("./cmd/cli/main.go"); err == nil {
		cmd := exec.Command("go", "build", "-o", destPath, "./cmd/cli")
		if _, err := cmd.CombinedOutput(); err != nil {
			userBin := filepath.Join(os.Getenv("HOME"), ".local", "bin")
			os.MkdirAll(userBin, 0755)
			destPath = filepath.Join(userBin, "stronghold")
			cmd = exec.Command("go", "build", "-o", destPath, "./cmd/cli")
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to build CLI: %s", string(output))
			}
		}
	}

	return nil
}

// configureService configures the system service
func (m *InstallModel) configureService() error {
	serviceManager := NewServiceManager(m.config)
	return serviceManager.InstallService()
}

// startProxy starts the proxy
func (m *InstallModel) startProxy() error {
	serviceManager := NewServiceManager(m.config)
	return serviceManager.Start()
}

// enableTransparentProxy enables transparent proxying
func (m *InstallModel) enableTransparentProxy() error {
	tp := NewTransparentProxy(m.config)
	if !tp.IsAvailable() {
		return fmt.Errorf("transparent proxy not available on this system")
	}
	return tp.Enable()
}

// RunInstall runs the interactive installer
func RunInstall() error {
	model := NewInstallModel()
	p := tea.NewProgram(model)
	_, err := p.Run()
	return err
}

// RunInstallNonInteractive runs a non-interactive installation
func RunInstallNonInteractive() error {
	config := DefaultConfig()

	// Check platform
	if !IsSupportedPlatform() {
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Check port
	if !config.IsPortAvailable() {
		newPort := FindAvailablePort(config.Proxy.Port + 1)
		if newPort == 0 {
			return fmt.Errorf("no available ports found")
		}
		config.Proxy.Port = newPort
		fmt.Printf("Port 8080 in use, using port %d\n", newPort)
	}

	// Save config
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Install binaries
	destPath := "/usr/local/bin/stronghold-proxy"
	if _, err := os.Stat("./cmd/proxy/main.go"); err == nil {
		cmd := exec.Command("go", "build", "-o", destPath, "./cmd/proxy")
		if _, err := cmd.CombinedOutput(); err != nil {
			userBin := filepath.Join(os.Getenv("HOME"), ".local", "bin")
			os.MkdirAll(userBin, 0755)
			destPath = filepath.Join(userBin, "stronghold-proxy")
			cmd = exec.Command("go", "build", "-o", destPath, "./cmd/proxy")
			if _, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to build proxy: %w", err)
			}
		}
	}

	// Install service
	serviceManager := NewServiceManager(config)
	if err := serviceManager.InstallService(); err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	// Start proxy
	if err := serviceManager.Start(); err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
	}

	// Enable transparent proxy
	tp := NewTransparentProxy(config)
	if !tp.IsAvailable() {
		return fmt.Errorf("transparent proxy not available on this system")
	}
	if err := tp.Enable(); err != nil {
		return fmt.Errorf("failed to enable transparent proxy: %w", err)
	}

	config.Installed = true
	config.InstallDate = time.Now().Format(time.RFC3339)
	config.Save()

	fmt.Println("✓ Installation complete!")
	fmt.Printf("Proxy running on %s (transparent mode)\n", config.GetProxyAddr())

	return nil
}

// Confirm prompts the user for confirmation
func Confirm(prompt string) bool {
	fmt.Print(prompt + " ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
