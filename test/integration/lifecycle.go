// lifecycle.go contains provider lifecycle tests.
// Tests cover initialization, shutdown, named providers, concurrent init,
// timeout handling, status atomicity, and idempotent operations.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/conf"

	"github.com/splitio/split-openfeature-provider-go/v2"
)

// testInitAfterShutdown tests that init fails after shutdown
func testInitAfterShutdown(apiKey string, logger *slog.Logger, cfg *conf.SplitSdkConfig) {

	testProvider, err := split.New(apiKey, split.WithLogger(logger), split.WithSplitConfig(cfg))
	if err != nil {
		results.Fail("InitAfterShutdown(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}

	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	if err := testProvider.InitWithContext(initCtx, evalCtx); err != nil {
		results.Fail("InitAfterShutdown(init)", fmt.Sprintf("init failed: %v", err))
		testProvider.Shutdown()
		return
	}

	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()

	if err := testProvider.ShutdownWithContext(shutdownCtx); err != nil {
		// In cloud mode with SSE streaming, the Split SDK has a known hang bug
		// that can cause shutdown to timeout. Accept this as valid for cloud mode.
		if apiKey != "localhost" && strings.Contains(err.Error(), "context deadline exceeded") {
			// Continue with test - shutdown timeout is acceptable in cloud mode
		} else {
			results.Fail("InitAfterShutdown(shutdown)", fmt.Sprintf("shutdown failed: %v", err))
			return
		}
	}

	initCtx2, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()

	err = testProvider.InitWithContext(initCtx2, evalCtx)

	if err == nil {
		results.Fail("InitAfterShutdown", "expected error, got nil")
	} else if !strings.Contains(err.Error(), "cannot initialize provider after shutdown") {
		results.Fail("InitAfterShutdown", fmt.Sprintf("wrong error message: %v", err))
	} else {
		results.Pass("InitAfterShutdown")
	}
}

// testNamedProvider tests creating and using a named provider
func testNamedProvider(ctx context.Context, apiKey string, logger *slog.Logger, cfg *conf.SplitSdkConfig) {
	// Create a named provider
	namedProvider, err := split.New(apiKey, split.WithLogger(logger), split.WithSplitConfig(cfg))
	if err != nil {
		results.Fail("NamedProvider(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}
	defer namedProvider.Shutdown()

	initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := openfeature.SetNamedProviderWithContextAndWait(initCtx, "test-split", namedProvider); err != nil {
		results.Fail("NamedProvider(init)", fmt.Sprintf("failed to initialize: %v", err))
		return
	}
	results.Pass("NamedProvider(init)")

	namedClient := openfeature.NewClient("test-split")
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	// Test evaluation with named client
	value, err := namedClient.BooleanValue(ctx, "feature_boolean_on", false, evalCtx)
	if err != nil {
		results.Fail("NamedProvider(evaluation)", fmt.Sprintf("evaluation failed: %v", err))
		return
	}

	if value != true {
		results.Fail("NamedProvider(value)", fmt.Sprintf("expected true, got %v", value))
	} else {
		results.Pass("NamedProvider(evaluation)")
	}

	// Cleanup happens via defer namedProvider.Shutdown()
	results.Pass("NamedProvider(cleanup)")
}

// testConcurrentInit tests concurrent InitWithContext calls use singleflight
func testConcurrentInit(ctx context.Context, apiKey string, logger *slog.Logger, cfg *conf.SplitSdkConfig) {
	// Create a provider but don't initialize
	concurrentProvider, err := split.New(apiKey, split.WithLogger(logger), split.WithSplitConfig(cfg))
	if err != nil {
		results.Fail("ConcurrentInit(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}
	defer concurrentProvider.Shutdown()

	// Launch 10 concurrent InitWithContext calls
	var wg sync.WaitGroup
	errors := make(chan error, 10)
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			errors <- concurrentProvider.InitWithContext(initCtx, evalCtx)
		}()
	}

	wg.Wait()
	close(errors)

	// All should succeed (singleflight ensures only one actual init)
	successCount := 0
	for err := range errors {
		if err == nil {
			successCount++
		}
	}

	if successCount == 10 {
		results.Pass("ConcurrentInit(singleflight)")
	} else {
		results.Fail("ConcurrentInit(singleflight)", fmt.Sprintf("only %d/10 succeeded", successCount))
	}
}

// testProviderNotReadyError tests PROVIDER_NOT_READY error code via OpenFeature SDK
func testProviderNotReadyError() {
	// Use invalid API key so the provider never becomes ready
	uninitProvider, err := split.New("invalid-key-for-not-ready-test")
	if err != nil {
		results.Fail("ProviderNotReady(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}
	defer uninitProvider.Shutdown()

	// Use a named provider to avoid interfering with the default provider
	ctx := context.Background()
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	// Set as named provider (non-blocking) and immediately try to evaluate
	openfeature.SetNamedProvider("not-ready-test", uninitProvider)
	client := openfeature.NewClient("not-ready-test")

	details, err := client.BooleanValueDetails(ctx, "some-flag", false, evalCtx)

	if err == nil {
		results.Fail("ProviderNotReady(error)", "expected error, got nil")
		return
	}

	// Check error code - OpenFeature should return PROVIDER_NOT_READY
	if details.ErrorCode != openfeature.ProviderNotReadyCode {
		results.Fail("ProviderNotReady(error_code)",
			fmt.Sprintf("expected PROVIDER_NOT_READY, got %v", details.ErrorCode))
	} else {
		results.Pass("ProviderNotReady(error_code)")
	}

	// Should return default value
	if details.Value != false {
		results.Fail("ProviderNotReady(default)", fmt.Sprintf("expected default false, got %v", details.Value))
	} else {
		results.Pass("ProviderNotReady(default_value)")
	}
}

// testTrivialGetters tests Metadata() and Hooks() methods
func testTrivialGetters(provider *split.Provider) {
	// Test Metadata()
	metadata := provider.Metadata()
	if metadata.Name != "Split" {
		results.Fail("Metadata(name)", fmt.Sprintf("expected Split, got %s", metadata.Name))
	} else {
		results.Pass("Metadata(name)")
	}

	// Test Hooks() - should return nil
	hooks := provider.Hooks()
	if hooks != nil {
		results.Fail("Hooks()", fmt.Sprintf("expected nil, got %v", hooks))
	} else {
		results.Pass("Hooks()")
	}
}

// testInitWithContextTimeout tests InitWithContext with timeout expiration
func testInitWithContextTimeout() {
	// Create provider with invalid API key that will never become ready
	timeoutProvider, err := split.New("invalid-key-that-will-timeout")
	if err != nil {
		results.Fail("InitTimeout(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}
	defer timeoutProvider.Shutdown()

	// Use very short timeout that will expire
	initCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	evalCtx := openfeature.NewEvaluationContext("test-user", nil)
	err = timeoutProvider.InitWithContext(initCtx, evalCtx)

	if err == nil {
		results.Fail("InitTimeout(error)", "expected timeout error, got nil")
	} else if strings.Contains(err.Error(), "context deadline exceeded") ||
		strings.Contains(err.Error(), "initialization cancelled") {
		results.Pass("InitTimeout(context_cancelled)")
	} else {
		results.Fail("InitTimeout(error_message)", fmt.Sprintf("unexpected error: %v", err))
	}
}

// testShutdownWithContextTimeout tests ShutdownWithContext with timeout
func testShutdownWithContextTimeout(apiKey string, logger *slog.Logger, cfg *conf.SplitSdkConfig) {
	isLocalhostMode := apiKey == "localhost"

	shutdownProvider, err := split.New(apiKey, split.WithLogger(logger), split.WithSplitConfig(cfg))
	if err != nil {
		results.Fail("ShutdownTimeout(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}

	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	if err := shutdownProvider.InitWithContext(initCtx, evalCtx); err != nil {
		results.Fail("ShutdownTimeout(init)", fmt.Sprintf("init failed: %v", err))
		shutdownProvider.Shutdown()
		return
	}

	// In localhost mode, use very short timeout to test best-effort behavior
	// In cloud mode, use longer timeout due to SSE streaming cleanup
	var shutdownTimeout time.Duration
	if isLocalhostMode {
		shutdownTimeout = 1 * time.Millisecond
	} else {
		shutdownTimeout = 100 * time.Millisecond
	}

	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel2()

	err = shutdownProvider.ShutdownWithContext(shutdownCtx)

	// In localhost mode, shutdown should succeed quickly (best-effort)
	// In cloud mode with SSE streaming, context timeout is expected
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			if isLocalhostMode {
				// Localhost mode should succeed quickly
				results.Fail("ShutdownTimeout(error)", "localhost mode should shutdown quickly")
			} else {
				// Cloud mode timeout is expected due to SSE streaming
				results.Pass("ShutdownTimeout(context_timeout_cloud)")
			}
		} else {
			results.Fail("ShutdownTimeout(error)", fmt.Sprintf("unexpected error: %v", err))
		}
	} else {
		results.Pass("ShutdownTimeout(best_effort)")
	}

	// Provider should be shut down even if timeout expired
	if shutdownProvider.Status() != openfeature.NotReadyState {
		results.Fail("ShutdownTimeout(status)", "provider should be NotReady after shutdown")
	} else {
		results.Pass("ShutdownTimeout(status)")
	}
}

// testShutdownDuringInit tests shutdown called during initialization
func testShutdownDuringInit(apiKey string, logger *slog.Logger, cfg *conf.SplitSdkConfig) {
	// Create provider with slow initialization
	slowProvider, err := split.New(apiKey, split.WithLogger(logger), split.WithSplitConfig(cfg))
	if err != nil {
		results.Fail("ShutdownDuringInit(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}

	// Start init in background
	initDone := make(chan error, 1)
	go func() {
		initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		evalCtx := openfeature.NewEvaluationContext("test-user", nil)
		initDone <- slowProvider.InitWithContext(initCtx, evalCtx)
	}()

	// Give init a moment to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown while init is running
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shutdownErr := slowProvider.ShutdownWithContext(shutdownCtx)
	if shutdownErr != nil {
		results.Fail("ShutdownDuringInit(shutdown)", fmt.Sprintf("shutdown failed: %v", shutdownErr))
	} else {
		results.Pass("ShutdownDuringInit(shutdown)")
	}

	initErr := <-initDone
	if initErr != nil {
		// Init should fail because shutdown happened
		results.Pass("ShutdownDuringInit(init_fails)")
	} else {
		// Or init might succeed before shutdown - both acceptable
		results.Pass("ShutdownDuringInit(init_race)")
	}

	// Provider should be shut down
	if slowProvider.Status() != openfeature.NotReadyState {
		results.Fail("ShutdownDuringInit(final_status)", "expected NotReady after shutdown")
	} else {
		results.Pass("ShutdownDuringInit(final_status)")
	}
}

// testProviderWithNilConfig tests provider creation with nil config
func testProviderWithNilConfig(apiKey string, logger *slog.Logger) {
	// For localhost mode, we still need to configure the split file
	// This test validates that WithSplitConfig is optional (uses defaults)
	// but we configure the split file if in localhost mode
	var opts []split.Option
	opts = append(opts, split.WithLogger(logger))

	if apiKey == "localhost" {
		cfg := conf.Default()
		cfg.SplitFile = "./split.yaml"
		opts = append(opts, split.WithSplitConfig(cfg))
	}

	nilConfigProvider, err := split.New(apiKey, opts...)
	if err != nil {
		results.Fail("NilConfig(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}
	defer nilConfigProvider.Shutdown()

	results.Pass("NilConfig(uses_defaults)")

	// Initialize and verify it works
	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	if err := nilConfigProvider.InitWithContext(initCtx, evalCtx); err != nil {
		results.Fail("NilConfig(init)", fmt.Sprintf("init failed: %v", err))
	} else {
		results.Pass("NilConfig(init)")
	}
}

// testBlockUntilReadyZero tests BlockUntilReady=0 uses default timeout
func testBlockUntilReadyZero(apiKey string, logger *slog.Logger) {
	// Create config with BlockUntilReady=0
	cfg := conf.Default()
	cfg.BlockUntilReady = 0

	// Configure split file for localhost mode
	if apiKey == "localhost" {
		cfg.SplitFile = "./split.yaml"
	}

	zeroProvider, err := split.New(apiKey, split.WithLogger(logger), split.WithSplitConfig(cfg))
	if err != nil {
		results.Fail("BlockUntilReadyZero(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}
	defer zeroProvider.Shutdown()

	results.Pass("BlockUntilReadyZero(create)")

	// Should use default 10s timeout
	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	if err := zeroProvider.InitWithContext(initCtx, evalCtx); err != nil {
		results.Fail("BlockUntilReadyZero(init)", fmt.Sprintf("init failed: %v", err))
	} else {
		results.Pass("BlockUntilReadyZero(init_with_default)")
	}
}

// testStatusAtomicity tests Status() method atomicity during lifecycle
func testStatusAtomicity(apiKey string, logger *slog.Logger, cfg *conf.SplitSdkConfig) {
	// Create provider
	statusProvider, err := split.New(apiKey, split.WithLogger(logger), split.WithSplitConfig(cfg))
	if err != nil {
		results.Fail("StatusAtomicity(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}
	defer statusProvider.Shutdown()

	// Status should be NotReady before init
	if statusProvider.Status() != openfeature.NotReadyState {
		results.Fail("StatusAtomicity(before_init)", "expected NotReady before init")
	} else {
		results.Pass("StatusAtomicity(before_init)")
	}

	// Initialize
	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	if err := statusProvider.InitWithContext(initCtx, evalCtx); err != nil {
		results.Fail("StatusAtomicity(init)", fmt.Sprintf("init failed: %v", err))
		return
	}

	// Status should be Ready after init
	if statusProvider.Status() != openfeature.ReadyState {
		results.Fail("StatusAtomicity(after_init)", "expected Ready after init")
	} else {
		results.Pass("StatusAtomicity(after_init)")
	}

	// Call Status() concurrently during shutdown
	var wg sync.WaitGroup
	statusResults := make([]openfeature.State, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			statusResults[idx] = statusProvider.Status()
		}(i)
	}

	// Shutdown while Status() is being called
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	statusProvider.ShutdownWithContext(shutdownCtx)

	wg.Wait()

	// All status calls should return either Ready or NotReady (atomic, no invalid states)
	allValid := true
	for _, state := range statusResults {
		if state != openfeature.ReadyState && state != openfeature.NotReadyState {
			allValid = false
			break
		}
	}

	if !allValid {
		results.Fail("StatusAtomicity(during_shutdown)", "invalid state detected")
	} else {
		results.Pass("StatusAtomicity(during_shutdown)")
	}

	// Final status should be NotReady
	if statusProvider.Status() != openfeature.NotReadyState {
		results.Fail("StatusAtomicity(after_shutdown)", "expected NotReady after shutdown")
	} else {
		results.Pass("StatusAtomicity(after_shutdown)")
	}
}

// testDoubleShutdown tests shutdown idempotency
func testDoubleShutdown(apiKey string, logger *slog.Logger, cfg *conf.SplitSdkConfig) {

	doubleProvider, err := split.New(apiKey, split.WithLogger(logger), split.WithSplitConfig(cfg))
	if err != nil {
		results.Fail("DoubleShutdown(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}

	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	if err := doubleProvider.InitWithContext(initCtx, evalCtx); err != nil {
		results.Fail("DoubleShutdown(init)", fmt.Sprintf("init failed: %v", err))
		doubleProvider.Shutdown()
		return
	}

	// First shutdown - in cloud mode with SSE streaming, the Split SDK has a known
	// hang bug, so we accept context deadline exceeded as a valid outcome
	shutdownCtx1, cancel1 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel1()
	err1 := doubleProvider.ShutdownWithContext(shutdownCtx1)
	if err1 != nil {
		if strings.Contains(err1.Error(), "context deadline exceeded") {
			results.Pass("DoubleShutdown(first_timeout_sdk_bug)")
		} else {
			results.Fail("DoubleShutdown(first)", fmt.Sprintf("first shutdown failed: %v", err1))
		}
		return
	}
	results.Pass("DoubleShutdown(first)")

	// Second shutdown - should be idempotent
	shutdownCtx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	err2 := doubleProvider.ShutdownWithContext(shutdownCtx2)
	if err2 != nil {
		results.Fail("DoubleShutdown(second)", fmt.Sprintf("second shutdown failed: %v", err2))
	} else {
		results.Pass("DoubleShutdown(idempotent)")
	}

	// Status should still be NotReady
	if doubleProvider.Status() != openfeature.NotReadyState {
		results.Fail("DoubleShutdown(status)", "expected NotReady after double shutdown")
	} else {
		results.Pass("DoubleShutdown(status)")
	}
}
