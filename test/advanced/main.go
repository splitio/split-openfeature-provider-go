// Package main provides advanced tests for cloud-only Split features.
//
// This test validates:
//   - Event tracking (view events in Split Data Hub)
//   - PROVIDER_CONFIGURATION_CHANGED event detection
//
// Prerequisites:
//   - A real Split account with SDK API key
//   - For config change test: any flag to modify in the Split dashboard
//
// Run: SPLIT_API_KEY=your-key go run main.go
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

	"github.com/splitio/split-openfeature-provider-go/v2"
)

// Event counters for validation
var (
	readyCount         atomic.Int32
	configChangedCount atomic.Int32
	errorCount         atomic.Int32
	configChangedChan  = make(chan struct{}, 10)
)

func main() {
	fmt.Println("============================================================")
	fmt.Println("   Split OpenFeature Provider - Advanced Cloud Tests")
	fmt.Println("   Testing: Event Tracking & Configuration Change Detection")
	fmt.Println("============================================================")
	fmt.Println()

	// ============================================================
	// SETUP: CONTEXT WITH TIMEOUT AND SIGNAL HANDLING
	// ============================================================

	// 5-minute timeout for interactive test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// ============================================================
	// 1. LOGGING CONFIGURATION
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

	appLogger.Info("logging configured", "level", logLevel.String())

	// ============================================================
	// 2. CHECK API KEY
	// ============================================================

	apiKey := os.Getenv("SPLIT_API_KEY")
	if apiKey == "" {
		appLogger.Error("SPLIT_API_KEY environment variable is required")
		appLogger.Info("Usage: SPLIT_API_KEY=your-key go run main.go")
		os.Exit(1)
	}

	// ============================================================
	// 3. OPENFEATURE LOGGING HOOK
	// ============================================================
	openfeature.AddHooks(hooks.NewLoggingHook(false, ofLogger))

	// ============================================================
	// 4. EVENT HANDLERS
	// ============================================================

	readyHandler := func(details openfeature.EventDetails) {
		readyCount.Add(1)
		appLogger.Info("EVENT: PROVIDER_READY",
			"count", readyCount.Load(),
			"message", details.Message)
	}
	openfeature.AddHandler(openfeature.ProviderReady, &readyHandler)

	configChangeHandler := func(details openfeature.EventDetails) {
		configChangedCount.Add(1)
		appLogger.Info("EVENT: PROVIDER_CONFIGURATION_CHANGED",
			"count", configChangedCount.Load(),
			"message", details.Message)
		select {
		case configChangedChan <- struct{}{}:
		default:
		}
	}
	openfeature.AddHandler(openfeature.ProviderConfigChange, &configChangeHandler)

	errorHandler := func(details openfeature.EventDetails) {
		errorCount.Add(1)
		appLogger.Error("EVENT: PROVIDER_ERROR",
			"count", errorCount.Load(),
			"message", details.Message)
	}
	openfeature.AddHandler(openfeature.ProviderError, &errorHandler)

	appLogger.Info("event handlers registered", "handlers", 3)

	// ============================================================
	// 5. CREATE PROVIDER WITH OPTIMIZED CONFIG
	// ============================================================

	// Use optimized test configuration for faster execution
	cfg := split.TestConfig()

	provider, err := split.New(apiKey,
		split.WithLogger(baseLogger),
		split.WithSplitConfig(cfg),
		split.WithMonitoringInterval(5*time.Second), // Fast config change detection
	)
	if err != nil {
		appLogger.Error("failed to create provider", "error", err)
		os.Exit(1)
	}

	appLogger.Info("provider created",
		"monitoring_interval", "5s",
		"block_until_ready", cfg.BlockUntilReady)

	// ============================================================
	// 6. GRACEFUL SHUTDOWN SETUP
	// ============================================================

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic during shutdown", "panic", r)
				}
			}()

			fmt.Println()
			fmt.Println(strings.Repeat("-", 60))
			slog.Info("initiating graceful shutdown")

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := openfeature.ShutdownWithContext(shutdownCtx); err != nil {
				slog.Error("shutdown error", "error", err)
			}

			slog.Info("graceful shutdown complete")
		})
	}

	defer cleanup()

	// Setup interrupt handling
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
	// 7. PROVIDER INITIALIZATION
	// ============================================================

	initCtx, initCancel := context.WithTimeout(ctx, 30*time.Second)
	defer initCancel()

	appLogger.Info("initializing provider...")
	if err := openfeature.SetProviderWithContextAndWait(initCtx, provider); err != nil {
		appLogger.Error("failed to initialize provider", "error", err)
		cleanup()
		os.Exit(1)
	}

	appLogger.Info("provider initialized successfully")

	// Create OpenFeature client for tracking
	client := openfeature.NewDefaultClient()

	// ============================================================
	// TRACKING TEST
	// ============================================================

	fmt.Println()
	fmt.Println("------------------------------------------------------------")
	fmt.Println(">> TRACKING EVENTS (view in Split Data Hub)")
	fmt.Println("------------------------------------------------------------")

	testTracking(ctx, client, appLogger)

	// ============================================================
	// CONFIGURATION CHANGE TEST
	// ============================================================

	fmt.Println()
	fmt.Println("------------------------------------------------------------")
	fmt.Println(">> CONFIGURATION CHANGE EVENT DETECTION")
	fmt.Println("------------------------------------------------------------")

	testConfigurationChange(ctx, appLogger)

	// ============================================================
	// EVENT SUMMARY
	// ============================================================

	fmt.Println()
	fmt.Println("------------------------------------------------------------")
	fmt.Println(">> EVENT SUMMARY")
	fmt.Println("------------------------------------------------------------")

	appLogger.Info("provider event summary",
		"PROVIDER_READY", readyCount.Load(),
		"PROVIDER_CONFIGURATION_CHANGED", configChangedCount.Load(),
		"PROVIDER_ERROR", errorCount.Load())

	if readyCount.Load() >= 1 {
		appLogger.Info("PASS: received PROVIDER_READY event")
	} else {
		appLogger.Error("FAIL: did not receive PROVIDER_READY event")
	}

	appLogger.Info("configuration change test completed")
}

func testConfigurationChange(ctx context.Context, logger *slog.Logger) {
	if ctx.Err() != nil {
		logger.Info("skipping - context cancelled")
		return
	}

	// Drain any config change events that occurred before this test
	for len(configChangedChan) > 0 {
		<-configChangedChan
	}

	logger.Info("waiting for PROVIDER_CONFIGURATION_CHANGED event...")
	logger.Info("modify any flag in Split dashboard to trigger the event", "timeout", "2m")

	select {
	case <-ctx.Done():
		logger.Info("context canceled")
		return
	case <-configChangedChan:
		logger.Info("PASS: PROVIDER_CONFIGURATION_CHANGED event detected")
	case <-time.After(2 * time.Minute):
		logger.Warn("no configuration change detected within timeout", "timeout", "2m")
		return
	}
}

// testTracking sends tracking events to Split for viewing in the console.
// Events can be viewed in Split Data Hub.
func testTracking(ctx context.Context, client *openfeature.Client, logger *slog.Logger) {
	if ctx.Err() != nil {
		logger.Info("skipping - context cancelled")
		return
	}

	logger.Info("sending tracking events to Split...")

	// Test 1: Basic event with default traffic type ("user")
	evalCtx := openfeature.NewEvaluationContext("test-user-123", nil)
	details := openfeature.NewTrackingEventDetails(1.0)
	client.Track(ctx, "page_view", evalCtx, details)
	logger.Info("sent tracking event",
		"event", "page_view",
		"key", "test-user-123",
		"trafficType", "user",
		"value", 1.0)

	// Test 2: Event with custom traffic type
	evalCtxAccount := openfeature.NewEvaluationContext("account-456", map[string]any{
		"trafficType": "account",
	})
	client.Track(ctx, "subscription_created", evalCtxAccount, openfeature.NewTrackingEventDetails(99.99))
	logger.Info("sent tracking event",
		"event", "subscription_created",
		"key", "account-456",
		"trafficType", "account",
		"value", 99.99)

	// Test 3: Event with properties
	evalCtxPurchase := openfeature.NewEvaluationContext("user-789", nil)
	purchaseDetails := openfeature.NewTrackingEventDetails(149.99).
		Add("currency", "USD").
		Add("item_count", 3).
		Add("category", "electronics").
		Add("is_first_purchase", true)
	client.Track(ctx, "purchase_completed", evalCtxPurchase, purchaseDetails)
	logger.Info("sent tracking event",
		"event", "purchase_completed",
		"key", "user-789",
		"trafficType", "user",
		"value", 149.99,
		"properties", "currency=USD, item_count=3, category=electronics, is_first_purchase=true")

	// Test 4: Event without value (count-only)
	client.Track(ctx, "button_clicked", evalCtx, openfeature.NewTrackingEventDetails(0))
	logger.Info("sent tracking event",
		"event", "button_clicked",
		"key", "test-user-123",
		"trafficType", "user",
		"value", 0)

	logger.Info("tracking events sent successfully",
		"total_events", 4,
		"note", "view events in Split Data Hub")
}
