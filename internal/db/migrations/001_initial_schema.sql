-- Initial schema for Stronghold Mullvad-style authentication
-- Migration: 001_initial_schema

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Account status enum
CREATE TYPE account_status AS ENUM ('active', 'suspended', 'closed');

-- Deposit status enum
CREATE TYPE deposit_status AS ENUM ('pending', 'completed', 'failed', 'cancelled');

-- Deposit provider enum
CREATE TYPE deposit_provider AS ENUM ('stripe', 'direct');

-- Payment status enum
CREATE TYPE payment_status AS ENUM (
    'reserved',
    'executing',
    'settling',
    'completed',
    'failed',
    'expired'
);

-- Accounts table - core account entity
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_number VARCHAR(19) UNIQUE NOT NULL, -- formatted with dashes: XXXX-XXXX-XXXX-XXXX
    wallet_address VARCHAR(64), -- Wallet address for x402 (EVM 0x... or Solana base58)
    balance_usdc DECIMAL(20,6) NOT NULL DEFAULT 0.000000,
    status account_status NOT NULL DEFAULT 'active',
    wallet_escrow_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    totp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    totp_secret_encrypted TEXT,
    encrypted_private_key TEXT,
    kms_key_id VARCHAR(255),
    key_encrypted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}'::jsonb,

    CONSTRAINT valid_account_number CHECK (account_number ~ '^[0-9]{4}-[0-9]{4}-[0-9]{4}-[0-9]{4}$'),
    CONSTRAINT valid_wallet_address CHECK (wallet_address IS NULL OR wallet_address ~ '^0x[a-fA-F0-9]{40}$' OR wallet_address ~ '^[1-9A-HJ-NP-Za-km-z]{32,44}$'),
    CONSTRAINT accounts_balance_non_negative CHECK (balance_usdc >= 0)
);

-- Create indexes for account lookups (UNIQUE on account_number already creates an implicit index)
CREATE UNIQUE INDEX accounts_wallet_address_unique ON accounts(wallet_address) WHERE wallet_address IS NOT NULL;
CREATE INDEX idx_accounts_status ON accounts(status);
CREATE INDEX idx_accounts_has_encrypted_key
ON accounts((encrypted_private_key IS NOT NULL));

-- Trusted devices for per-device TOTP trust
CREATE TABLE account_devices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    device_token_hash VARCHAR(64) NOT NULL,
    label TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX idx_account_devices_token_hash ON account_devices(device_token_hash);
CREATE INDEX idx_account_devices_account_id ON account_devices(account_id);
CREATE INDEX idx_account_devices_expires_at ON account_devices(expires_at);

-- Recovery codes for TOTP fallback
CREATE TABLE totp_recovery_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    code_hash VARCHAR(64) NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX idx_totp_recovery_code_hash ON totp_recovery_codes(code_hash);
CREATE INDEX idx_totp_recovery_account_id ON totp_recovery_codes(account_id);

-- Sessions table - JWT refresh token storage
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(64) NOT NULL, -- SHA-256 hash
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMP WITH TIME ZONE,
    ip_address INET,
    user_agent TEXT,

    CONSTRAINT valid_token_hash CHECK (LENGTH(refresh_token_hash) = 64)
);

-- Create indexes for session management
CREATE INDEX idx_sessions_account_id ON sessions(account_id);
CREATE INDEX idx_sessions_refresh_hash ON sessions(refresh_token_hash);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- Usage logs table - billing and analytics
CREATE TABLE usage_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    request_id VARCHAR(64) NOT NULL,
    endpoint VARCHAR(255) NOT NULL,
    method VARCHAR(10) NOT NULL,
    cost_usdc DECIMAL(20,6) NOT NULL DEFAULT 0.000000,
    status VARCHAR(50) NOT NULL,
    threat_detected BOOLEAN NOT NULL DEFAULT FALSE,
    threat_type VARCHAR(100),
    request_size_bytes INTEGER,
    response_size_bytes INTEGER,
    latency_ms INTEGER,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create indexes for usage queries
CREATE INDEX idx_usage_logs_account_id ON usage_logs(account_id);
CREATE INDEX idx_usage_logs_created_at ON usage_logs(created_at);
CREATE INDEX idx_usage_logs_account_created ON usage_logs(account_id, created_at);
CREATE INDEX idx_usage_logs_request_id ON usage_logs(request_id);

-- Payment transactions table to track atomic payment lifecycle
CREATE TABLE payment_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payment_nonce VARCHAR(128) UNIQUE NOT NULL,
    payment_header TEXT NOT NULL,
    payer_address VARCHAR(64) NOT NULL,
    receiver_address VARCHAR(64) NOT NULL,
    endpoint VARCHAR(255) NOT NULL,
    amount_usdc DECIMAL(20,6) NOT NULL,
    network VARCHAR(32) NOT NULL,
    status payment_status NOT NULL DEFAULT 'reserved',
    facilitator_payment_id VARCHAR(255),
    settlement_attempts INT DEFAULT 0,
    last_error TEXT,
    service_result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    executed_at TIMESTAMPTZ,
    settled_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,

    chain VARCHAR(10) DEFAULT 'base',
    CONSTRAINT valid_payer_address CHECK (payer_address ~ '^0x[a-fA-F0-9]{40}$' OR payer_address ~ '^[1-9A-HJ-NP-Za-km-z]{32,44}$'),
    CONSTRAINT valid_receiver_address CHECK (receiver_address ~ '^0x[a-fA-F0-9]{40}$' OR receiver_address ~ '^[1-9A-HJ-NP-Za-km-z]{32,44}$')
);

-- Index for nonce lookups (idempotency)
CREATE INDEX idx_payment_tx_nonce ON payment_transactions(payment_nonce);

-- Index for status queries
CREATE INDEX idx_payment_tx_status ON payment_transactions(status);

-- Partial index for pending settlements (retry worker)
CREATE INDEX idx_payment_tx_pending ON payment_transactions(status, created_at)
    WHERE status IN ('executing', 'settling', 'failed');

-- Index for expiration cleanup
CREATE INDEX idx_payment_tx_expires ON payment_transactions(expires_at)
    WHERE status = 'reserved';

-- Index for payer history
CREATE INDEX idx_payment_tx_payer ON payment_transactions(payer_address, created_at);

-- Link usage logs to payment transactions
ALTER TABLE usage_logs ADD COLUMN payment_transaction_id UUID
    REFERENCES payment_transactions(id);

CREATE INDEX idx_usage_logs_payment_tx ON usage_logs(payment_transaction_id);

-- Deposits table - payment tracking
CREATE TABLE deposits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    provider deposit_provider NOT NULL,
    amount_usdc DECIMAL(20,6) NOT NULL,
    fee_usdc DECIMAL(20,6) NOT NULL DEFAULT 0.000000,
    net_amount_usdc DECIMAL(20,6) NOT NULL, -- amount - fee
    status deposit_status NOT NULL DEFAULT 'pending',
    provider_transaction_id VARCHAR(255) UNIQUE,
    wallet_address VARCHAR(64), -- for direct deposits (EVM or Solana)
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT valid_deposit_wallet CHECK (wallet_address IS NULL OR wallet_address ~ '^0x[a-fA-F0-9]{40}$' OR wallet_address ~ '^[1-9A-HJ-NP-Za-km-z]{32,44}$')
);

-- Create indexes for deposit queries
CREATE INDEX idx_deposits_account_id ON deposits(account_id);
CREATE INDEX idx_deposits_status ON deposits(status);
CREATE INDEX idx_deposits_created_at ON deposits(created_at);
CREATE INDEX idx_deposits_provider_tx ON deposits(provider_transaction_id);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to auto-update updated_at on accounts
CREATE TRIGGER update_accounts_updated_at
    BEFORE UPDATE ON accounts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Function to calculate net amount on deposit insert/update
CREATE OR REPLACE FUNCTION calculate_deposit_net_amount()
RETURNS TRIGGER AS $$
BEGIN
    NEW.net_amount_usdc = NEW.amount_usdc - NEW.fee_usdc;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to auto-calculate net amount on deposits
CREATE TRIGGER calculate_deposit_net_amount_trigger
    BEFORE INSERT OR UPDATE ON deposits
    FOR EACH ROW
    EXECUTE FUNCTION calculate_deposit_net_amount();

-- Function to update account balance on completed deposit
CREATE OR REPLACE FUNCTION update_account_balance_on_deposit()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'completed' AND (OLD.status IS NULL OR OLD.status != 'completed') THEN
        UPDATE accounts
        SET balance_usdc = balance_usdc + NEW.net_amount_usdc
        WHERE id = NEW.account_id;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to auto-update account balance on deposit completion
CREATE TRIGGER update_account_balance_on_deposit_trigger
    AFTER INSERT OR UPDATE ON deposits
    FOR EACH ROW
    EXECUTE FUNCTION update_account_balance_on_deposit();

-- Function to update account balance on usage
CREATE OR REPLACE FUNCTION deduct_account_balance_on_usage()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE accounts
    SET balance_usdc = balance_usdc - NEW.cost_usdc
    WHERE id = NEW.account_id;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to auto-deduct account balance on usage log creation
CREATE TRIGGER deduct_account_balance_on_usage_trigger
    AFTER INSERT ON usage_logs
    FOR EACH ROW
    EXECUTE FUNCTION deduct_account_balance_on_usage();

-- Comments for documentation
COMMENT ON TABLE accounts IS 'Core account entity with Mullvad-style 16-digit account numbers';
COMMENT ON TABLE sessions IS 'JWT refresh token storage for session management';
COMMENT ON TABLE usage_logs IS 'Billing and analytics for API usage';
COMMENT ON TABLE deposits IS 'Payment tracking for account funding';
COMMENT ON COLUMN accounts.account_number IS 'Formatted as XXXX-XXXX-XXXX-XXXX';
COMMENT ON COLUMN accounts.wallet_address IS 'Wallet address for x402 payments (EVM 0x... or Solana base58)';
COMMENT ON COLUMN accounts.encrypted_private_key IS 'KMS-encrypted wallet private key (base64-encoded ciphertext)';
COMMENT ON COLUMN accounts.kms_key_id IS 'ARN or alias of the KMS key used for encryption';
COMMENT ON COLUMN accounts.key_encrypted_at IS 'Timestamp when the key was encrypted';
COMMENT ON TABLE payment_transactions IS 'Atomic payment lifecycle tracking for x402 reserve-commit pattern';
COMMENT ON COLUMN payment_transactions.payment_nonce IS 'Unique nonce from x402 payload (256-bit), used as idempotency key';
COMMENT ON COLUMN payment_transactions.payment_header IS 'Full X-Payment header for settlement retry';
COMMENT ON COLUMN payment_transactions.status IS 'State machine: reserved -> executing -> settling -> completed/failed/expired';
COMMENT ON COLUMN payment_transactions.service_result IS 'Cached scan result for idempotent replay';
COMMENT ON COLUMN payment_transactions.settlement_attempts IS 'Number of settlement retry attempts';
COMMENT ON COLUMN payment_transactions.chain IS 'Blockchain used for this payment (base, solana, etc.)';
-- Processed webhook events for idempotency (H6)
CREATE TABLE processed_webhook_events (
    event_id VARCHAR(255) PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_processed_webhook_events_processed_at ON processed_webhook_events(processed_at);
