-- Add CHECK constraint to prevent empty payment nonces.
-- Defense-in-depth: the UNIQUE constraint allows one empty-nonce row,
-- but a CHECK constraint prevents it entirely.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'payment_nonce_not_empty'
    ) THEN
        ALTER TABLE payment_transactions
        ADD CONSTRAINT payment_nonce_not_empty CHECK (payment_nonce != '');
    END IF;
END $$;
