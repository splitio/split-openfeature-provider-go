// Package main provides a test for PROVIDER_CONFIGURATION_CHANGED event detection.
//
// This test validates configuration change event detection which requires:
//   - A real Split account with SDK API key
//   - Manual flag modification in the Split dashboard during the test
//
// All other cloud-only features (flag sets, targeting rules) are tested in the
// integration test when SPLIT_API_KEY is provided.
//
// Prerequisites:
//   - A Split account with SDK API key
//   - Any flag to modify during the test
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
	fmt.Println("   Split OpenFeature Provider - Configuration Change Test")
	fmt.Println("   Testing: PROVIDER_CONFIGURATION_CHANGED Event Detection")
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
	// 5. CREATE PROVIDER WITH FAST MONITORING
	// ============================================================

	// Use 5-second monitoring interval for faster configuration change detection
	provider, err := split.New(apiKey,
		split.WithLogger(baseLogger),
		split.WithMonitoringInterval(5*time.Second),
	)
	if err != nil {
		appLogger.Error("failed to create provider", "error", err)
		os.Exit(1)
	}

	appLogger.Info("provider created", "monitoring_interval", "5s")

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
