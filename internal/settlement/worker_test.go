package settlement

import (
	"context"
	"testing"
	"time"

	"stronghold/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorker_ExponentialBackoff(t *testing.T) {
	w := &Worker{}

	// Backoff formula: base * 2^attempts + jitter (0 to 50% of delay)
	// So max is 1.5x the base delay, capped at 30s
	testCases := []struct {
		attempts     int
		expectedMin  time.Duration
		expectedMax  time.Duration // Base + 50% jitter
	}{
		{0, 2 * time.Second, 2*time.Second + 2*time.Second/2},        // 2s base + up to 1s jitter
		{1, 4 * time.Second, 4*time.Second + 4*time.Second/2},        // 4s base + up to 2s jitter
		{2, 8 * time.Second, 8*time.Second + 8*time.Second/2},        // 8s base + up to 4s jitter
		{3, 16 * time.Second, 16*time.Second + 16*time.Second/2},     // 16s base + up to 8s jitter
		{4, 30 * time.Second, 30*time.Second + 30*time.Second/2},     // Capped at 30s + up to 15s jitter
		{5, 30 * time.Second, 30*time.Second + 30*time.Second/2},     // Still capped
		{10, 30 * time.Second, 30*time.Second + 30*time.Second/2},    // Still capped
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			backoff := w.calculateBackoff(tc.attempts)
			assert.GreaterOrEqual(t, backoff, tc.expectedMin)
			assert.LessOrEqual(t, backoff, tc.expectedMax)
		})
	}
}

func TestDefaultWorkerConfig(t *testing.T) {
	cfg := DefaultWorkerConfig()

	assert.Equal(t, 10*time.Second, cfg.RetryInterval)
	assert.Equal(t, 5, cfg.MaxRetryAttempts)
	assert.Equal(t, 100, cfg.BatchSize)
	assert.Equal(t, 1*time.Minute, cfg.ExpirationCheckInterval)
}

func TestNewWorker(t *testing.T) {
	x402cfg := &config.X402Config{
		EVMWalletAddress: "0x1234567890123456789012345678901234567890",
		FacilitatorURL:   "https://x402.org/facilitator",
		Networks:         []string{"base-sepolia"},
	}

	t.Run("with default config", func(t *testing.T) {
		worker := NewWorker(nil, x402cfg, nil)
		assert.NotNil(t, worker)
		assert.NotNil(t, worker.config)
		assert.Equal(t, 10*time.Second, worker.config.RetryInterval)
	})

	t.Run("with custom config", func(t *testing.T) {
		customCfg := &WorkerConfig{
			RetryInterval:           10 * time.Second,
			MaxRetryAttempts:        3,
			BatchSize:               50,
			ExpirationCheckInterval: 30 * time.Second,
		}

		worker := NewWorker(nil, x402cfg, customCfg)
		assert.NotNil(t, worker)
		assert.Equal(t, 10*time.Second, worker.config.RetryInterval)
		assert.Equal(t, 3, worker.config.MaxRetryAttempts)
	})
}

func TestWorker_GracefulShutdown(t *testing.T) {
	x402cfg := &config.X402Config{
		EVMWalletAddress: "0x1234567890123456789012345678901234567890",
		FacilitatorURL:   "https://x402.org/facilitator",
		Networks:         []string{"base-sepolia"},
	}

	cfg := &WorkerConfig{
		RetryInterval:           100 * time.Millisecond,
		MaxRetryAttempts:        3,
		BatchSize:               10,
		ExpirationCheckInterval: 100 * time.Millisecond,
	}

	worker := NewWorker(nil, x402cfg, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	// Start worker
	worker.Start(ctx)

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Stop should complete within reasonable time
	done := make(chan struct{})
	go func() {
		cancel() // Cancel context
		worker.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success - worker stopped gracefully
	case <-time.After(2 * time.Second):
		t.Fatal("Worker did not shut down within 2 seconds")
	}
}

func TestWorker_ContextCancellation(t *testing.T) {
	x402cfg := &config.X402Config{
		EVMWalletAddress: "0x1234567890123456789012345678901234567890",
		FacilitatorURL:   "https://x402.org/facilitator",
		Networks:         []string{"base-sepolia"},
	}

	cfg := &WorkerConfig{
		RetryInterval:           100 * time.Millisecond,
		MaxRetryAttempts:        3,
		BatchSize:               10,
		ExpirationCheckInterval: 100 * time.Millisecond,
	}

	worker := NewWorker(nil, x402cfg, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	// Start worker
	worker.Start(ctx)

	// Cancel context
	cancel()

	// Worker should stop on context cancellation
	done := make(chan struct{})
	go func() {
		worker.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Worker did not stop on context cancellation")
	}
}

// Integration tests that require real database
func TestWorker_RetriesFailedSettlements_Integration(t *testing.T) {
	t.Skip("Integration test requires database and mock facilitator")

	// This would test:
	// 1. Create a payment in failed state
	// 2. Start worker
	// 3. Mock facilitator to succeed
	// 4. Verify payment becomes completed
}

func TestWorker_ExpiresStaleReservations_Integration(t *testing.T) {
	t.Skip("Integration test requires database")

	// This would test:
	// 1. Create a payment in reserved state with expired timestamp
	// 2. Run expireStaleReservations
	// 3. Verify payment is now expired
}

func TestWorker_BackoffTiming(t *testing.T) {
	w := &Worker{}

	// Test that backoff generally increases with attempts (accounting for jitter)
	// Base delays: 2s, 4s, 8s, 16s, 30s (cap)
	baseDelays := []time.Duration{
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second, // cap
		30 * time.Second,
		30 * time.Second,
	}

	for i := 0; i < len(baseDelays); i++ {
		backoff := w.calculateBackoff(i)
		// Backoff should be at least the base delay
		assert.GreaterOrEqual(t, backoff, baseDelays[i],
			"Backoff at attempt %d should be at least %v", i, baseDelays[i])
		// Backoff should be at most base + 50% jitter
		maxExpected := baseDelays[i] + baseDelays[i]/2
		assert.LessOrEqual(t, backoff, maxExpected,
			"Backoff at attempt %d should be at most %v", i, maxExpected)
	}
}

func TestWorker_HttpClientTimeout(t *testing.T) {
	x402cfg := &config.X402Config{
		EVMWalletAddress: "0x1234567890123456789012345678901234567890",
		FacilitatorURL:   "https://x402.org/facilitator",
		Networks:         []string{"base-sepolia"},
	}

	worker := NewWorker(nil, x402cfg, nil)

	assert.Equal(t, 30*time.Second, worker.httpClient.Timeout)
}

func TestWorker_StopChannelClosed(t *testing.T) {
	x402cfg := &config.X402Config{}

	worker := NewWorker(nil, x402cfg, nil)

	// Stop without starting - should not panic
	require.NotPanics(t, func() {
		close(worker.stopCh)
	})
}

// TestWorker_RunRetryLoop_ExitsOnStop tests that the retry loop exits when stopped
func TestWorker_RunRetryLoop_ExitsOnStop(t *testing.T) {
	x402cfg := &config.X402Config{}

	cfg := &WorkerConfig{
		RetryInterval:           50 * time.Millisecond,
		MaxRetryAttempts:        3,
		BatchSize:               10,
		ExpirationCheckInterval: 50 * time.Millisecond,
	}

	worker := NewWorker(nil, x402cfg, cfg)

	ctx := context.Background()
	done := make(chan struct{})

	go func() {
		worker.runRetryLoop(ctx)
		close(done)
	}()

	// Let loop start
	time.Sleep(25 * time.Millisecond)

	// Close stop channel
	close(worker.stopCh)

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("runRetryLoop did not exit on stop")
	}
}

// TestWorker_RunExpirationLoop_ExitsOnStop tests that the expiration loop exits when stopped
func TestWorker_RunExpirationLoop_ExitsOnStop(t *testing.T) {
	x402cfg := &config.X402Config{}

	cfg := &WorkerConfig{
		RetryInterval:           50 * time.Millisecond,
		MaxRetryAttempts:        3,
		BatchSize:               10,
		ExpirationCheckInterval: 50 * time.Millisecond,
	}

	worker := NewWorker(nil, x402cfg, cfg)

	ctx := context.Background()
	done := make(chan struct{})

	go func() {
		worker.runExpirationLoop(ctx)
		close(done)
	}()

	// Let loop start
	time.Sleep(25 * time.Millisecond)

	// Close stop channel
	close(worker.stopCh)

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("runExpirationLoop did not exit on stop")
	}
}

func TestWorker_CalculateBackoff_MaxCap(t *testing.T) {
	w := &Worker{}

	// Even with very high attempt counts, should never exceed 5 minutes + 50% jitter (7.5 minutes)
	maxWithJitter := 5*time.Minute + 5*time.Minute/2
	for attempts := 0; attempts < 100; attempts++ {
		backoff := w.calculateBackoff(attempts)
		assert.LessOrEqual(t, backoff, maxWithJitter,
			"Backoff should never exceed 7.5 minutes (5 min + 50%% jitter), got %v for attempt %d", backoff, attempts)
	}
}
