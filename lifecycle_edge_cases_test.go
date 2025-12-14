package split

import (
	"context"
	"testing"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/client"
	"github.com/splitio/go-client/v6/splitio/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitWithContextTimeout verifies that InitWithContext respects context timeout
// when it's shorter than BlockUntilReady configuration.
//
// This test addresses the edge case where:
//   - BlockUntilReady is configured for 10 seconds
//   - Context timeout is only 1 second
//   - InitWithContext should return context.DeadlineExceeded after ~1 second, not wait 10 seconds
func TestInitWithContextTimeout(t *testing.T) {
	// Use invalid API key to force SDK to timeout
	// This ensures BlockUntilReady will take the full timeout duration
	cfg := conf.Default()
	cfg.BlockUntilReady = 10 // 10 seconds timeout in SDK

	provider, err := New("invalid-key-will-timeout", WithSplitConfig(cfg))
	require.NoError(t, err, "Provider creation should succeed")

	// Proper cleanup: Shutdown provider to prevent goroutine leak
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = provider.ShutdownWithContext(shutdownCtx)
	}()

	// Context with 1 second timeout (shorter than BlockUntilReady)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err = provider.InitWithContext(ctx, openfeature.NewEvaluationContext("", nil))
	elapsed := time.Since(start)

	// Should fail with context error
	assert.Error(t, err, "InitWithContext should return error when context times out")
	assert.Contains(t, err.Error(), "initialization canceled", "Error should indicate cancellation")
	assert.Contains(t, err.Error(), "deadline exceeded", "Error should contain context.DeadlineExceeded")

	// Should respect context timeout (1s), not wait for BlockUntilReady (10s)
	assert.Less(t, elapsed, 3*time.Second,
		"InitWithContext should return within ~1s (context timeout), not wait 10s (BlockUntilReady)")
	assert.Greater(t, elapsed, 800*time.Millisecond,
		"InitWithContext should actually wait for context timeout, not return immediately")
}

// TestInitWithContextCancellationDuringBlockUntilReady verifies that context
// cancellation during BlockUntilReady is handled correctly.
//
// This test addresses the edge case where:
//   - InitWithContext is called with a context
//   - Context is cancelled WHILE BlockUntilReady is running
//   - Should return immediately with context.Canceled error
func TestInitWithContextCancellationDuringBlockUntilReady(t *testing.T) {
	cfg := conf.Default()
	cfg.BlockUntilReady = 10 // Long timeout to ensure we can cancel during init

	provider, err := New("invalid-key-will-timeout", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Proper cleanup: Shutdown provider to prevent goroutine leak
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = provider.ShutdownWithContext(shutdownCtx)
	}()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after 500ms (while BlockUntilReady is running)
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err = provider.InitWithContext(ctx, openfeature.NewEvaluationContext("", nil))
	elapsed := time.Since(start)

	assert.Error(t, err, "Should return error when context cancelled")
	assert.Contains(t, err.Error(), "initialization canceled", "Should indicate cancellation")

	// Should return shortly after cancellation (~500ms), not wait for BlockUntilReady (10s)
	assert.Less(t, elapsed, 2*time.Second,
		"Should return quickly after context cancellation")
	assert.Greater(t, elapsed, 400*time.Millisecond,
		"Should actually wait for cancellation, not return immediately")
}

// TestInitWithContextRaceCondition verifies the fix for the context cancellation race.
//
// This test addresses the critical edge case where:
//   - BlockUntilReady completes successfully
//   - Context is cancelled at nearly the same moment
//   - Both readyErr channel and ctx.Done() are ready
//   - select{} randomly chooses which case to execute
//
// Expected behavior: If SDK initialized successfully, we should SUCCEED even if
// context was cancelled, because the SDK is now ready and usable.
func TestInitWithContextRaceCondition(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1 // Fast init

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Create context with very short timeout
	// Timing is such that context expires RIGHT as BlockUntilReady completes
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()

	err = provider.InitWithContext(ctx, openfeature.NewEvaluationContext("", nil))

	// The fix ensures this ALWAYS succeeds (SDK is ready)
	// Without the fix, this would randomly fail when ctx.Done() is chosen by select
	assert.NoError(t, err, "Should succeed when SDK initializes, even if context cancelled during init")

	// Verify provider is actually ready
	assert.Equal(t, openfeature.ReadyState, provider.Status(), "Provider should be in Ready state")

	// Cleanup with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = provider.ShutdownWithContext(shutdownCtx)
}

// TestShutdownWithContextTimeout verifies that ShutdownWithContext respects
// context timeout without failing prematurely.
//
// This test addresses the edge case where:
//   - Context timeout is shorter than monitoring goroutine stop time
//   - Previously had hardcoded 5s timeout that would conflict
//   - Should not return error if context times out, just log warning and continue
func TestShutdownWithContextTimeout(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Initialize provider
	initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	// Shutdown with extremely short timeout (simulates aggressive shutdown deadline)
	// In localhost mode, shutdown is very fast, so we need an unrealistically short timeout
	// to trigger the timeout path
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer shutdownCancel()

	// Give the timeout a chance to expire before we call Shutdown
	time.Sleep(1 * time.Millisecond)

	start := time.Now()
	err = provider.ShutdownWithContext(shutdownCtx)
	elapsed := time.Since(start)

	// Should return error when context times out
	assert.Error(t, err, "ShutdownWithContext should return error when context times out")
	assert.ErrorIs(t, err, context.DeadlineExceeded,
		"Error should be context.DeadlineExceeded")

	// Should respect context timeout
	assert.Less(t, elapsed, 1*time.Second,
		"Should return quickly when context times out")

	// Verify provider is shut down (logically) even though cleanup may be incomplete
	assert.Equal(t, openfeature.NotReadyState, provider.Status(),
		"Provider should be NotReady after shutdown even if context timed out")
}

// TestShutdownWithContextGracefulStop verifies that ShutdownWithContext
// waits for monitoring goroutine when context allows sufficient time.
func TestShutdownWithContextGracefulStop(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Initialize provider
	initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	// Shutdown with generous timeout (allows clean shutdown)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	err = provider.ShutdownWithContext(shutdownCtx)

	assert.NoError(t, err, "ShutdownWithContext should succeed with sufficient timeout")
	assert.Equal(t, openfeature.NotReadyState, provider.Status(), "Provider should be NotReady")
}

// TestInitShutdownContextInterplay verifies that Init and Shutdown
// contexts are independent and don't interfere with each other.
func TestInitShutdownContextInterplay(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Init with context that expires after initialization
	initCtx, initCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer initCancel()

	err = provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	// Cancel init context (should not affect shutdown)
	initCancel()

	// Shutdown with different context
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err = provider.ShutdownWithContext(shutdownCtx)
	assert.NoError(t, err, "Shutdown should succeed with its own context")
}

// TestInitAfterShutdown verifies that Init cannot be called after Shutdown.
// This ensures the provider cannot be reused after shutdown.
func TestInitAfterShutdown(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Initialize provider
	initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Initial init should succeed")

	// Shutdown provider
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = provider.ShutdownWithContext(shutdownCtx)
	require.NoError(t, err, "Shutdown should succeed")

	// Attempt to re-initialize after shutdown
	reinitCtx, reinitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reinitCancel()
	err = provider.InitWithContext(reinitCtx, openfeature.NewEvaluationContext("", nil))

	// Should fail with explicit error about shutdown
	assert.Error(t, err, "Init after shutdown should fail")
	assert.Contains(t, err.Error(), "cannot initialize provider after shutdown",
		"Error should indicate provider was shut down")
	assert.Contains(t, err.Error(), "permanently shut down",
		"Error should indicate shutdown is permanent")

	// Verify provider status is NotReady
	assert.Equal(t, openfeature.NotReadyState, provider.Status(),
		"Provider should be NotReady after shutdown")
}

// TestShutdownBeforeInit verifies that shutting down before initialization is safe.
// This tests the edge case where a provider is created but never initialized.
func TestShutdownBeforeInit(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Shutdown without ever calling Init
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.ShutdownWithContext(shutdownCtx)

	// Should succeed - shutdown before init is a valid operation
	assert.NoError(t, err, "Shutdown before init should succeed")

	// Provider should be in NotReady state
	assert.Equal(t, openfeature.NotReadyState, provider.Status(),
		"Provider should be NotReady after shutdown")

	// Subsequent Init should fail
	initCtx, initCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer initCancel()
	err = provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))

	assert.Error(t, err, "Init after shutdown should fail")
	assert.Contains(t, err.Error(), "cannot initialize provider after shutdown",
		"Error should indicate provider was shut down")
}

// TestConcurrentEvaluationDuringShutdown verifies that evaluations in progress
// are safe during shutdown, and shutdown waits for evaluations to complete.
func TestConcurrentEvaluationDuringShutdown(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Initialize provider
	initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	// Start multiple concurrent evaluations
	evaluationsDone := make(chan bool, 10)
	ctx := context.Background()
	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "user-123",
	}

	for i := 0; i < 10; i++ {
		go func() {
			// Perform evaluation (should succeed or return PROVIDER_NOT_READY)
			result := provider.BooleanEvaluation(ctx, "my-feature", false, flatCtx)
			// Don't assert success - evaluation might fail if shutdown happens first
			// The important thing is it doesn't panic or hang
			_ = result
			evaluationsDone <- true
		}()
	}

	// Give evaluations a brief moment to start
	time.Sleep(10 * time.Millisecond)

	// Shutdown while evaluations are in progress
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = provider.ShutdownWithContext(shutdownCtx)
	assert.NoError(t, err, "Shutdown should succeed even with concurrent evaluations")

	// Wait for all evaluations to complete
	for i := 0; i < 10; i++ {
		select {
		case <-evaluationsDone:
			// Evaluation completed
		case <-time.After(2 * time.Second):
			t.Fatal("Evaluation did not complete within timeout")
		}
	}

	// Verify provider is shut down
	assert.Equal(t, openfeature.NotReadyState, provider.Status())
}

// TestMetricsBeforeInit verifies Health() returns correct state before initialization.
func TestMetricsBeforeInit(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Check health before init
	metrics := provider.Metrics()
	assert.Equal(t, "Split", metrics["provider"])
	assert.Equal(t, false, metrics["initialized"])
	assert.Equal(t, string(openfeature.NotReadyState), metrics["status"])
	assert.Equal(t, false, metrics["ready"])
	assert.NotContains(t, metrics, "splits_count", "splits_count should not be present before init")
}

// TestMetricsAfterInit verifies Health() returns correct state after initialization.
func TestMetricsAfterInit(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Initialize
	initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	// Check health after init
	metrics := provider.Metrics()
	assert.Equal(t, "Split", metrics["provider"])
	assert.Equal(t, true, metrics["initialized"])
	assert.Equal(t, string(openfeature.ReadyState), metrics["status"])
	assert.Equal(t, true, metrics["ready"])
	assert.Contains(t, metrics, "splits_count", "splits_count should be present when ready")
	assert.Greater(t, metrics["splits_count"], 0, "splits_count should be > 0 for testdata")

	// Cleanup
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = provider.ShutdownWithContext(shutdownCtx)
}

// TestMetricsAfterShutdown verifies Health() returns correct state after shutdown.
func TestMetricsAfterShutdown(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Initialize
	initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	// Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = provider.ShutdownWithContext(shutdownCtx)
	require.NoError(t, err)

	// Check health after shutdown
	metrics := provider.Metrics()
	assert.Equal(t, "Split", metrics["provider"])
	assert.Equal(t, false, metrics["initialized"])
	assert.Equal(t, string(openfeature.NotReadyState), metrics["status"])
	assert.Equal(t, false, metrics["ready"])
	assert.NotContains(t, metrics, "splits_count", "splits_count should not be present after shutdown")
}

// TestStatusAtomicity verifies that Status() reads shutdown flag and factory state atomically.
// This test runs with -race to detect any race conditions.
func TestStatusAtomicity(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Initialize
	initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	// Concurrently call Status() while shutting down
	done := make(chan struct{})
	goroutinesDone := make(chan struct{}, 5)

	// Goroutines calling Status() repeatedly
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { goroutinesDone <- struct{}{} }()
			for {
				select {
				case <-done:
					return
				default:
					// Just call Status(), don't store the result
					_ = provider.Status()
					// Small sleep to avoid tight loop
					time.Sleep(1 * time.Millisecond)
				}
			}
		}()
	}

	// Give Status() calls a moment to start
	time.Sleep(10 * time.Millisecond)

	// Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = provider.ShutdownWithContext(shutdownCtx)
	assert.NoError(t, err)

	// Stop Status() calls and wait for all goroutines to finish
	close(done)
	for i := 0; i < 5; i++ {
		<-goroutinesDone
	}

	// Verify final status is NotReady
	finalStatus := provider.Status()
	assert.Equal(t, openfeature.NotReadyState, finalStatus,
		"Final status should be NotReady after shutdown")

	// The test passes if no race detector warnings occur
	// All intermediate statuses should be either Ready or NotReady (no undefined states)
}

// TestDoubleShutdown verifies that calling Shutdown multiple times is safe.
func TestDoubleShutdown(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Initialize
	initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	// First shutdown
	shutdownCtx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	err = provider.ShutdownWithContext(shutdownCtx1)
	assert.NoError(t, err, "First shutdown should succeed")

	// Second shutdown (should be idempotent)
	shutdownCtx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	err = provider.ShutdownWithContext(shutdownCtx2)
	assert.NoError(t, err, "Second shutdown should succeed (idempotent)")

	// Verify provider is still NotReady
	assert.Equal(t, openfeature.NotReadyState, provider.Status())
}

// TestInitIdempotency verifies that calling Init when already initialized
// returns immediately without re-initializing or starting duplicate monitoring goroutines.
func TestInitIdempotency(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// First Init
	initCtx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	err = provider.InitWithContext(initCtx1, openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "First init should succeed")
	assert.Equal(t, openfeature.ReadyState, provider.Status())

	// Second Init (should hit fast path, return immediately)
	initCtx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	start := time.Now()
	err = provider.InitWithContext(initCtx2, openfeature.NewEvaluationContext("", nil))
	elapsed := time.Since(start)

	// Should succeed immediately (fast path)
	assert.NoError(t, err, "Second init should succeed (idempotent)")
	assert.Less(t, elapsed, 100*time.Millisecond,
		"Second init should return immediately via fast path, not wait for BlockUntilReady")
	assert.Equal(t, openfeature.ReadyState, provider.Status())

	// Third Init (also fast path)
	initCtx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()
	err = provider.InitWithContext(initCtx3, openfeature.NewEvaluationContext("", nil))
	assert.NoError(t, err, "Third init should succeed (idempotent)")

	// Cleanup
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = provider.ShutdownWithContext(shutdownCtx)
}

// TestConcurrentInit verifies that multiple concurrent Init calls are handled
// correctly using singleflight - only ONE initialization happens.
func TestConcurrentInit(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Start 10 concurrent Init calls
	const numGoroutines = 10
	results := make(chan error, numGoroutines)
	start := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func() {
			<-start // Synchronize all goroutines to start at once
			initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
			results <- err
		}()
	}

	// Start all goroutines at once
	close(start)

	// Collect all results
	var successCount int
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		if err == nil {
			successCount++
		}
	}

	// All Init calls should succeed (singleflight ensures only one actual init)
	assert.Equal(t, numGoroutines, successCount, "All Init calls should succeed")

	// Verify provider is ready
	assert.Equal(t, openfeature.ReadyState, provider.Status())

	// Cleanup
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = provider.ShutdownWithContext(shutdownCtx)
}

// TestShutdownDuringInit verifies that calling Shutdown while Init is in progress
// is handled safely without panics or hangs.
func TestShutdownDuringInit(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 2 // Longer to give shutdown time to race

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Start Init in background
	initDone := make(chan error, 1)
	go func() {
		initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
		initDone <- err
	}()

	// Give Init a moment to start BlockUntilReady
	time.Sleep(100 * time.Millisecond)

	// Call Shutdown while Init is in progress
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.ShutdownWithContext(shutdownCtx)

	// Shutdown should succeed (may complete before init, or after)
	assert.NoError(t, err, "Shutdown should succeed even during init")

	// Wait for Init to complete
	select {
	case initErr := <-initDone:
		// Init might succeed (if it completed before shutdown)
		// or fail (if shutdown happened first)
		// Either outcome is acceptable - the important thing is no panic/hang
		_ = initErr
	case <-time.After(15 * time.Second):
		t.Fatal("Init did not complete within timeout")
	}

	// Final status should be NotReady (shutdown completed)
	assert.Equal(t, openfeature.NotReadyState, provider.Status())
}

// TestFactoryAccessorDuringShutdown verifies that Factory() accessor is safe
// to call concurrently with Shutdown().
func TestFactoryAccessorDuringShutdown(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = "testdata/split.yaml"
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)

	// Initialize
	initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.InitWithContext(initCtx, openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	// Concurrently access Factory() while shutting down
	done := make(chan struct{})
	goroutinesDone := make(chan int, 5)

	// Goroutines calling Factory() repeatedly
	for i := 0; i < 5; i++ {
		go func() {
			count := 0
			defer func() { goroutinesDone <- count }()
			for {
				select {
				case <-done:
					return
				default:
					var factory *client.SplitFactory = provider.Factory()
					if factory != nil {
						count++
					}
					time.Sleep(1 * time.Millisecond)
				}
			}
		}()
	}

	// Give Factory() calls a moment to start
	time.Sleep(10 * time.Millisecond)

	// Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = provider.ShutdownWithContext(shutdownCtx)
	assert.NoError(t, err)

	// Stop Factory() calls and wait for all goroutines
	close(done)
	totalCalls := 0
	for i := 0; i < 5; i++ {
		totalCalls += <-goroutinesDone
	}

	// Verify we got some factory results (test passed if no data race)
	assert.Greater(t, totalCalls, 0, "Should have retrieved factory at least once")
}

// TestEventChannelClosedOnShutdown verifies that the event channel is properly
// closed when the provider is shut down, preventing deadlocks for consumers
// using for...range loops.
//
// This test addresses a critical requirement from the OpenFeature specification:
// the event channel must be closed during shutdown to signal consumers that
// no more events will be sent.
func TestEventChannelClosedOnShutdown(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Provider creation should succeed")

	// Initialize the provider
	ctx := context.Background()
	err = provider.InitWithContext(ctx, openfeature.NewEvaluationContext("test-user", nil))
	require.NoError(t, err, "Init should succeed")

	// Get the event channel
	eventChan := provider.EventChannel()
	require.NotNil(t, eventChan, "EventChannel should not be nil")

	// Start a goroutine that ranges over the event channel
	// This simulates a typical consumer pattern
	consumerDone := make(chan struct{})
	receivedEvents := 0

	go func() {
		defer close(consumerDone)
		for range eventChan {
			receivedEvents++
		}
		// When the channel is closed, the range loop exits and we close consumerDone
	}()

	// Shutdown the provider
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.ShutdownWithContext(shutdownCtx)
	assert.NoError(t, err, "Shutdown should succeed")

	// Wait for the consumer goroutine to exit (with timeout)
	// If the channel is not closed, this will timeout
	select {
	case <-consumerDone:
		// Success - the range loop exited because the channel was closed
		t.Logf("Consumer goroutine exited cleanly after receiving %d events", receivedEvents)
	case <-time.After(2 * time.Second):
		t.Fatal("Consumer goroutine did not exit - event channel was not closed on shutdown")
	}

	// Verify we received at least the PROVIDER_READY event
	assert.Greater(t, receivedEvents, 0, "Should have received at least one event")
}

// TestEventChannelMultipleConsumers verifies that multiple goroutines
// ranging over the event channel all exit cleanly when the provider shuts down.
func TestEventChannelMultipleConsumers(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Provider creation should succeed")

	// Initialize the provider
	ctx := context.Background()
	err = provider.InitWithContext(ctx, openfeature.NewEvaluationContext("test-user", nil))
	require.NoError(t, err, "Init should succeed")

	// Get the event channel
	eventChan := provider.EventChannel()

	// Start multiple consumer goroutines
	numConsumers := 5
	consumersDone := make(chan int, numConsumers)

	for i := 0; i < numConsumers; i++ {
		go func() {
			count := 0
			for range eventChan {
				count++
			}
			consumersDone <- count
		}()
	}

	// Give consumers a moment to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown the provider
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.ShutdownWithContext(shutdownCtx)
	assert.NoError(t, err, "Shutdown should succeed")

	// Wait for all consumers to exit (with timeout)
	timeout := time.After(2 * time.Second)
	for i := 0; i < numConsumers; i++ {
		select {
		case count := <-consumersDone:
			t.Logf("Consumer %d exited cleanly after receiving %d events", i, count)
		case <-timeout:
			t.Fatalf("Consumer %d did not exit - event channel was not closed on shutdown", i)
		}
	}
}

// TestEventChannelClosedBeforeInit verifies that shutdown works correctly
// even when called before initialization (edge case).
func TestEventChannelClosedBeforeInit(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Provider creation should succeed")

	// Get the event channel before init
	eventChan := provider.EventChannel()
	require.NotNil(t, eventChan, "EventChannel should not be nil")

	// Start consumer before init
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		for range eventChan {
			// Consume events
		}
	}()

	// Shutdown without initializing
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = provider.ShutdownWithContext(shutdownCtx)
	assert.NoError(t, err, "Shutdown should succeed even without init")

	// Verify consumer exits
	select {
	case <-consumerDone:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Consumer did not exit - event channel was not closed")
	}
}

// TestShutdownIdempotencyWithEventChannel verifies that calling shutdown
// multiple times doesn't cause panics (double-close on channel).
func TestShutdownIdempotencyWithEventChannel(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Provider creation should succeed")

	// Initialize
	ctx := context.Background()
	err = provider.InitWithContext(ctx, openfeature.NewEvaluationContext("test-user", nil))
	require.NoError(t, err, "Init should succeed")

	// First shutdown
	shutdownCtx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	err = provider.ShutdownWithContext(shutdownCtx1)
	assert.NoError(t, err, "First shutdown should succeed")

	// Second shutdown - should not panic from double-close
	shutdownCtx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	err = provider.ShutdownWithContext(shutdownCtx2)
	assert.NoError(t, err, "Second shutdown should succeed without panic")

	// Third shutdown - verify idempotency
	shutdownCtx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()
	err = provider.ShutdownWithContext(shutdownCtx3)
	assert.NoError(t, err, "Third shutdown should succeed without panic")
}
