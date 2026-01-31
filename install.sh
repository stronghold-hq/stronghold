#!/bin/bash
#
# Stronghold CLI Installer
# Usage: curl -fsSL https://install.stronghold.security | sh
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO_URL="https://github.com/strongholdsecurity/stronghold"
INSTALL_DIR="/usr/local/bin"
VERSION="${VERSION:-latest}"

# Print functions
print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)
            PLATFORM="linux"
            ;;
        darwin)
            PLATFORM="darwin"
            ;;
        *)
            print_error "Unsupported operating system: $OS"
            print_info "Stronghold supports Linux and macOS only."
            exit 1
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    print_success "Detected platform: $PLATFORM/$ARCH"
}

# Check dependencies
check_dependencies() {
    print_info "Checking dependencies..."

    if ! command -v curl &> /dev/null; then
        print_error "curl is required but not installed"
        exit 1
    fi

    print_success "All dependencies satisfied"
}

# Download and install
download_and_install() {
    print_info "Downloading Stronghold..."

    # Determine download URL
    if [ "$VERSION" = "latest" ]; then
        DOWNLOAD_URL="$REPO_URL/releases/latest/download/stronghold-$PLATFORM-$ARCH.tar.gz"
    else
        DOWNLOAD_URL="$REPO_URL/releases/download/$VERSION/stronghold-$PLATFORM-$ARCH.tar.gz"
    fi

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    # Download
    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/stronghold.tar.gz"; then
        print_error "Failed to download Stronghold"
        print_info "If you're building from source, run: go build ./cmd/cli && go build ./cmd/proxy"
        exit 1
    fi

    # Extract
    tar -xzf "$TMP_DIR/stronghold.tar.gz" -C "$TMP_DIR"

    # Install binaries
    print_info "Installing binaries..."

    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        SUDO=""
    else
        print_warning "Need sudo access to install to $INSTALL_DIR"
        SUDO="sudo"
    fi

    # Install CLI
    $SUDO install -m 755 "$TMP_DIR/stronghold" "$INSTALL_DIR/stronghold"
    print_success "Installed stronghold CLI to $INSTALL_DIR/stronghold"

    # Install proxy
    $SUDO install -m 755 "$TMP_DIR/stronghold-proxy" "$INSTALL_DIR/stronghold-proxy"
    print_success "Installed stronghold-proxy to $INSTALL_DIR/stronghold-proxy"
}

# Run post-install setup
post_install() {
    print_info "Running post-install setup..."

    # Add to PATH if needed
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        print_warning "$INSTALL_DIR is not in your PATH"
        print_info "Add the following to your shell profile:"
        echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    fi

    print_success "Installation complete!"
    echo
    echo "To get started, run:"
    echo "  stronghold install"
    echo
    echo "For help:"
    echo "  stronghold --help"
}

# Build from source if in a git repo
build_from_source() {
    print_info "Building from source..."

    if ! command -v go &> /dev/null; then
        print_error "Go is required to build from source"
        exit 1
    fi

    # Check if we're in the stronghold repo
    if [ ! -f "go.mod" ] || ! grep -q "module stronghold" go.mod 2>/dev/null; then
        print_error "Not in the stronghold repository"
        exit 1
    fi

    # Build CLI
    go build -o "$TMP_DIR/stronghold" ./cmd/cli
    print_success "Built stronghold CLI"

    # Build proxy
    go build -o "$TMP_DIR/stronghold-proxy" ./cmd/proxy
    print_success "Built stronghold-proxy"

    # Install
    if [ -w "$INSTALL_DIR" ]; then
        SUDO=""
    else
        SUDO="sudo"
    fi

    $SUDO install -m 755 "$TMP_DIR/stronghold" "$INSTALL_DIR/stronghold"
    $SUDO install -m 755 "$TMP_DIR/stronghold-proxy" "$INSTALL_DIR/stronghold-proxy"
}

# Main installation flow
main() {
    echo
    echo "╔══════════════════════════════════════════╗"
    echo "║       Stronghold Installer               ║"
    echo "║   AI Security for LLM Agents             ║"
    echo "╚══════════════════════════════════════════╝"
    echo

    detect_platform
    check_dependencies

    # Check if we should build from source
    if [ "${BUILD_FROM_SOURCE:-false}" = "true" ] || [ -f "go.mod" ] && grep -q "module stronghold" go.mod 2>/dev/null; then
        TMP_DIR=$(mktemp -d)
        trap "rm -rf $TMP_DIR" EXIT
        build_from_source
    else
        download_and_install
    fi

    post_install
}

# Run main function
main "$@"
