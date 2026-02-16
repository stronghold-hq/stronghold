-- Migration: 002_dual_wallets
-- Add separate EVM and Solana wallet address columns to support dual-chain wallets

-- Step 1: Add new columns
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS evm_wallet_address VARCHAR(42);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS solana_wallet_address VARCHAR(44);

-- Step 2: Migrate existing wallet_address data to the appropriate new column
-- Only runs if wallet_address column still exists (idempotency guard)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'accounts' AND column_name = 'wallet_address'
    ) THEN
        -- EVM addresses start with 0x
        UPDATE accounts
        SET evm_wallet_address = wallet_address
        WHERE wallet_address IS NOT NULL
          AND wallet_address ~ '^0x[a-fA-F0-9]{40}$'
          AND evm_wallet_address IS NULL;

        -- Solana addresses are base58, 32-44 chars, no 0x prefix
        UPDATE accounts
        SET solana_wallet_address = wallet_address
        WHERE wallet_address IS NOT NULL
          AND wallet_address ~ '^[1-9A-HJ-NP-Za-km-z]{32,44}$'
          AND wallet_address !~ '^0x'
          AND solana_wallet_address IS NULL;
    END IF;
END $$;

-- Step 3: Drop old column and its index/constraint
DROP INDEX IF EXISTS accounts_wallet_address_unique;
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS valid_wallet_address;
ALTER TABLE accounts DROP COLUMN IF EXISTS wallet_address;

-- Step 4: Add CHECK constraints for valid address formats (idempotent with DO block)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'valid_evm_wallet_address'
    ) THEN
        ALTER TABLE accounts ADD CONSTRAINT valid_evm_wallet_address
            CHECK (evm_wallet_address IS NULL OR evm_wallet_address ~ '^0x[a-fA-F0-9]{40}$');
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'valid_solana_wallet_address'
    ) THEN
        ALTER TABLE accounts ADD CONSTRAINT valid_solana_wallet_address
            CHECK (solana_wallet_address IS NULL OR solana_wallet_address ~ '^[1-9A-HJ-NP-Za-km-z]{32,44}$');
    END IF;
END $$;

-- Step 5: Add unique indexes (WHERE NOT NULL, same pattern as the original)
CREATE UNIQUE INDEX IF NOT EXISTS accounts_evm_wallet_address_unique
    ON accounts(evm_wallet_address) WHERE evm_wallet_address IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS accounts_solana_wallet_address_unique
    ON accounts(solana_wallet_address) WHERE solana_wallet_address IS NOT NULL;

-- Step 6: Update column comments
COMMENT ON COLUMN accounts.evm_wallet_address IS 'EVM wallet address for x402 payments (0x + 40 hex chars)';
COMMENT ON COLUMN accounts.solana_wallet_address IS 'Solana wallet address for x402 payments (base58, 32-44 chars)';
