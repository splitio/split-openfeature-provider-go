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

// testProviderHealth tests provider status and metrics
func testProviderHealth(provider *split.Provider) {
	status := provider.Status()
	if status != openfeature.ReadyState {
		results.Fail("Health(Status)", fmt.Sprintf("expected Ready, got %s", status))
	} else {
		results.Pass("Health(Status)")
	}

	metrics := provider.Metrics()

	if metrics["provider"] != "Split" {
		results.Fail("Health(provider)", fmt.Sprintf("expected Split, got %v", metrics["provider"]))
	} else {
		results.Pass("Health(provider)")
	}

	if metrics["status"] != string(openfeature.ReadyState) {
		results.Fail("Health(status)", fmt.Sprintf("expected Ready, got %v", metrics["status"]))
	} else {
		results.Pass("Health(status)")
	}

	if initialized, ok := metrics["initialized"].(bool); !ok || !initialized {
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
	// Use proper localhost config with SplitFile to avoid SDK errors looking for ~/.splits
	cfg := conf.Default()
	cfg.SplitFile = "./split.yaml"

	uninitProvider, err := split.New("localhost", split.WithSplitConfig(cfg))
	if err != nil {
		results.Fail("MetricsBeforeInit(create)", fmt.Sprintf("failed to create provider: %v", err))
		return
	}
	defer uninitProvider.Shutdown()

	metrics := uninitProvider.Metrics()

	if initialized, ok := metrics["initialized"].(bool); !ok {
		results.Fail("MetricsBeforeInit(initialized_type)", "initialized field has wrong type")
	} else if initialized {
		results.Fail("MetricsBeforeInit(initialized_value)", "expected initialized=false")
	} else {
		results.Pass("MetricsBeforeInit(initialized)")
	}

	if status, ok := metrics["status"].(string); !ok {
		results.Fail("MetricsBeforeInit(status_type)", "status field has wrong type")
	} else if status != string(openfeature.NotReadyState) {
		results.Fail("MetricsBeforeInit(status_value)", fmt.Sprintf("expected NotReady, got %s", status))
	} else {
		results.Pass("MetricsBeforeInit(status)")
	}

	if ready, ok := metrics["ready"].(bool); !ok {
		results.Fail("MetricsBeforeInit(ready_type)", "ready field has wrong type")
	} else if ready {
		results.Fail("MetricsBeforeInit(ready_value)", "expected ready=false")
	} else {
		results.Pass("MetricsBeforeInit(ready)")
	}

	if _, ok := metrics["splits_count"]; ok {
		results.Fail("MetricsBeforeInit(splits_count)", "splits_count should not be present when not ready")
	} else {
		results.Pass("MetricsBeforeInit(splits_count_absent)")
	}
}

// testMetricsAllFields tests all Metrics() fields after initialization
func testMetricsAllFields(provider *split.Provider) {
	metrics := provider.Metrics()

	tests := []struct {
		field    string
		expected interface{}
	}{
		{"provider", "Split"},
		{"status", string(openfeature.ReadyState)},
		{"initialized", true},
		{"ready", true},
	}

	for _, tt := range tests {
		val, ok := metrics[tt.field]
		if !ok {
			results.Fail(fmt.Sprintf("MetricsAllFields(%s_present)", tt.field), "field not found")
			continue
		}
		if val != tt.expected {
			results.Fail(fmt.Sprintf("MetricsAllFields(%s)", tt.field),
				fmt.Sprintf("expected %v, got %v", tt.expected, val))
		} else {
			results.Pass(fmt.Sprintf("MetricsAllFields(%s)", tt.field))
		}
	}

	if count, ok := metrics["splits_count"].(int); !ok {
		results.Fail("MetricsAllFields(splits_count_type)", "splits_count has wrong type")
	} else if count < 0 {
		results.Fail("MetricsAllFields(splits_count_value)", fmt.Sprintf("invalid count: %d", count))
	} else {
		results.Pass(fmt.Sprintf("MetricsAllFields(splits_count=%d)", count))
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

	// Should show not ready
	if status, ok := metrics["status"].(string); !ok {
		results.Fail("MetricsAfterShutdown(status_type)", "status has wrong type")
	} else if status != string(openfeature.NotReadyState) {
		results.Fail("MetricsAfterShutdown(status)", fmt.Sprintf("expected NotReady, got %s", status))
	} else {
		results.Pass("MetricsAfterShutdown(status)")
	}

	// initialized should be false
	if initialized, ok := metrics["initialized"].(bool); !ok {
		results.Fail("MetricsAfterShutdown(initialized_type)", "initialized has wrong type")
	} else if initialized {
		results.Fail("MetricsAfterShutdown(initialized)", "expected false after shutdown")
	} else {
		results.Pass("MetricsAfterShutdown(initialized)")
	}

	// ready should be false
	if ready, ok := metrics["ready"].(bool); !ok {
		results.Fail("MetricsAfterShutdown(ready_type)", "ready has wrong type")
	} else if ready {
		results.Fail("MetricsAfterShutdown(ready)", "expected false after shutdown")
	} else {
		results.Pass("MetricsAfterShutdown(ready)")
	}
}
