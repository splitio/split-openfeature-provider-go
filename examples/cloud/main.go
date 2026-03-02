// Package main demonstrates cloud mode usage of the Split OpenFeature Provider.
//
// This example shows how to:
//   - Create and initialize a Split provider in streaming/cloud mode
//   - Evaluate different flag types (boolean, string, int, float, object)
//   - Get evaluation details (variant, reason, flag metadata)
//   - Monitor provider health
//
// This example requires a Split API key and connects to Split's cloud service.
// Flags that don't exist return their default values - create flags in Split dashboard.
//
// Run: SPLIT_API_KEY=your-key-here go run main.go
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

	apiKey := os.Getenv("SPLIT_API_KEY")
	if apiKey == "" {
		appLogger.Error("SPLIT_API_KEY environment variable is required")
		os.Exit(1)
	}

	// Use optimized test configuration for faster startup
	cfg := split.TestConfig()

	provider, err := split.New(apiKey,
		split.WithLogger(baseLogger),
		split.WithSplitConfig(cfg),
	)
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

	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := openfeature.SetNamedProviderWithContextAndWait(initCtx, "cloud-streaming", provider); err != nil {
		appLogger.Error("failed to initialize provider", "error", err)
		os.Exit(1)
	}

	appLogger.Info("Split provider initialized successfully in cloud/streaming mode")

	client := openfeature.NewClient("cloud-streaming")
	ctx := context.Background()

	// Check provider state
	if client.State() == openfeature.ReadyState {
		appLogger.Info("provider is ready for evaluations")
	}

	// Get client metadata
	metadata := client.Metadata()
	appLogger.Info("client metadata", "domain", metadata.Domain())

	evalCtx := openfeature.NewEvaluationContext("user-123", map[string]any{
		"email": "user@example.com",
		"plan":  "premium",
	})

	// Example 1: Boolean flag evaluation
	appLogger.Info("boolean flag evaluation")
	showNewFeature, err := client.BooleanValue(ctx, "feature_boolean_on", false, evalCtx)
	if err != nil {
		appLogger.Warn("error evaluating boolean flag", "error", err)
	}
	appLogger.Info("flag evaluated", "flag", "feature_boolean_on", "value", showNewFeature, "default", false)

	// Example 2: String flag evaluation
	appLogger.Info("string flag evaluation")
	theme, err := client.StringValue(ctx, "ui_theme", "light", evalCtx)
	if err != nil {
		appLogger.Warn("error evaluating string flag", "error", err)
	}
	appLogger.Info("flag evaluated", "flag", "ui_theme", "value", theme, "default", "light")

	// Example 3: Integer flag evaluation
	appLogger.Info("integer flag evaluation")
	maxRetries, err := client.IntValue(ctx, "max_retries", 3, evalCtx)
	if err != nil {
		appLogger.Warn("error evaluating integer flag", "error", err)
	}
	appLogger.Info("flag evaluated", "flag", "max_retries", "value", maxRetries, "default", 3)

	// Example 4: Float flag evaluation
	appLogger.Info("float flag evaluation")
	discountRate, err := client.FloatValue(ctx, "discount_rate", 0.0, evalCtx)
	if err != nil {
		appLogger.Warn("error evaluating float flag", "error", err)
	}
	appLogger.Info("flag evaluated", "flag", "discount_rate", "value", discountRate, "default", 0.0)

	// Example 5: Object flag evaluation (evaluates flag sets in cloud mode)
	appLogger.Info("object flag evaluation (flag set)")
	flagSetData, err := client.ObjectValue(ctx, "split_provider_test", split.FlagSetResult{}, evalCtx)
	if err != nil {
		appLogger.Warn("error evaluating object flag", "error", err)
	} else if flags, ok := flagSetData.(split.FlagSetResult); ok {
		appLogger.Info("flag set evaluated",
			"flag_set", "split_provider_test",
			"flags_count", len(flags))
		// Access individual flags using the struct
		if uiTheme, ok := flags["ui_theme"]; ok {
			appLogger.Info("flag from set", "flag", "ui_theme", "treatment", uiTheme.Treatment)
		}
	}

	// Example 6: Get evaluation details with flag metadata
	appLogger.Info("getting evaluation details with metadata")
	details, err := client.StringValueDetails(ctx, "ui_theme", "light", evalCtx)
	if err != nil {
		appLogger.Warn("error getting evaluation details", "error", err)
	} else {
		appLogger.Info("evaluation details",
			"value", details.Value,
			"variant", details.Variant,
			"reason", details.Reason,
			"flag_key", details.FlagKey,
			"has_metadata", len(details.FlagMetadata) > 0)
		if len(details.FlagMetadata) > 0 {
			appLogger.Info("flag metadata available",
				"metadata_keys", len(details.FlagMetadata))
		}
	}

	// Example 7: Evaluation mode - force individual flag evaluation in cloud mode
	// By default, ObjectEvaluation in cloud mode evaluates flag sets.
	// Use WithEvaluationMode to evaluate a single flag as an object instead.
	appLogger.Info("individual flag evaluation in cloud mode")
	individualCtx := split.WithEvaluationMode(ctx, split.EvaluationModeIndividual)
	singleFlag, err := client.ObjectValue(individualCtx, "ui_theme", split.FlagSetResult{}, evalCtx)
	if err != nil {
		appLogger.Warn("error evaluating individual flag", "error", err)
	} else if flags, ok := singleFlag.(split.FlagSetResult); ok {
		appLogger.Info("individual flag evaluated", "flags_count", len(flags))
	}

	// Example 8: Track event with metric value
	appLogger.Info("tracking events")
	trackDetails := openfeature.NewTrackingEventDetails(99.99).
		Add("currency", "USD")
	client.Track(ctx, "purchase", evalCtx, trackDetails)
	appLogger.Info("tracked event with metric value", "event", "purchase", "value", 99.99)

	// Example 9: Track count-only event (no metric value)
	// Use WithoutMetricValue to avoid polluting sum/average metrics with zeros
	noValueCtx := split.WithoutMetricValue(ctx)
	countDetails := openfeature.NewTrackingEventDetails(0).
		Add("page", "/home")
	client.Track(noValueCtx, "page_view", evalCtx, countDetails)
	appLogger.Info("tracked count-only event (nil value sent to Split)", "event", "page_view")

	// Example 10: Provider health check
	appLogger.Info("provider health check")
	metrics := provider.Metrics()
	appLogger.Info("provider health",
		"provider", metrics.Provider,
		"status", metrics.Status,
		"initialized", metrics.Initialized,
		"ready", metrics.Ready,
		"splits_count", metrics.SplitsCount)

	appLogger.Info("example completed successfully")
}
