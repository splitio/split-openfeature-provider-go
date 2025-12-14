package split

import (
	"context"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/conf"
	"github.com/splitio/go-toolkit/v5/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTrack tests the Track method (Tracker interface).
func TestTrack(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	t.Run("basic tracking", func(t *testing.T) {
		evalCtx := openfeature.NewEvaluationContext("test-user", nil)
		details := openfeature.NewTrackingEventDetails(42.0)

		// Should not panic - Track returns void
		provider.Track(context.Background(), "test_event", evalCtx, details)
	})

	t.Run("tracking with custom traffic type", func(t *testing.T) {
		evalCtx := openfeature.NewEvaluationContext("test-user", map[string]any{
			"trafficType": "account",
		})
		details := openfeature.NewTrackingEventDetails(99.99)

		// Should not panic
		provider.Track(context.Background(), "account_event", evalCtx, details)
	})

	t.Run("tracking with properties", func(t *testing.T) {
		evalCtx := openfeature.NewEvaluationContext("test-user", nil)
		details := openfeature.NewTrackingEventDetails(149.99).
			Add("currency", "USD").
			Add("item_count", 3).
			Add("is_premium", true)

		// Should not panic
		provider.Track(context.Background(), "purchase", evalCtx, details)
	})

	t.Run("tracking with empty targeting key is ignored", func(t *testing.T) {
		evalCtx := openfeature.NewEvaluationContext("", nil)
		details := openfeature.NewTrackingEventDetails(1.0)

		// Should not panic - silently ignored
		provider.Track(context.Background(), "ignored_event", evalCtx, details)
	})

	t.Run("tracking with canceled context is ignored", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		evalCtx := openfeature.NewEvaluationContext("test-user", nil)
		details := openfeature.NewTrackingEventDetails(1.0)

		// Should not panic - silently ignored due to canceled context
		provider.Track(ctx, "canceled_event", evalCtx, details)
	})
}

// TestTrackProviderNotReady tests that Track is ignored when provider is not ready.
func TestTrackProviderNotReady(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	// Don't initialize - provider is NotReady
	assert.Equal(t, openfeature.NotReadyState, provider.Status(), "Provider should be NotReady")

	evalCtx := openfeature.NewEvaluationContext("test-user", nil)
	details := openfeature.NewTrackingEventDetails(1.0)

	// Should not panic - silently ignored because provider not ready
	provider.Track(context.Background(), "ignored_event", evalCtx, details)
}

// =============================================================================
// Track Options Tests (Localhost Mode)
// =============================================================================

func TestTrack_WithoutMetricValue_Localhost(t *testing.T) {
	provider := setupLocalhostProvider(t)

	ctx := WithoutMetricValue(context.Background())
	evalCtx := openfeature.NewEvaluationContext("key", nil)
	details := openfeature.NewTrackingEventDetails(0)

	// In localhost mode, Track runs the full code path but the SDK discards the event
	// (no server to send to). Should not panic.
	trackOpts := GetTrackOptions(ctx)
	assert.True(t, trackOpts.MetricValueAbsent)

	// Track call should not panic
	provider.Track(ctx, "page_view", evalCtx, details)
}

func TestTrack_WithMetricValue_Localhost(t *testing.T) {
	provider := setupLocalhostProvider(t)

	evalCtx := openfeature.NewEvaluationContext("key", nil)
	details := openfeature.NewTrackingEventDetails(99.99)

	// Track call should not panic
	provider.Track(context.Background(), "purchase", evalCtx, details)
}
