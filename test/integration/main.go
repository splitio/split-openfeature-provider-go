// Package main is a comprehensive integration test suite for the Split OpenFeature Provider.
//
// This test suite validates ALL provider functionality and serves as both
// integration testing and a reference implementation. It demonstrates:
//
//   - Custom Split SDK configuration
//   - Structured logging with slog
//   - Event handling (PROVIDER_READY, PROVIDER_ERROR, PROVIDER_CONFIGURATION_CHANGED)
//   - Graceful shutdown with context cancellation
//   - All evaluation types (boolean, string, int, float, object)
//   - Evaluation details (variant, reason)
//   - Targeting with attributes
//   - Context cancellation and timeout handling
//   - Flag metadata (configurations attached to flags)
//   - Flag set evaluation (cloud mode only)
//   - Direct Split SDK access (Track, Treatments)
//   - Concurrent evaluations (100 goroutines x 10 evaluations)
//   - Comprehensive error handling
//
// This test suite supports both localhost mode and real Split API keys:
//
//	Run with localhost mode: go run .
//	Run with Split API key: SPLIT_API_KEY=your-key-here go run .
//
// Exit codes:
//   - 0: All tests passed
//   - 1: One or more tests failed
//   - 2: Timeout or fatal error
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lmittmann/tint"
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/open-feature/go-sdk/openfeature/hooks"
	"github.com/splitio/go-client/v6/splitio/conf"

	"github.com/splitio/split-openfeature-provider-go/v2"
)

func main() {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("   Split OpenFeature Provider - Integration Test Suite")
	fmt.Println("   Comprehensive Validation & Reference Implementation")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// ============================================================
	// SETUP: CONTEXT WITH TIMEOUT AND SIGNAL HANDLING
	// ============================================================

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var (
		cleanupSuccess = true
		exitCode       = 0
	)

	// ============================================================
	// 1. LOGGING CONFIGURATION (with colored output via tint)
	// ============================================================

	logLevel := slog.LevelInfo
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		switch level {
		case "debug", "DEBUG", "trace", "TRACE":
			logLevel = slog.LevelDebug
		case "info", "INFO":
			logLevel = slog.LevelInfo
		case "warn", "WARN", "warning", "WARNING":
			logLevel = slog.LevelWarn
		case "error", "ERROR":
			logLevel = slog.LevelError
		default:
			logLevel = slog.LevelInfo
		}
	}

	baseLogger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      logLevel,
		TimeFormat: time.TimeOnly,
	}))

	appLogger := baseLogger.With("source", "app")
	ofLogger := baseLogger.With("source", "openfeature-sdk")

	slog.SetDefault(baseLogger)

	section("LOGGING CONFIGURATION")
	appLogger.Info("logging configured", "format", "tint (colored)", "level", logLevel.String())

	// ============================================================
	// 2. OPENFEATURE LOGGING HOOK (must be first to capture all evaluations)
	// ============================================================
	section("OPENFEATURE LOGGING HOOK")
	openfeature.AddHooks(hooks.NewLoggingHook(false, ofLogger))
	appLogger.Info("logging hook added (captures all flag evaluations)")

	// ============================================================
	// 3. EVENT HANDLERS (API-level handlers run before client handlers)
	// ============================================================
	section("EVENT HANDLERS")

	var eventsReceived sync.Map

	handleEvent := func(eventType openfeature.EventType) openfeature.EventCallback {
		callback := func(details openfeature.EventDetails) {
			val, _ := eventsReceived.LoadOrStore(eventType, new(atomic.Int64))
			counter := val.(*atomic.Int64)
			count := counter.Add(1)

			slog.Info("event received",
				"type", eventType,
				"provider", details.ProviderName,
				"message", details.Message,
				"count", count)
		}
		return &callback
	}

	openfeature.AddHandler(openfeature.ProviderReady, handleEvent(openfeature.ProviderReady))
	openfeature.AddHandler(openfeature.ProviderError, handleEvent(openfeature.ProviderError))
	openfeature.AddHandler(openfeature.ProviderConfigChange, handleEvent(openfeature.ProviderConfigChange))

	appLogger.Info("event handlers registered", "handlers", 3)

	// ============================================================
	// 4. SPLIT SDK CONFIGURATION
	// ============================================================
	section("SPLIT SDK CONFIGURATION")

	apiKey := os.Getenv("SPLIT_API_KEY")
	if apiKey == "" {
		apiKey = "localhost"
		appLogger.Info("no SPLIT_API_KEY provided, using localhost mode")
	} else {
		appLogger.Info("using Split API key from environment")
	}

	cfg := conf.Default()
	cfg.BlockUntilReady = 10

	if apiKey == "localhost" {
		cfg.SplitFile = "./split.yaml"
		appLogger.Info("split SDK configured",
			"mode", "localhost",
			"file", "./split.yaml",
			"timeout", "10s")
	} else {
		appLogger.Info("split SDK configured",
			"mode", "cloud",
			"timeout", "10s")
	}

	// ============================================================
	// 5. PROVIDER CREATION
	// ============================================================
	section("PROVIDER CREATION")

	provider, err := split.New(apiKey,
		split.WithLogger(baseLogger),
		split.WithSplitConfig(cfg),
	)
	if err != nil {
		slog.Error("failed to create provider", "error", err)
		os.Exit(2)
	}

	appLogger.Info("provider created with unified logging")

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic during shutdown", "panic", r)
					cleanupSuccess = false
				}
			}()

			fmt.Println()
			fmt.Println(strings.Repeat("─", 60))
			slog.Info("initiating graceful shutdown")

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := openfeature.ShutdownWithContext(shutdownCtx); err != nil {
				slog.Error("shutdown error", "error", err)
				cleanupSuccess = false
			}

			slog.Info("graceful shutdown complete")
		})
	}

	defer cleanup()

	shutdownChan := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-shutdownChan:
			slog.Warn("interrupt signal received", "signal", sig)
			signal.Stop(shutdownChan)
			cancel()
		case <-done:
			signal.Stop(shutdownChan)
			return
		}
	}()

	defer close(done)

	// ============================================================
	// 6. PROVIDER INITIALIZATION
	// ============================================================
	section("PROVIDER INITIALIZATION")

	initCtx, initCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer initCancel()

	if err := openfeature.SetProviderWithContextAndWait(initCtx, provider); err != nil {
		slog.Error("failed to initialize provider", "error", err)
		cleanup()
		os.Exit(2)
	}

	appLogger.Info("provider initialized and ready")

	// ============================================================
	// 7. OPENFEATURE CLIENT CREATION
	// ============================================================
	section("CLIENT CREATION")

	ofClient := openfeature.NewDefaultClient()

	appLogger.Info("OpenFeature client created")

	// ============================================================
	// RUN ALL TESTS
	// ============================================================
	section("RUNNING TESTS")
	runTests(ctx, ofClient, provider, &eventsReceived, apiKey, baseLogger, cfg)

	// ============================================================
	// RESULTS SUMMARY
	// ============================================================
	results.Summary()

	// Print event statistics
	fmt.Println()
	fmt.Println("Event Statistics:")
	eventsReceived.Range(func(key, value any) bool {
		eventType := key.(openfeature.EventType)
		counter := value.(*atomic.Int64)
		count := counter.Load()
		fmt.Printf("  %s: %d events\n", eventType, count)
		return true
	})

	close(done)
	cleanup()

	if !cleanupSuccess {
		exitCode = 2
	} else if results.total.Load() == 0 {
		exitCode = 2
	} else if results.failed.Load() > 0 {
		exitCode = 1
	}

	os.Exit(exitCode)
}

// runTests executes all integration tests with the provided context.
func runTests(ctx context.Context, client *openfeature.Client, provider *split.Provider, eventsReceived *sync.Map, apiKey string, baseLogger *slog.Logger, cfg *conf.SplitSdkConfig) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic during test execution", "panic", r)
			results.Fail("panic", fmt.Sprintf("test execution panicked: %v", r))
		}
	}()

	isLocalhostMode := apiKey == "localhost"

	// ============================================================
	// FLAG EVALUATION TESTS
	// ============================================================

	section("BOOLEAN FLAG EVALUATIONS")
	testBooleanEvaluations(ctx, client)

	section("STRING FLAG EVALUATIONS")
	testStringEvaluations(ctx, client)

	section("INTEGER FLAG EVALUATIONS")
	testIntEvaluations(ctx, client)

	section("FLOAT FLAG EVALUATIONS")
	testFloatEvaluations(ctx, client)

	// Object evaluations only work in localhost mode (cloud mode only evaluates flag sets)
	if isLocalhostMode {
		section("OBJECT FLAG EVALUATIONS")
		testObjectEvaluations(ctx, client)
	} else {
		section("OBJECT FLAG EVALUATIONS (SKIPPED - cloud mode)")
		slog.Info("skipping object evaluations - cloud mode only evaluates flag sets")
	}

	section("EVALUATION DETAILS")
	testEvaluationDetails(ctx, client)

	// Flag metadata tests run in both modes - tests that metadata field is properly populated
	// In localhost mode, flags have JSON configs attached
	// In cloud mode, flags may or may not have configs (test handles both cases)
	section("FLAG METADATA")
	testFlagMetadata(ctx, client)

	// Flag set evaluation only works in cloud mode (localhost doesn't support flag sets)
	if !isLocalhostMode {
		section("FLAG SET EVALUATION")
		testFlagSetEvaluation(ctx, client)
	} else {
		section("FLAG SET EVALUATION (SKIPPED - localhost mode)")
		slog.Info("skipping flag set evaluation - localhost mode doesn't support flag sets")
	}

	section("TARGETING WITH ATTRIBUTES")
	testAttributeTargeting(ctx, client)

	section("CONTEXT CANCELLATION")
	testContextCancellation(client)

	section("ERROR HANDLING")
	testErrorHandling(ctx, client)

	// ============================================================
	// ADVANCED TESTS (SDK access, concurrency, metrics)
	// ============================================================

	section("DIRECT SPLIT SDK ACCESS")
	testDirectSDKAccess(provider)

	section("CONCURRENT EVALUATIONS")
	testConcurrentEvaluations(ctx, client)

	section("PROVIDER STATUS & HEALTH")
	testProviderHealth(provider)

	section("EVENT TRACKING VALIDATION")
	testEventTracking(eventsReceived)

	section("METRICS BEFORE INIT")
	testMetricsBeforeInit()

	section("METRICS ALL FIELDS")
	testMetricsAllFields(provider)

	// ============================================================
	// LIFECYCLE TESTS (init, shutdown, named providers)
	// ============================================================

	section("INIT AFTER SHUTDOWN")
	testInitAfterShutdown(apiKey, baseLogger, cfg)

	section("NAMED PROVIDER SUPPORT")
	testNamedProvider(ctx, apiKey, baseLogger, cfg)

	section("CONCURRENT INIT CALLS")
	testConcurrentInit(ctx, apiKey, baseLogger, cfg)

	section("PROVIDER_NOT_READY ERROR")
	testProviderNotReadyError()

	section("METADATA & HOOKS")
	testTrivialGetters(provider)

	section("INIT TIMEOUT")
	testInitWithContextTimeout()

	section("SHUTDOWN TIMEOUT")
	testShutdownWithContextTimeout(apiKey, baseLogger, cfg)

	section("SHUTDOWN DURING INIT")
	testShutdownDuringInit(apiKey, baseLogger, cfg)

	section("METRICS AFTER SHUTDOWN")
	testMetricsAfterShutdown(apiKey, baseLogger, cfg)

	section("NIL CONFIG DEFAULTS")
	testProviderWithNilConfig(apiKey, baseLogger)

	section("BLOCKUNTILREADY ZERO")
	testBlockUntilReadyZero(apiKey, baseLogger)

	section("STATUS ATOMICITY")
	testStatusAtomicity(apiKey, baseLogger, cfg)

	section("DOUBLE SHUTDOWN")
	testDoubleShutdown(apiKey, baseLogger, cfg)
}
