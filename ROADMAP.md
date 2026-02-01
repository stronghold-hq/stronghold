# Production Readiness Roadmap

This document tracks the remaining work needed to launch Stronghold in production.

## Launch Ready ✅

These items have been completed and verified:

### Security
- [x] **HSTS header** - Forces HTTPS, prevents downgrade attacks
- [x] **Security headers** - CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Permissions-Policy
- [x] **JWT_SECRET validation** - Requires ≥32 chars in production, validated at startup
- [x] **CORS validation** - Rejects wildcard origins when credentials enabled
- [x] **Auth event logging** - Logs successful logins, failed attempts, token refreshes
- [x] **Account number validation** - Server-side 16-digit format validation
- [x] **Generic error messages** - Internal errors logged, generic messages to clients
- [x] **Non-root Docker** - Container runs as unprivileged user
- [x] **Stripe webhook replay protection** - Rejects webhooks older than 5 minutes
- [x] **Database query timeouts** - 30-second default timeout on all queries

### Resilience
- [x] **Proxy panic recovery** - HTTPS tunnel goroutines have panic recovery
- [x] **Settlement worker jitter** - Exponential backoff with random jitter prevents thundering herd
- [x] **Fly.io min machines** - At least 1 machine always running (no cold starts)
- [x] **Critical error logging** - DB errors logged instead of silently ignored

### Infrastructure
- [x] **Comprehensive test suite** - Handlers, DB, middleware, integration tests
- [x] **Structured logging** - `log/slog` with JSON in production
- [x] **CI/CD pipeline** - Tests, linting, coverage on every PR
- [x] **API documentation** - Swagger UI at `/docs`
- [x] **Rate limiting** - Per-IP rate limits on auth endpoints

### Features
- [x] **Stripe Crypto Onramp** - Credit card to USDC deposits
- [x] **Dashboard** - Usage stats, billing history, account management
- [x] **x402 atomic payments** - Reserve-commit pattern prevents service without payment

---

## Pre-Launch Checklist

Final items before going live:

- [ ] **Stripe keys configured** - `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PUBLISHABLE_KEY`
- [ ] **Database migrations executed** - Run `001_initial_schema.sql`
- [ ] **CORS origins set** - `DASHBOARD_ALLOWED_ORIGINS` for production domain
- [ ] **SSL/TLS verified** - Fly.io handles this, but verify cert is valid
- [ ] **Secrets audit** - Confirm no secrets in git history
- [ ] **Smoke test** - Manual verification of critical flows (signup, login, scan, deposit)

---

## Post-Launch (Medium Priority)

Nice to have, but not blocking launch:

- [ ] **Database migration tooling** - Proper versioning with golang-migrate
- [ ] **CSRF tokens** - Defense-in-depth for dashboard forms (SameSite cookies provide baseline protection)
- [ ] **Health check coverage** - Add ML model availability, connection pool health
- [ ] **Graceful shutdown** - Add IdleTimeout to Fiber config
- [ ] **External API retry logic** - Exponential backoff for x402 facilitator, Stripe API
- [ ] **Resource leak fixes** - Close log file in proxy, reuse HTTP client in account handler

---

## Future (Low Priority)

For later iterations:

- [ ] **Distributed tracing** - OpenTelemetry integration
- [ ] **Prometheus metrics** - `/metrics` endpoint for monitoring
- [ ] **Database backups** - Automated backup strategy
- [ ] **Secret rotation** - Mechanism for rotating JWT_SECRET, DB credentials
- [ ] **Load testing** - Verify performance under expected traffic

---

## Architecture Notes

Current state:
- **Core scanning**: 4-layer detection (heuristics, ML/Citadel, semantic similarity, LLM)
- **Payment flow**: x402 with atomic settlement (reserve → execute → settle)
- **Database**: PostgreSQL with proper indexes, constraints, and timeouts
- **CLI/Proxy**: Transparent HTTP/HTTPS proxy with panic recovery
- **Docker/Deployment**: Non-root container, health checks, resource limits

The platform is production-ready. Focus is on final configuration and smoke testing.
