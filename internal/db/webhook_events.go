package db

import (
	"context"
	"fmt"
)

// IsWebhookEventProcessed checks whether a webhook event has already been
// successfully processed. This is a read-only check — the event is not recorded.
func (db *DB) IsWebhookEventProcessed(ctx context.Context, eventID string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM processed_webhook_events WHERE event_id = $1)`,
		eventID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check webhook event: %w", err)
	}
	return exists, nil
}

// RecordWebhookEvent marks a webhook event as successfully processed.
// Uses ON CONFLICT DO NOTHING so concurrent calls are safe.
func (db *DB) RecordWebhookEvent(ctx context.Context, eventID, eventType string) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO processed_webhook_events (event_id, event_type)
		VALUES ($1, $2)
		ON CONFLICT (event_id) DO NOTHING
	`, eventID, eventType)
	if err != nil {
		return fmt.Errorf("failed to record webhook event: %w", err)
	}
	return nil
}

// CleanupOldWebhookEvents removes processed webhook events older than the given number of days
func (db *DB) CleanupOldWebhookEvents(ctx context.Context, retentionDays int) (int64, error) {
	query := `
		DELETE FROM processed_webhook_events
		WHERE processed_at < NOW() - make_interval(days => $1)
	`

	result, err := db.pool.Exec(ctx, query, retentionDays)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old webhook events: %w", err)
	}

	return result.RowsAffected(), nil
}
