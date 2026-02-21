#!/usr/bin/env bash
# Setup script for docs.getstronghold.xyz infrastructure
# Requires: wrangler CLI authenticated with 'wrangler login'

set -e

PROJECT_NAME="stronghold-docs"
SUBDOMAIN="docs"
DOMAIN="getstronghold.xyz"
FULL_DOMAIN="${SUBDOMAIN}.${DOMAIN}"

echo "=== Stronghold Docs Infrastructure Setup ==="
echo ""

# Check wrangler is installed and logged in
if ! command -v wrangler &> /dev/null; then
    echo "❌ wrangler not found. Install with: bun add -g wrangler"
    exit 1
fi

# Check if logged in
if ! wrangler whoami &> /dev/null; then
    echo "❌ Not logged in. Run: wrangler login"
    exit 1
fi

echo "✅ Wrangler authenticated"
echo ""

# Get zone ID
ZONE_ID=$(wrangler zone list 2>/dev/null | grep "$DOMAIN" | awk '{print $1}' || echo "")

if [ -z "$ZONE_ID" ]; then
    echo "❌ Could not find zone ID for $DOMAIN"
    echo "   Make sure you have access to the zone in Cloudflare"
    exit 1
fi

echo "✅ Found zone ID: $ZONE_ID"
echo ""

# Create Pages project
echo "Creating Cloudflare Pages project: $PROJECT_NAME..."
if wrangler pages project list 2>/dev/null | grep -q "$PROJECT_NAME"; then
    echo "⚠️  Project '$PROJECT_NAME' already exists"
else
    wrangler pages project create "$PROJECT_NAME"
    echo "✅ Created project: $PROJECT_NAME"
fi
echo ""

# Add DNS record
echo "Creating DNS record: $FULL_DOMAIN -> ${PROJECT_NAME}.pages.dev..."

# Check if record exists
EXISTING_RECORD=$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/dns_records?type=CNAME&name=${FULL_DOMAIN}" \
    -H "Authorization: Bearer $(wrangler token)" 2>/dev/null | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")

if [ -n "$EXISTING_RECORD" ]; then
    echo "⚠️  DNS record already exists for $FULL_DOMAIN"
else
    curl -s -X POST "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/dns_records" \
        -H "Authorization: Bearer $(wrangler token)" \
        -H "Content-Type: application/json" \
        --data "{\"type\":\"CNAME\",\"name\":\"${SUBDOMAIN}\",\"content\":\"${PROJECT_NAME}.pages.dev\",\"proxied\":true}" > /dev/null
    echo "✅ Created DNS record: $FULL_DOMAIN -> ${PROJECT_NAME}.pages.dev"
fi
echo ""

# Output summary
echo "=== Setup Complete ==="
echo ""
echo "Project:     https://dash.cloudflare.com/?to=/:account/pages/view/$PROJECT_NAME"
echo "Domain:      https://$FULL_DOMAIN"
echo ""
echo "Next steps:"
echo "1. Push to GitHub: git push origin master"
echo "2. Add GitHub secrets (if using GitHub Actions):"
echo "   - CLOUDFLARE_API_TOKEN"
echo "   - CLOUDFLARE_ACCOUNT_ID"
echo ""
echo "Or connect via Git integration in Cloudflare Dashboard:"
echo "   Build command: cd docs-site && bun install && bun run build"
echo "   Build output:  docs-site/dist"
