# Repository Guidelines

## Project Structure & Module Organization
Stronghold is split across Go services and a Next.js dashboard.

- `cmd/`: executable entry points (`api`, `cli`, `proxy`, `x402test`).
- `internal/`: core Go packages (`handlers`, `middleware`, `db`, `wallet`, `proxy`, `cli`, `server`).
- `internal/db/migrations/`: SQL migrations (apply incrementally; do not rewrite old migrations).
- `web/`: Next.js app, UI components, and Vitest tests.
- `scripts/`: local/e2e helpers such as `scripts/e2e-x402-test.sh`.
- Root deployment files: `docker-compose*.yml`, `Dockerfile`, `facilitator/`.

## Build, Test, and Development Commands
- `go build -o stronghold ./cmd/cli`: build CLI.
- `go build -o stronghold-api ./cmd/api`: build API server.
- `go build -o stronghold-proxy ./cmd/proxy`: build proxy daemon.
- `go test ./...`: run Go test suite.
- `go test -race -coverprofile=coverage.out -covermode=atomic ./...`: CI-equivalent Go verification.
- `docker-compose up -d`: start local API stack dependencies.
- `cd web && bun install`: install frontend dependencies.
- `cd web && bun run dev|build|lint|test:run`: run dashboard locally, build, lint, and test.

## Coding Style & Naming Conventions
Use Go defaults: tabs, `gofmt` formatting, and idiomatic package structure. Keep package names lowercase and exported identifiers in `PascalCase`. For TypeScript/React, use `PascalCase` for components (`web/components/...`) and `useX` naming for hooks (`web/lib/hooks/...`). Prefer explicit chain-specific wallet fields (`evm_wallet_address`, `solana_wallet_address`) instead of ambiguous generic names.

## Testing Guidelines
Go tests use the standard `testing` package plus `testify` assertions. DB-heavy tests use `testcontainers-go`, so ensure Docker is running. Add regression tests with every behavior fix (handlers, CLI parsing/help text, health checks). Naming conventions: Go `*_test.go` with `TestXxx`; frontend `*.test.ts(x)` under `web/__tests__/`.

## Commit & Pull Request Guidelines
Follow concise, imperative commit subjects, e.g. `Add automated wallet and health regression tests`, `Fix x402 facilitator request format`. Keep one logical change per commit. PRs should include: summary, risk/migration notes (especially DB/config/API contract changes), and exact test commands run. If CLI behavior/help changes, also update `README.md`, `web/public/llms.txt`, and `web/public/llms-full.txt`.

# Agent Commit Policy

- Never create a git commit unless the user explicitly asks to commit in the current conversation state.
- Default behavior is to leave all code changes uncommitted for user review.
- Never push commits unless the user explicitly asks to push.
