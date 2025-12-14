// Package main demonstrates localhost mode usage of the Split OpenFeature Provider.
//
// Localhost mode is ideal for:
//   - Development and testing without Split.io account
//   - Testing flag configurations locally before deployment
//   - CI/CD pipelines and integration tests
//
// This example shows how to:
//   - Configure Split SDK in localhost mode
//   - Load flags from a local YAML file (split.yaml)
//   - Evaluate flags with different user attributes
//
// Run: go run main.go
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/open-feature/go-sdk/openfeature/hooks"

	"github.com/splitio/split-openfeature-provider-go/v2"
)

func main() {
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
			slog.Warn("invalid LOG_LEVEL, using INFO", "provided", level, "valid", "debug|info|warn|error")
		}
	}

	baseLogger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      logLevel,
		TimeFormat: time.TimeOnly,
	}))

	appLogger := baseLogger.With("source", "app")
	ofLogger := baseLogger.With("source", "openfeature-sdk")

	slog.SetDefault(baseLogger)

	appLogger.Info("Split OpenFeature Provider - localhost mode example")
	appLogger.Warn("this example runs in LOCALHOST MODE for development/testing")
	appLogger.Info("reading feature flags from ./split.yaml")

	// Use optimized test configuration for faster startup
	cfg := split.TestConfig()
	cfg.SplitFile = "./split.yaml"

	provider, err := split.New("localhost", split.WithSplitConfig(cfg), split.WithLogger(baseLogger))
	if err != nil {
		appLogger.Error("failed to create provider", "error", err)
		os.Exit(1)
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := openfeature.ShutdownWithContext(shutdownCtx); err != nil {
			appLogger.Error("shutdown error", "error", err)
		}
	}()

	openfeature.AddHooks(hooks.NewLoggingHook(false, ofLogger))

	initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := openfeature.SetProviderWithContextAndWait(initCtx, provider); err != nil {
		appLogger.Error("failed to initialize provider", "error", err)
		os.Exit(1)
	}

	appLogger.Info("provider initialized in localhost mode")

	// Create default OpenFeature client (uses default provider)
	ofClient := openfeature.NewDefaultClient()
	ctx := context.Background()

	// Check provider state
	if ofClient.State() == openfeature.ReadyState {
		appLogger.Info("provider is ready for evaluations")
	}

	// Get client metadata
	metadata := ofClient.Metadata()
	appLogger.Info("client metadata", "domain", metadata.Domain())

	// Test with different users to see targeting in action
	testUsers := []string{"user-123", "user-456", "user-789"}

	for _, userID := range testUsers {
		appLogger.Info("evaluating flags for user", "user_id", userID)
		evalCtx := openfeature.NewEvaluationContext(userID, nil)

		// Boolean flag with targeting
		newFeature, _ := ofClient.BooleanValue(ctx, "new_feature", false, evalCtx)
		appLogger.Info("boolean flag evaluated", "flag", "new_feature", "value", newFeature)

		// String flag
		theme, _ := ofClient.StringValue(ctx, "ui_theme", "light", evalCtx)
		appLogger.Info("string flag evaluated", "flag", "ui_theme", "value", theme)

		// Integer flag
		maxRetries, _ := ofClient.IntValue(ctx, "max_retries", 3, evalCtx)
		appLogger.Info("integer flag evaluated", "flag", "max_retries", "value", maxRetries)

		// Float flag
		discount, _ := ofClient.FloatValue(ctx, "discount_rate", 0.0, evalCtx)
		appLogger.Info("float flag evaluated", "flag", "discount_rate", "value", discount)

		// Object flag with dynamic configuration - returns FlagSetResult
		premiumFeatures, _ := ofClient.ObjectValue(ctx, "premium_features", split.FlagSetResult{}, evalCtx)
		if flags, ok := premiumFeatures.(split.FlagSetResult); ok {
			if flag, ok := flags["premium_features"]; ok {
				appLogger.Info("object flag evaluated",
					"flag", "premium_features",
					"treatment", flag.Treatment,
					"has_config", flag.Config != nil)
			}
		}

		// Get evaluation details to see variant/treatment
		details, _ := ofClient.BooleanValueDetails(ctx, "new_feature", false, evalCtx)
		appLogger.Info("flag details", "variant", details.Variant, "reason", details.Reason)
	}

	// Demonstrate evaluation options
	// Note: In localhost mode, EvaluationModeSet is ignored (always uses individual).
	// EvaluationModeIndividual works as expected.
	appLogger.Info("evaluation mode options (localhost mode)")
	demoEvalCtx := openfeature.NewEvaluationContext("demo-user", nil)
	individualCtx := split.WithEvaluationMode(ctx, split.EvaluationModeIndividual)
	premiumIndividual, _ := ofClient.ObjectValue(individualCtx, "premium_features", split.FlagSetResult{}, demoEvalCtx)
	if flags, ok := premiumIndividual.(split.FlagSetResult); ok {
		appLogger.Info("individual mode evaluation", "flags_count", len(flags))
	}

	// Demonstrate track options
	appLogger.Info("track options")

	// Track with metric value (purchase amount)
	purchaseDetails := openfeature.NewTrackingEventDetails(149.99).
		Add("currency", "USD")
	ofClient.Track(ctx, "purchase", demoEvalCtx, purchaseDetails)
	appLogger.Info("tracked with metric value", "event", "purchase", "value", 149.99)

	// Track without metric value (count-only event)
	// WithoutMetricValue prevents polluting sum/average metrics with zeros
	noValueCtx := split.WithoutMetricValue(ctx)
	pageViewDetails := openfeature.NewTrackingEventDetails(0).
		Add("page", "/dashboard")
	ofClient.Track(noValueCtx, "page_view", demoEvalCtx, pageViewDetails)
	appLogger.Info("tracked count-only event (nil value)", "event", "page_view")

	// Demonstrate onboarding flow with configuration
	appLogger.Info("onboarding flow configuration")
	evalCtx := openfeature.NewEvaluationContext("new-user", nil)
	onboardingFlow, _ := ofClient.StringValue(ctx, "onboarding_flow", "v1", evalCtx)
	appLogger.Info("onboarding flow evaluated", "version", onboardingFlow)

	// Get the configuration
	details, _ := ofClient.StringValueDetails(ctx, "onboarding_flow", "v1", evalCtx)
	appLogger.Info("onboarding flow details", "variant", details.Variant)

	// Demonstrate maintenance mode flag
	appLogger.Info("system flags")
	maintenanceMode, _ := ofClient.BooleanValue(ctx, "maintenance_mode", false, evalCtx)
	if maintenanceMode {
		appLogger.Warn("system is in maintenance mode")
	} else {
		appLogger.Info("system is operational")
	}

	// Show provider health
	appLogger.Info("provider health")
	metrics := provider.Metrics()
	appLogger.Info("health status",
		"status", metrics.Status,
		"splits_count", metrics.SplitsCount)

	appLogger.Info("localhost mode example completed successfully")
	appLogger.Info("tips",
		"edit_config", "Edit split.yaml to change flag values",
		"network", "No network connection required",
		"ci_cd", "Perfect for CI/CD pipelines and unit tests",
		"docs", "See README.md for YAML format details")
}
