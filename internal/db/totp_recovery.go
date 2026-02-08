package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// StoreRecoveryCodes stores hashed recovery codes for an account.
func (db *DB) StoreRecoveryCodes(ctx context.Context, accountID uuid.UUID, codes []string) error {
	if len(codes) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, code := range codes {
		batch.Queue(`
			INSERT INTO totp_recovery_codes (account_id, code_hash)
			VALUES ($1, $2)
		`, accountID, HashToken(code))
	}

	br := db.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range codes {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to store recovery code: %w", err)
		}
	}
	return nil
}

// ClearRecoveryCodes deletes all recovery codes for an account.
func (db *DB) ClearRecoveryCodes(ctx context.Context, accountID uuid.UUID) error {
	_, err := db.pool.Exec(ctx, `
		DELETE FROM totp_recovery_codes
		WHERE account_id = $1
	`, accountID)
	if err != nil {
		return fmt.Errorf("failed to clear recovery codes: %w", err)
	}
	return nil
}

// UseRecoveryCode marks a recovery code as used if valid.
func (db *DB) UseRecoveryCode(ctx context.Context, accountID uuid.UUID, code string) (bool, error) {
	hashed := HashToken(code)
	now := time.Now().UTC()
	res, err := db.pool.Exec(ctx, `
		UPDATE totp_recovery_codes
		SET used_at = $1
		WHERE account_id = $2 AND code_hash = $3 AND used_at IS NULL
	`, now, accountID, hashed)
	if err != nil {
		return false, fmt.Errorf("failed to use recovery code: %w", err)
	}
	return res.RowsAffected() > 0, nil
}
