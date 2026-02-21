#!/usr/bin/env nu
# Setup script for docs.getstronghold.xyz infrastructure
# Requires: wrangler CLI authenticated with 'wrangler login'

let PROJECT_NAME = "stronghold-docs"
let SUBDOMAIN = "docs"
let DOMAIN = "getstronghold.xyz"
let FULL_DOMAIN = $"($SUBDOMAIN).($DOMAIN)"

print "=== Stronghold Docs Infrastructure Setup ==="
print ""

# Check wrangler is installed
if (which wrangler | is-empty) {
    print "❌ wrangler not found. Install with: bun add -g wrangler"
    exit 1
}

# Check if logged in and get account info
let whoami = try { wrangler whoami | complete } catch { { exit_code: 1, stdout: "" } }
if ($whoami.exit_code != 0) {
    print "❌ Not logged in. Run: wrangler login"
    exit 1
}

print "✅ Wrangler authenticated"

 # Extract account ID from output - strip ANSI codes and parse with regex
let ACCOUNT_ID = ($whoami.stdout | ansi strip | parse -r '([a-f0-9]{32})' | get capture0? | first | default "")

if ($ACCOUNT_ID | is-empty) {
    print "❌ Could not extract account ID from wrangler whoami"
    exit 1
}

print $"✅ Account ID: ($ACCOUNT_ID)"

# Get token for API calls from wrangler config
let token = try {
    open ~/.config/.wrangler/config/default.toml | get oauth_token
} catch {
    try {
        open ~/.wrangler/config/default.toml | get oauth_token
    } catch { "" }
}

if ($token | is-empty) {
    print "❌ Could not get wrangler token from config. Make sure you're authenticated with 'wrangler login'."
    exit 1
}

# List zones via API to find zone ID
print "Finding zone ID..."
let zones_response = try {
    curl -s -X GET "https://api.cloudflare.com/client/v4/zones" -H $"Authorization: Bearer ($token)" -H "Content-Type: application/json" | from json
} catch {
    print "❌ Failed to list zones via API"
    exit 1
}

if ($zones_response.success != true) {
    print $"❌ API error: ($zones_response.errors | default [] | first | get message? | default 'Unknown error')"
    exit 1
}

let zones = ($zones_response.result | default [])
let zone = ($zones | where name == $DOMAIN | first)

if ($zone | is-empty) {
    print $"❌ Could not find zone for ($DOMAIN)"
    print "   Make sure you have access to the zone in Cloudflare"
    exit 1
}

let ZONE_ID = ($zone.id)
print $"✅ Found zone ID: ($ZONE_ID)"
print ""

# Create Pages project
print $"Creating Cloudflare Pages project: ($PROJECT_NAME)..."
let project_check = try {
    curl -s -X GET $"https://api.cloudflare.com/client/v4/accounts/($ACCOUNT_ID)/pages/projects/($PROJECT_NAME)" -H $"Authorization: Bearer ($token)" | from json
} catch { { success: false, result: null } }

let project_exists = ($project_check.success? == true and $project_check.result? != null)

if $project_exists {
    print $"⚠️  Project '($PROJECT_NAME)' already exists"
} else {
    let create_project = try {
        let payload = {name: $PROJECT_NAME, production_branch: "master"} | to json
        curl -s -X POST $"https://api.cloudflare.com/client/v4/accounts/($ACCOUNT_ID)/pages/projects" -H $"Authorization: Bearer ($token)" -H "Content-Type: application/json" -d $payload | from json
    } catch { { success: false } }

    if ($create_project.success? == true) {
        print $"✅ Created project: ($PROJECT_NAME)"
    } else {
        print $"❌ Failed to create project: ($create_project.errors? | default [] | first | get message? | default 'Unknown error')"
    }
}
print ""

# Check if DNS record exists
let dns_response = try {
    curl -s -X GET $"https://api.cloudflare.com/client/v4/zones/($ZONE_ID)/dns_records?type=CNAME&name=($FULL_DOMAIN)" -H $"Authorization: Bearer ($token)" | from json
} catch { { success: false, result: [] } }

let existing_records = ($dns_response.result | default [])

if ($existing_records | length) > 0 {
    print $"⚠️  DNS record already exists for ($FULL_DOMAIN)"
} else {
    # Create DNS record
    let body = {
        type: "CNAME"
        name: $SUBDOMAIN
        content: $"($PROJECT_NAME).pages.dev"
        proxied: true
    }

    let create_dns = try {
        curl -s -X POST $"https://api.cloudflare.com/client/v4/zones/($ZONE_ID)/dns_records" -H $"Authorization: Bearer ($token)" -H "Content-Type: application/json" -d ($body | to json) | from json
    } catch { { success: false } }

    if ($create_dns.success? == true) {
        print $"✅ Created DNS record: ($FULL_DOMAIN) -> ($PROJECT_NAME).pages.dev"
    } else {
        print "⚠️  Failed to create DNS record. Create it manually in Cloudflare Dashboard:"
        print $"   Type: CNAME, Name: ($SUBDOMAIN), Target: ($PROJECT_NAME).pages.dev"
        print $"   Error: ($create_dns.errors? | default [] | first | get message? | default 'Unknown error')"
    }
}
print ""

# Output summary
print "=== Setup Complete ==="
print ""
print $"Project:     https://dash.cloudflare.com/?to=/:account/pages/view/($PROJECT_NAME)"
print $"Domain:      https://($FULL_DOMAIN)"
print ""
print "Next steps:"
print "1. Push to GitHub: git push origin master"
print "2. Add GitHub secrets (if using GitHub Actions):"
print "   - CLOUDFLARE_API_TOKEN"
print "   - CLOUDFLARE_ACCOUNT_ID"
print ""
print "Or connect via Git integration in Cloudflare Dashboard:"
print "   Build command: cd docs-site && bun install && bun run build"
print "   Build output:  docs-site/dist"
