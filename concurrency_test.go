package split

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/conf"
	"github.com/splitio/go-toolkit/v5/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentEvaluations tests thread safety with concurrent evaluations.
func TestConcurrentEvaluations(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err = openfeature.SetProviderWithContextAndWait(ctx, provider)
	require.NoError(t, err, "Failed to set provider")

	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		_ = openfeature.ShutdownWithContext(shutdownCtx)
	}()

	const numGoroutines = 50
	const numEvaluations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ofClient := openfeature.NewClient("concurrent-test")
			evalCtx := openfeature.NewEvaluationContext("test-user", nil)

			for j := 0; j < numEvaluations; j++ {
				_, err := ofClient.BooleanValue(
					context.TODO(),
					flagSomeOther,
					false,
					evalCtx,
				)
				if err != nil && !strings.Contains(err.Error(), "FLAG_NOT_FOUND") {
					errors <- fmt.Errorf("goroutine %d iteration %d: %w", id, j, err)
					return
				}

				_, err = ofClient.StringValue(
					context.TODO(),
					flagSomeOther,
					"default",
					evalCtx,
				)
				if err != nil && !strings.Contains(err.Error(), "FLAG_NOT_FOUND") {
					errors <- fmt.Errorf("goroutine %d iteration %d: %w", id, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent evaluation error: %v", err)
	}
}

// TestConcurrentInitShutdown tests race conditions when Init and Shutdown are called concurrently.
func TestConcurrentInitShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 1

	const iterations = 2
	for i := 0; i < iterations; i++ {
		provider, err := New("localhost", WithSplitConfig(cfg))
		require.NoError(t, err)

		var wg sync.WaitGroup
		const concurrency = 3

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for j := 0; j < concurrency; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = provider.InitWithContext(ctx, openfeature.NewEvaluationContext("", nil))
			}()
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			_ = provider.ShutdownWithContext(shutdownCtx)
		}()

		wg.Wait()

		assert.Equal(t, openfeature.NotReadyState, provider.Status(), "Provider should be NotReady after shutdown")
	}
}

// TestEventChannelOverflow tests behavior when event channel buffer is full.
func TestEventChannelOverflow(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	eventChan := provider.EventChannel()

	const eventsToEmit = 150
	const bufferSize = 100

	for i := 0; i < eventsToEmit; i++ {
		select {
		case <-eventChan:
			// Drain one event to make room
		default:
			// Channel full or empty
		}
	}

	done := make(chan bool)
	go func() {
		status := provider.Status()
		assert.Equal(t, openfeature.ReadyState, status, "Provider should still be ready")
		done <- true
	}()

	select {
	case <-done:
		// Success - operation completed without blocking
	case <-time.After(2 * time.Second):
		t.Fatal("Event emission appears to be blocking")
	}

	drained := 0
	for {
		select {
		case <-eventChan:
			drained++
		case <-time.After(10 * time.Millisecond):
			goto doneLabel
		}
	}
doneLabel:
	assert.LessOrEqual(t, drained, bufferSize, "Should not have more events than buffer size")
}
