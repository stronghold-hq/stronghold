package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Device represents a trusted device for an account.
type Device struct {
	ID         uuid.UUID  `json:"id"`
	AccountID  uuid.UUID  `json:"account_id"`
	Label      *string    `json:"label,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// CreateDeviceToken stores a new trusted device token.
func (db *DB) CreateDeviceToken(ctx context.Context, accountID uuid.UUID, token, label string, expiresAt *time.Time) (*Device, error) {
	hashed := HashToken(token)
	var device Device
	err := db.pool.QueryRow(ctx, `
		INSERT INTO account_devices (account_id, device_token_hash, label, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, account_id, label, created_at, last_seen_at, expires_at
	`, accountID, hashed, labelOrNull(label), expiresAt).Scan(
		&device.ID, &device.AccountID, &device.Label, &device.CreatedAt, &device.LastSeenAt, &device.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create device token: %w", err)
	}
	return &device, nil
}

// GetDeviceByToken returns the device if the token is valid and not expired.
func (db *DB) GetDeviceByToken(ctx context.Context, accountID uuid.UUID, token string) (*Device, error) {
	hashed := HashToken(token)
	var device Device
	err := db.pool.QueryRow(ctx, `
		SELECT id, account_id, label, created_at, last_seen_at, expires_at
		FROM account_devices
		WHERE account_id = $1 AND device_token_hash = $2
	`, accountID, hashed).Scan(
		&device.ID, &device.AccountID, &device.Label, &device.CreatedAt, &device.LastSeenAt, &device.ExpiresAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	if device.ExpiresAt != nil && device.ExpiresAt.Before(time.Now().UTC()) {
		_ = db.RevokeDevice(ctx, accountID, device.ID)
		return nil, pgx.ErrNoRows
	}

	return &device, nil
}

// TouchDevice updates last_seen_at for a device token.
func (db *DB) TouchDevice(ctx context.Context, accountID uuid.UUID, token string) error {
	hashed := HashToken(token)
	_, err := db.pool.Exec(ctx, `
		UPDATE account_devices
		SET last_seen_at = $1
		WHERE account_id = $2 AND device_token_hash = $3
	`, time.Now().UTC(), accountID, hashed)
	if err != nil {
		return fmt.Errorf("failed to update device last_seen_at: %w", err)
	}
	return nil
}

// ListDevices returns trusted devices for an account.
func (db *DB) ListDevices(ctx context.Context, accountID uuid.UUID) ([]*Device, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, account_id, label, created_at, last_seen_at, expires_at
		FROM account_devices
		WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var device Device
		if err := rows.Scan(&device.ID, &device.AccountID, &device.Label, &device.CreatedAt, &device.LastSeenAt, &device.ExpiresAt); err != nil {
			return nil, fmt.Errorf("failed to scan device: %w", err)
		}
		devices = append(devices, &device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate devices: %w", err)
	}
	return devices, nil
}

// RevokeDevice deletes a specific trusted device.
func (db *DB) RevokeDevice(ctx context.Context, accountID uuid.UUID, deviceID uuid.UUID) error {
	_, err := db.pool.Exec(ctx, `
		DELETE FROM account_devices
		WHERE account_id = $1 AND id = $2
	`, accountID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to revoke device: %w", err)
	}
	return nil
}

// RevokeAllDevices deletes all trusted devices for an account.
func (db *DB) RevokeAllDevices(ctx context.Context, accountID uuid.UUID) error {
	_, err := db.pool.Exec(ctx, `
		DELETE FROM account_devices
		WHERE account_id = $1
	`, accountID)
	if err != nil {
		return fmt.Errorf("failed to revoke all devices: %w", err)
	}
	return nil
}

func labelOrNull(label string) *string {
	if label == "" {
		return nil
	}
	return &label
}
