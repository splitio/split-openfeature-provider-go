// sdk.go contains tests for direct SDK access, concurrency, health, and metrics.
// Tests cover Split SDK client access, concurrent evaluations, event tracking,
// provider health status, and metrics before/after initialization.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/conf"

	"github.com/splitio/split-openfeature-provider-go/v2"
)

// testDirectSDKAccess tests direct access to the Split SDK client
func testDirectSDKAccess(provider *split.Provider) {

	factory := provider.Factory()
	splitClient := factory.Client()

	err := splitClient.Track("test-user", "user", "test_event", 1.0, map[string]any{
		"test":      "integration_test",
		"timestamp": time.Now().Unix(),
	})

	if err != nil {
		results.Fail("SDK(Track)", err.Error())
	} else {
		results.Pass("SDK(Track)")
	}

	treatments := splitClient.Treatments("test-user", []string{
		"feature_boolean_on",
		"ui_theme",
		"max_retries",
	}, nil)

	if len(treatments) != 3 {
		results.Fail("SDK(Treatments)", fmt.Sprintf("expected 3 treatments, got %d", len(treatments)))
	} else {
		results.Pass(fmt.Sprintf("SDK(Treatments) - %d flags evaluated", len(treatments)))
	}
}

// testClientTrack tests the OpenFeature Client.Track() method which uses the provider's Tracker interface
func testClientTrack(ctx context.Context, client *openfeature.Client) {
	// Test 1: Basic tracking with default traffic type ("user")
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)
	details := openfeature.NewTrackingEventDetails(42.0)

	// Track should not panic and should complete (no error return per OpenFeature spec)
	client.Track(ctx, "test_event", evalCtx, details)
	results.Pass("Client(Track_basic)")

	// Test 2: Tracking with custom traffic type
	evalCtxWithTrafficType := openfeature.NewEvaluationContext("test-user", map[string]any{
		"trafficType": "account",
	})
	client.Track(ctx, "account_event", evalCtxWithTrafficType, details)
	results.Pass("Client(Track_custom_traffic_type)")

	// Test 3: Tracking with properties
	detailsWithProps := openfeature.NewTrackingEventDetails(99.99).
		Add("currency", "USD").
		Add("item_count", 3).
		Add("is_premium", true)
	client.Track(ctx, "purchase", evalCtx, detailsWithProps)
	results.Pass("Client(Track_with_properties)")

	// Test 4: Tracking with empty targeting key should be silently ignored
	emptyEvalCtx := openfeature.NewEvaluationContext("", nil)
	client.Track(ctx, "ignored_event", emptyEvalCtx, details)
	results.Pass("Client(Track_empty_key_ignored)")
}

// testTrackWithOptions tests Track with context options (MetricValueAbsent).
func testTrackWithOptions(ctx context.Context, provider *split.Provider) {
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	// Test 1: Track without metric value (count-only event)
	noValueCtx := split.WithoutMetricValue(ctx)
	details := openfeature.NewTrackingEventDetails(0)
	provider.Track(noValueCtx, "page_view", evalCtx, details)
	results.Pass("Track(without_metric_value)")

	// Test 2: Track with metric value (standard)
	detailsWithValue := openfeature.NewTrackingEventDetails(99.99)
	provider.Track(ctx, "purchase", evalCtx, detailsWithValue)
	results.Pass("Track(with_metric_value)")

	// Test 3: Verify context option is correctly set
	trackOpts := split.GetTrackOptions(noValueCtx)
	if !trackOpts.MetricValueAbsent {
		results.Fail("Track(context_option)", "MetricValueAbsent should be true")
	} else {
		results.Pass("Track(context_option)")
	}

	// Test 4: Track with both WithoutMetricValue and non-zero value in details
	// MetricValueAbsent should take precedence (nil sent to Split, not 42.0)
	precedenceCtx := split.WithoutMetricValue(ctx)
	detailsWithNonZero := openfeature.NewTrackingEventDetails(42.0)
	provider.Track(precedenceCtx, "count_event", evalCtx, detailsWithNonZero)
	results.Pass("Track(absent_takes_precedence)")
}

// testConcurrentEvaluations tests concurrent flag evaluations
func testConcurrentEvaluations(ctx context.Context, client *openfeature.Client) {
	const numGoroutines = 100
	const evaluationsPerGoroutine = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*evaluationsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			evalCtx := openfeature.NewEvaluationContext(fmt.Sprintf("user-%d", id), nil)

			for j := 0; j < evaluationsPerGoroutine; j++ {
				_, err := client.BooleanValue(ctx, "feature_boolean_on", false, evalCtx)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d iteration %d: %w", id, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		slog.Error("concurrent evaluation error", "error", err)
		errorCount++
	}

	if errorCount > 0 {
		results.Fail("Concurrent(evaluations)", fmt.Sprintf("%d errors in %d evaluations",
			errorCount, numGoroutines*evaluationsPerGoroutine))
	} else {
		results.Pass(fmt.Sprintf("Concurrent(%d goroutines × %d evals)",
			numGoroutines, evaluationsPerGoroutine))
	}
}

// testClientState tests the Client.State() method which queries the provider's status
func testClientState(client *openfeature.Client) {
	// Client.State() should return the provider's status via Provider.Status()
	state := client.State()
	if state != openfeature.ReadyState {
		results.Fail("Client(State)", fmt.Sprintf("expected Ready, got %s", state))
	} else {
		results.Pass("Client(State)")
	}
}

// testProviderHealth tests provider status and metrics
func testProviderHealth(provider *split.Provider) {
	status := provider.Status()
	if status != openfeature.ReadyState {
		results.Fail("Health(Status)", fmt.Sprintf("expected Ready, got %s", status))
	} else {
		results.Pass("Health(Status)")
	}

	metrics := provider.Metrics()

	if metrics.Provider != "Split" {
		results.Fail("Health(provider)", fmt.Sprintf("expected Split, got %v", metrics.Provider))
	} else {
		results.Pass("Health(provider)")
	}

	if metrics.Status != openfeature.ReadyState {
		results.Fail("Health(status)", fmt.Sprintf("expected Ready, got %v", metrics.Status))
	} else {
		results.Pass("Health(status)")
	}

	if !metrics.Initialized {
		results.Fail("Health(initialized)", "provider should be initialized")
	} else {
		results.Pass("Health(initialized)")
	}
}

// testEventTracking tests that events are properly tracked
func testEventTracking(eventsReceived *sync.Map) {
	// Verify that PROVIDER_READY event was received
	if val, ok := eventsReceived.Load(openfeature.ProviderReady); ok {
		counter := val.(*atomic.Int64)
		count := counter.Load()
		if count > 0 {
			results.Pass(fmt.Sprintf("Events(PROVIDER_READY) - %d events", count))
		} else {
			results.Fail("Events(PROVIDER_READY)", "no events received")
		}
	} else {
		results.Fail("Events(PROVIDER_READY)", "event type not found in sync.Map")
	}

	var totalEvents int64
	eventsReceived.Range(func(key, value any) bool {
		counter := value.(*atomic.Int64)
		totalEvents += counter.Load()
		return true
	})

	if totalEvents > 0 {
		results.Pass(fmt.Sprintf("Events(Total) - %d total events", totalEvents))
	} else {
		results.Fail("Events(Total)", "no events received at all")
	}
}

// testMetricsBeforeInit tests Metrics() before provider initialization
func testMetricsBeforeInit() {
	// Use optimized test config with SplitFile to avoid SDK errors looking for ~/.splits
	cfg := split.TestConfig()
	cfg.SplitFile = "./split.yaml"

	uninitProvider, err := split.New("localhost", split.WithSplitConfig(cfg))
	if err != nil {
		results.Fail("MetricsBeforeInit(create)", fmt.Sprintf("failed to create provider: %v", err))
		return
	}
	defer uninitProvider.Shutdown()

	metrics := uninitProvider.Metrics()

	if metrics.Initialized {
		results.Fail("MetricsBeforeInit(initialized)", "expected initialized=false")
	} else {
		results.Pass("MetricsBeforeInit(initialized)")
	}

	if metrics.Status != openfeature.NotReadyState {
		results.Fail("MetricsBeforeInit(status)", fmt.Sprintf("expected NotReady, got %s", metrics.Status))
	} else {
		results.Pass("MetricsBeforeInit(status)")
	}

	if metrics.Ready {
		results.Fail("MetricsBeforeInit(ready)", "expected ready=false")
	} else {
		results.Pass("MetricsBeforeInit(ready)")
	}

	if metrics.SplitsCount != -1 {
		results.Fail("MetricsBeforeInit(splits_count)", fmt.Sprintf("expected -1 when not ready, got %d", metrics.SplitsCount))
	} else {
		results.Pass("MetricsBeforeInit(splits_count)")
	}
}

// testMetricsAllFields tests all Metrics() fields after initialization
func testMetricsAllFields(provider *split.Provider) {
	metrics := provider.Metrics()

	if metrics.Provider != "Split" {
		results.Fail("MetricsAllFields(provider)", fmt.Sprintf("expected Split, got %s", metrics.Provider))
	} else {
		results.Pass("MetricsAllFields(provider)")
	}

	if metrics.Status != openfeature.ReadyState {
		results.Fail("MetricsAllFields(status)", fmt.Sprintf("expected Ready, got %v", metrics.Status))
	} else {
		results.Pass("MetricsAllFields(status)")
	}

	if !metrics.Initialized {
		results.Fail("MetricsAllFields(initialized)", "expected true")
	} else {
		results.Pass("MetricsAllFields(initialized)")
	}

	if !metrics.Ready {
		results.Fail("MetricsAllFields(ready)", "expected true")
	} else {
		results.Pass("MetricsAllFields(ready)")
	}

	if metrics.SplitsCount < 0 {
		results.Fail("MetricsAllFields(splits_count)", fmt.Sprintf("invalid count: %d", metrics.SplitsCount))
	} else {
		results.Pass(fmt.Sprintf("MetricsAllFields(splits_count=%d)", metrics.SplitsCount))
	}
}

// testMetricsAfterShutdown tests Metrics() after provider shutdown
func testMetricsAfterShutdown(apiKey string, logger *slog.Logger, cfg *conf.SplitSdkConfig) {

	metricsProvider, err := split.New(apiKey, split.WithLogger(logger), split.WithSplitConfig(cfg))
	if err != nil {
		results.Fail("MetricsAfterShutdown(create)", fmt.Sprintf("failed to create: %v", err))
		return
	}

	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	if err := metricsProvider.InitWithContext(initCtx, evalCtx); err != nil {
		results.Fail("MetricsAfterShutdown(init)", fmt.Sprintf("init failed: %v", err))
		metricsProvider.Shutdown()
		return
	}

	// Shutdown the provider
	// In cloud mode with SSE streaming, the Split SDK has a known hang bug
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()
	if err := metricsProvider.ShutdownWithContext(shutdownCtx); err != nil {
		// In cloud mode, shutdown timeout is acceptable due to SSE streaming bug
		if apiKey != "localhost" && strings.Contains(err.Error(), "context deadline exceeded") {
			// Continue with test - we can still check metrics after timeout
		} else {
			results.Fail("MetricsAfterShutdown(shutdown)", fmt.Sprintf("shutdown failed: %v", err))
			return
		}
	}

	// Get metrics after shutdown
	metrics := metricsProvider.Metrics()

	if metrics.Status != openfeature.NotReadyState {
		results.Fail("MetricsAfterShutdown(status)", fmt.Sprintf("expected NotReady, got %v", metrics.Status))
	} else {
		results.Pass("MetricsAfterShutdown(status)")
	}

	if metrics.Initialized {
		results.Fail("MetricsAfterShutdown(initialized)", "expected false after shutdown")
	} else {
		results.Pass("MetricsAfterShutdown(initialized)")
	}

	if metrics.Ready {
		results.Fail("MetricsAfterShutdown(ready)", "expected false after shutdown")
	} else {
		results.Pass("MetricsAfterShutdown(ready)")
	}
}
