// Package settlement provides background workers for payment settlement retry and cleanup
package settlement

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"stronghold/internal/config"
	"stronghold/internal/db"
	"stronghold/internal/wallet"
)

// WorkerConfig holds configuration for the settlement worker
type WorkerConfig struct {
	// RetryInterval is how often to check for failed settlements
	RetryInterval time.Duration
	// MaxRetryAttempts is the maximum number of settlement retry attempts
	MaxRetryAttempts int
	// BatchSize is the maximum number of payments to process per retry cycle
	BatchSize int
	// ExpirationCheckInterval is how often to check for expired reservations
	ExpirationCheckInterval time.Duration
}

// DefaultWorkerConfig returns sensible defaults for the worker
func DefaultWorkerConfig() *WorkerConfig {
	return &WorkerConfig{
		RetryInterval:           30 * time.Second,
		MaxRetryAttempts:        5,
		BatchSize:               100,
		ExpirationCheckInterval: 1 * time.Minute,
	}
}

// Worker handles background settlement retry and reservation expiration
type Worker struct {
	db         *db.DB
	x402Config *config.X402Config
	config     *WorkerConfig
	httpClient *http.Client
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewWorker creates a new settlement worker
func NewWorker(database *db.DB, x402Config *config.X402Config, cfg *WorkerConfig) *Worker {
	if cfg == nil {
		cfg = DefaultWorkerConfig()
	}
	return &Worker{
		db:         database,
		x402Config: x402Config,
		config:     cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

// Start begins the background worker
func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(2)

	// Settlement retry worker
	go func() {
		defer w.wg.Done()
		w.runRetryLoop(ctx)
	}()

	// Expiration cleanup worker
	go func() {
		defer w.wg.Done()
		w.runExpirationLoop(ctx)
	}()

	slog.Info("settlement worker started")
}

// Stop gracefully stops the worker
func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	slog.Info("settlement worker stopped")
}

// runRetryLoop periodically retries failed settlements
func (w *Worker) runRetryLoop(ctx context.Context) {
	ticker := time.NewTicker(w.config.RetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.retryFailedSettlements(ctx)
		}
	}
}

// runExpirationLoop periodically expires stale reservations
func (w *Worker) runExpirationLoop(ctx context.Context) {
	ticker := time.NewTicker(w.config.ExpirationCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.expireStaleReservations(ctx)
		}
	}
}

// retryFailedSettlements processes payments that failed settlement
func (w *Worker) retryFailedSettlements(ctx context.Context) {
	// Get failed payments that haven't exceeded max retries
	payments, err := w.db.GetPendingSettlements(ctx, w.config.MaxRetryAttempts, w.config.BatchSize)
	if err != nil {
		slog.Error("failed to get pending settlements", "error", err)
		return
	}

	if len(payments) == 0 {
		return
	}

	slog.Info("retrying failed settlements", "count", len(payments))

	for _, payment := range payments {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
		}

		// Calculate backoff delay based on attempt number
		backoff := w.calculateBackoff(payment.SettlementAttempts)
		timeSinceExecution := time.Since(*payment.ExecutedAt)
		if timeSinceExecution < backoff {
			// Not yet time to retry this payment
			continue
		}

		// Transition to settling
		if err := w.db.MarkSettling(ctx, payment.ID); err != nil {
			slog.Error("failed to mark payment as settling", "payment_id", payment.ID, "error", err)
			continue
		}

		// Attempt settlement
		paymentID, err := w.settlePayment(payment.PaymentHeader)
		if err != nil {
			slog.Error("settlement retry failed", "payment_id", payment.ID, "attempt", payment.SettlementAttempts+1, "error", err)
			if err := w.db.FailSettlement(ctx, payment.ID, err.Error()); err != nil {
				slog.Error("failed to record settlement failure", "error", err)
			}
			continue
		}

		// Success!
		if err := w.db.CompleteSettlement(ctx, payment.ID, paymentID); err != nil {
			slog.Error("failed to mark payment as completed", "payment_id", payment.ID, "error", err)
			continue
		}

		slog.Info("successfully settled payment on retry", "payment_id", payment.ID, "attempt", payment.SettlementAttempts+1)
	}
}

// expireStaleReservations marks old reserved payments as expired
func (w *Worker) expireStaleReservations(ctx context.Context) {
	count, err := w.db.ExpireStaleReservations(ctx)
	if err != nil {
		slog.Error("failed to expire stale reservations", "error", err)
		return
	}

	if count > 0 {
		slog.Info("expired stale payment reservations", "count", count)
	}
}

// calculateBackoff returns the backoff duration for a given attempt number
// Uses exponential backoff: 5s, 10s, 20s, 40s, 80s
func (w *Worker) calculateBackoff(attempts int) time.Duration {
	baseDelay := 5 * time.Second
	maxDelay := 5 * time.Minute

	delay := baseDelay
	for i := 0; i < attempts; i++ {
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
			break
		}
	}

	return delay
}

// settlePayment attempts to settle a payment with the facilitator
func (w *Worker) settlePayment(paymentHeader string) (string, error) {
	payload, err := wallet.ParseX402Payment(paymentHeader)
	if err != nil {
		return "", fmt.Errorf("failed to parse payment: %w", err)
	}

	settleReq := struct {
		Payment  string `json:"payment"`
		Network  string `json:"network"`
		Amount   string `json:"amount"`
		Receiver string `json:"receiver"`
		Token    string `json:"token"`
	}{
		Payment:  paymentHeader,
		Network:  payload.Network,
		Amount:   payload.Amount,
		Receiver: payload.Receiver,
		Token:    payload.TokenAddress,
	}

	settleBody, err := json.Marshal(settleReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal settle request: %w", err)
	}

	facilitatorURL := w.x402Config.FacilitatorURL
	if facilitatorURL == "" {
		facilitatorURL = "https://x402.org/facilitator"
	}

	resp, err := w.httpClient.Post(
		facilitatorURL+"/settle",
		"application/json",
		bytes.NewReader(settleBody),
	)
	if err != nil {
		return "", fmt.Errorf("failed to call facilitator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("facilitator settlement failed: %s", resp.Status)
	}

	var settleResult struct {
		PaymentID string `json:"payment_id"`
		TxHash    string `json:"tx_hash,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&settleResult); err != nil {
		return "", fmt.Errorf("failed to decode settle response: %w", err)
	}

	return settleResult.PaymentID, nil
}
