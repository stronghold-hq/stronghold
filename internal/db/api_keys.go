package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// APIKey represents an API key for B2B authentication
type APIKey struct {
	ID         uuid.UUID  `json:"id"`
	AccountID  uuid.UUID  `json:"account_id"`
	KeyPrefix  string     `json:"key_prefix"`
	KeyHash    string     `json:"-"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// CreateAPIKey creates a new API key record
func (db *DB) CreateAPIKey(ctx context.Context, accountID uuid.UUID, keyPrefix, keyHash, name string) (*APIKey, error) {
	key := &APIKey{
		ID:        uuid.New(),
		AccountID: accountID,
		KeyPrefix: keyPrefix,
		KeyHash:   keyHash,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}

	_, err := db.pool.Exec(ctx, `
		INSERT INTO api_keys (id, account_id, key_prefix, key_hash, name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, key.ID, key.AccountID, key.KeyPrefix, key.KeyHash, key.Name, key.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return key, nil
}

// GetAPIKeyByHash retrieves an API key by its SHA-256 hash (must not be revoked)
func (db *DB) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	key := &APIKey{}
	err := db.QueryRow(ctx, `
		SELECT id, account_id, key_prefix, key_hash, name, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE key_hash = $1 AND revoked_at IS NULL
	`, keyHash).Scan(
		&key.ID, &key.AccountID, &key.KeyPrefix, &key.KeyHash, &key.Name,
		&key.CreatedAt, &key.LastUsedAt, &key.RevokedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("API key not found")
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return key, nil
}

// ListAPIKeys lists all non-revoked API keys for an account
func (db *DB) ListAPIKeys(ctx context.Context, accountID uuid.UUID) ([]APIKey, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, account_id, key_prefix, key_hash, name, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE account_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var key APIKey
		if err := rows.Scan(
			&key.ID, &key.AccountID, &key.KeyPrefix, &key.KeyHash, &key.Name,
			&key.CreatedAt, &key.LastUsedAt, &key.RevokedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API keys: %w", err)
	}

	return keys, nil
}

// RevokeAPIKey revokes an API key, verifying ownership
func (db *DB) RevokeAPIKey(ctx context.Context, keyID, accountID uuid.UUID) error {
	result, err := db.pool.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $1
		WHERE id = $2 AND account_id = $3 AND revoked_at IS NULL
	`, time.Now().UTC(), keyID, accountID)

	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("API key not found or already revoked")
	}

	return nil
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp for an API key
func (db *DB) UpdateAPIKeyLastUsed(ctx context.Context, keyID uuid.UUID) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE api_keys SET last_used_at = $1 WHERE id = $2
	`, time.Now().UTC(), keyID)

	if err != nil {
		return fmt.Errorf("failed to update API key last used: %w", err)
	}

	return nil
}

// CountActiveAPIKeys counts non-revoked API keys for an account
func (db *DB) CountActiveAPIKeys(ctx context.Context, accountID uuid.UUID) (int, error) {
	var count int
	err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM api_keys WHERE account_id = $1 AND revoked_at IS NULL
	`, accountID).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to count API keys: %w", err)
	}

	return count, nil
}
