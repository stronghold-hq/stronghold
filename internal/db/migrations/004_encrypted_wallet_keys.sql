-- Migration: Add encrypted wallet key storage for cross-device wallet access
-- Keys are encrypted via AWS KMS before storage

ALTER TABLE accounts
ADD COLUMN encrypted_private_key TEXT,
ADD COLUMN kms_key_id VARCHAR(255),
ADD COLUMN key_encrypted_at TIMESTAMP WITH TIME ZONE;

-- Index to quickly check which accounts have encrypted keys
CREATE INDEX idx_accounts_has_encrypted_key
ON accounts((encrypted_private_key IS NOT NULL));

COMMENT ON COLUMN accounts.encrypted_private_key IS 'KMS-encrypted wallet private key (base64-encoded ciphertext)';
COMMENT ON COLUMN accounts.kms_key_id IS 'ARN or alias of the KMS key used for encryption';
COMMENT ON COLUMN accounts.key_encrypted_at IS 'Timestamp when the key was encrypted';
