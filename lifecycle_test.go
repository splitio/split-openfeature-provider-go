package split

import (
	"context"
	"testing"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/conf"
	"github.com/splitio/go-toolkit/v5/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProviderInit tests the Init method and lifecycle initialization.
func TestProviderInit(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	assert.Equal(t, openfeature.NotReadyState, provider.Status(), "Provider should start in NotReady state")

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	assert.Equal(t, openfeature.ReadyState, provider.Status(), "Provider should be Ready after Init")

	// Calling Init again should be idempotent
	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	assert.NoError(t, err, "Second Init call should succeed (idempotent)")

	_ = provider.ShutdownWithContext(context.Background())
}

// TestProviderShutdown tests the Shutdown method and resource cleanup.
func TestProviderShutdown(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	assert.Equal(t, openfeature.ReadyState, provider.Status(), "Provider should be Ready after Init")

	_ = provider.ShutdownWithContext(context.Background())

	assert.Equal(t, openfeature.NotReadyState, provider.Status(), "Provider should be NotReady after Shutdown")

	// Calling Shutdown again should be idempotent (should not panic)
	_ = provider.ShutdownWithContext(context.Background())
}

// TestProviderShutdownTimeout tests that Shutdown completes within reasonable time.
func TestProviderShutdownTimeout(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	done := make(chan struct{})
	go func() {
		_ = provider.ShutdownWithContext(context.Background())
		close(done)
	}()

	select {
	case <-done:
		// Success - shutdown completed
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not complete within 10 seconds")
	}
}

// TestProviderEventChannel tests the EventChannel method and event emission.
func TestProviderEventChannel(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	eventChan := provider.EventChannel()
	require.NotNil(t, eventChan, "EventChannel() should not return nil")

	events := make([]openfeature.Event, 0)
	done := make(chan struct{})
	go func() {
		for event := range eventChan {
			events = append(events, event)
			if event.EventType == openfeature.ProviderReady {
				close(done)
				return
			}
		}
	}()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	select {
	case <-done:
		// Success - received ProviderReady event
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for ProviderReady event")
	}

	assert.NotEmpty(t, events, "Should receive at least one event")

	foundReady := false
	for _, event := range events {
		if event.EventType == openfeature.ProviderReady {
			foundReady = true
			assert.Equal(t, providerNameSplit, event.ProviderName, "Provider name should be 'Split'")
		}
	}
	assert.True(t, foundReady, "Should receive ProviderReady event")

	_ = provider.ShutdownWithContext(context.Background())
}

// TestProviderHealth tests the Health method.
func TestProviderHealth(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	// Health before Init
	metrics := provider.Metrics()
	assert.Equal(t, providerNameSplit, metrics.Provider, "Provider name should be 'Split'")
	assert.False(t, metrics.Initialized, "Should not be initialized before Init")
	assert.Equal(t, openfeature.NotReadyState, metrics.Status, "Status should be NOT_READY")

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	// Metrics after Init
	metrics = provider.Metrics()
	assert.True(t, metrics.Initialized, "Should be initialized after Init")
	assert.Equal(t, openfeature.ReadyState, metrics.Status, "Status should be READY")
	assert.True(t, metrics.Ready, "Should be ready")
	assert.Greater(t, metrics.SplitsCount, 0, "splits_count should be greater than 0")

	_ = provider.ShutdownWithContext(context.Background())
}

// TestInitWrapper tests the Init() method (StateHandler interface).
// Verifies it delegates to InitWithContext with a derived timeout.
func TestInitWrapper(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	assert.Equal(t, openfeature.NotReadyState, provider.Status(), "Provider should start in NotReady state")

	// Call Init (non-context version)
	err = provider.Init(openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	assert.Equal(t, openfeature.ReadyState, provider.Status(), "Provider should be Ready after Init")

	// Calling Init again should be idempotent
	err = provider.Init(openfeature.NewEvaluationContext("", nil))
	assert.NoError(t, err, "Second Init call should succeed (idempotent)")

	_ = provider.ShutdownWithContext(context.Background())
}

// TestShutdownWrapper tests the Shutdown() method (StateHandler interface).
// Verifies it delegates to ShutdownWithContext with a derived timeout.
func TestShutdownWrapper(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	assert.Equal(t, openfeature.ReadyState, provider.Status(), "Provider should be Ready after Init")

	// Call Shutdown (non-context version) - should not panic
	provider.Shutdown()

	assert.Equal(t, openfeature.NotReadyState, provider.Status(), "Provider should be NotReady after Shutdown")

	// Calling Shutdown again should be idempotent (should not panic)
	provider.Shutdown()
}

// TestProviderFactoryGetter tests the Factory method.
func TestProviderFactoryGetter(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Failed to initialize provider")

	factory := provider.Factory()
	require.NotNil(t, factory, "Factory should not be nil")

	splitClient := factory.Client()
	require.NotNil(t, splitClient, "Client should not be nil")

	err = splitClient.Track("test-user", "test-traffic", "test-event", 1.0, nil)
	assert.NoError(t, err, "Track call should succeed")

	manager := factory.Manager()
	require.NotNil(t, manager, "Manager should not be nil")

	splitNames := manager.SplitNames()
	assert.NotEmpty(t, splitNames, "Should have split definitions loaded")

	assert.True(t, factory.IsReady(), "Factory should be ready")

	_ = provider.ShutdownWithContext(context.Background())
}
