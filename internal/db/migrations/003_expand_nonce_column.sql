-- Expand payment_nonce column to support 256-bit nonces
-- Migration: 003_expand_nonce_column
--
-- Previous: 16 bytes (128 bits) = 32 hex chars
-- New: 32 bytes (256 bits) = 64 hex chars
-- Column size: VARCHAR(128) to safely accommodate future expansion

ALTER TABLE payment_transactions
    ALTER COLUMN payment_nonce TYPE VARCHAR(128);

COMMENT ON COLUMN payment_transactions.payment_nonce IS 'Unique nonce from x402 payload (256-bit), used as idempotency key';
