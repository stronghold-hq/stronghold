#!/bin/bash
# Stronghold Base Sepolia Testnet Setup
#
# This script helps you set up a test environment with Base Sepolia testnet.
#
# Prerequisites:
#   1. Docker and Docker Compose installed
#   2. A wallet with Base Sepolia ETH and USDC
#
# Getting testnet tokens:
#   1. Base Sepolia ETH (for gas):
#      - Alchemy Faucet: https://www.alchemy.com/faucets/base-sepolia
#      - Coinbase Faucet: https://www.coinbase.com/faucets/base-ethereum-goerli-faucet
#      - QuickNode: https://faucet.quicknode.com/base/sepolia
#
#   2. Base Sepolia USDC (for payments):
#      - Circle Faucet: https://faucet.circle.com/ (select Base Sepolia)
#      - USDC Contract: 0x036CbD53842c5426634e7929541eC2318f3dCF7e
#
# Usage:
#   export TEST_PRIVATE_KEY=0x...  # Your funded wallet's private key
#   export API_WALLET_ADDRESS=0x... # Wallet to receive payments (can be same)
#   ./scripts/testnet-setup.sh

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     Stronghold Base Sepolia Testnet Setup                  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Check prerequisites
echo -e "${YELLOW}Checking prerequisites...${NC}"

if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: Docker is not installed${NC}"
    exit 1
fi

if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo -e "${RED}Error: Docker Compose is not installed${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Docker installed${NC}"

# Check for private key
if [ -z "$TEST_PRIVATE_KEY" ]; then
    echo ""
    echo -e "${YELLOW}No TEST_PRIVATE_KEY environment variable set.${NC}"
    echo ""
    echo "To test with x402 payments, you need a funded wallet on Base Sepolia."
    echo ""
    echo -e "${BLUE}Getting testnet tokens:${NC}"
    echo ""
    echo "1. Create a wallet (MetaMask, etc.) and get the private key"
    echo ""
    echo "2. Get Base Sepolia ETH for gas:"
    echo "   - Alchemy: https://www.alchemy.com/faucets/base-sepolia"
    echo "   - QuickNode: https://faucet.quicknode.com/base/sepolia"
    echo ""
    echo "3. Get Base Sepolia USDC for payments:"
    echo "   - Circle: https://faucet.circle.com/ (select Base Sepolia)"
    echo "   - Contract: 0x036CbD53842c5426634e7929541eC2318f3dCF7e"
    echo ""
    echo "4. Export your private key and run again:"
    echo "   export TEST_PRIVATE_KEY=0x..."
    echo "   export API_WALLET_ADDRESS=0x...  # address to receive payments"
    echo "   ./scripts/testnet-setup.sh"
    echo ""
    read -p "Continue without a funded wallet? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 0
    fi
fi

# Set default API wallet address if not provided
if [ -z "$API_WALLET_ADDRESS" ]; then
    # Use a dummy address - payments will fail but we can test the flow
    export API_WALLET_ADDRESS="0x0000000000000000000000000000000000000001"
    echo -e "${YELLOW}Warning: API_WALLET_ADDRESS not set, using dummy address${NC}"
    echo "Payments will be required but may fail verification."
fi

echo ""
echo -e "${BLUE}Starting testnet environment...${NC}"
echo ""

# Build and start services
docker-compose -f docker-compose.testnet.yml build
docker-compose -f docker-compose.testnet.yml up -d postgres api

echo ""
echo -e "${YELLOW}Waiting for API to be ready...${NC}"
sleep 5

# Check if API is responding
for i in {1..30}; do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓ API is ready${NC}"
        break
    fi
    if [ $i -eq 30 ]; then
        echo -e "${RED}Error: API failed to start${NC}"
        docker-compose -f docker-compose.testnet.yml logs api
        exit 1
    fi
    sleep 1
done

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║     Testnet Environment Ready!                             ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "API URL:  http://localhost:8080"
echo "Network:  base-sepolia"
echo ""
echo -e "${BLUE}To enter the CLI test container:${NC}"
echo "  docker-compose -f docker-compose.testnet.yml run --rm cli"
echo ""
echo -e "${BLUE}Inside the container:${NC}"
if [ -n "$TEST_PRIVATE_KEY" ]; then
    echo "  stronghold init --yes --private-key \$TEST_PRIVATE_KEY"
else
    echo "  stronghold init --yes  # Creates new wallet (won't have funds)"
fi
echo "  stronghold enable"
echo "  stronghold status"
echo "  curl -x http://localhost:8402 http://httpbin.org/html"
echo ""
echo -e "${BLUE}To stop the environment:${NC}"
echo "  docker-compose -f docker-compose.testnet.yml down"
echo ""
echo -e "${BLUE}To view logs:${NC}"
echo "  docker-compose -f docker-compose.testnet.yml logs -f api"
echo ""
