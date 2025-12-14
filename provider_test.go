//nolint:dupl,gocognit // Test patterns: type-specific tests have similar structure, comprehensive tests have higher complexity
package split

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/conf"
	"github.com/splitio/go-toolkit/v5/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// =============================================================================
// Test Constants
// =============================================================================

// Test flag names used across multiple tests
const (
	flagNonExistent   = "random-non-existent-feature"
	flagSomeOther     = "some_other_feature"
	flagMyFeature     = "my_feature"
	flagInt           = "int_feature"
	flagObj           = "obj_feature"
	flagUnparseable   = "unparseable_feature"
	flagMalformedJSON = "malformed_json_feature"
	// treatmentOn and treatmentOff are defined in constants.go
	treatmentUnparseable = "not-a-valid-type" // Treatment that cannot be parsed as bool/int/float
	testClientName       = "test_client"
	testSplitFile        = "testdata/split.yaml"
	providerNameSplit    = "Split"
)

// =============================================================================
// Test Main & Shared Helpers
// =============================================================================

// TestMain adds goroutine leak detection to all tests.
// Uses goleak to detect goroutine leaks from OUR code (external dependencies ignored).
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// Ignore OpenFeature SDK event executor goroutines (created per test via SetProvider)
		// Use IgnoreAnyFunction because these goroutines can be in various states
		// Note: Function names differ between normal and race detector builds
		goleak.IgnoreAnyFunction("github.com/open-feature/go-sdk/openfeature.(*eventExecutor).startEventListener.func1.1"),
		goleak.IgnoreAnyFunction("github.com/open-feature/go-sdk/openfeature.newEventExecutor.(*eventExecutor).startEventListener.func1.1"), // -race variant
		goleak.IgnoreAnyFunction("github.com/open-feature/go-sdk/openfeature.(*eventExecutor).startListeningAndShutdownOld.func1"),
		goleak.IgnoreAnyFunction("github.com/open-feature/go-sdk/openfeature.newEventExecutor.(*eventExecutor).startListeningAndShutdownOld.func1"), // -race variant
		goleak.IgnoreAnyFunction("github.com/open-feature/go-sdk/openfeature.(*eventExecutor).triggerEvent"),
		// Ignore Split SDK background goroutines (created during individual tests)
		goleak.IgnoreTopFunction("github.com/splitio/go-split-commons/v8/synchronizer.(*ManagerImpl).Start.func1"),
		goleak.IgnoreTopFunction("github.com/splitio/go-split-commons/v8/synchronizer.(*ManagerImpl).StartBGSync.func1"),
		// Ignore standard library goroutines
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("time.Sleep"),
	)
}

// create sets up a provider via the global OpenFeature SDK and returns a client.
// Used for high-level OpenFeature API testing (BooleanValue, StringValue, etc.).
func create(t *testing.T) *openfeature.Client {
	t.Helper()
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10 // Must be positive

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")
	require.NotNil(t, provider, "Provider should not be nil")

	// Proper cleanup: Shutdown provider when test completes
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = openfeature.ShutdownWithContext(ctx)
	})

	// Use context-aware SetProviderWithContextAndWait (gold standard)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err = openfeature.SetProviderWithContextAndWait(ctx, provider)
	require.NoError(t, err, "Failed to set provider")

	return openfeature.NewClient(testClientName)
}

func evaluationContext() openfeature.EvaluationContext {
	return openfeature.NewEvaluationContext("key", nil)
}

// =============================================================================
// Provider Creation Tests
// =============================================================================

func TestCreateSimple(t *testing.T) {
	// Test New() with configuration
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10 // Must be positive

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Provider creation should succeed")
	assert.NotNil(t, provider, "Provider should not be nil")
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()
}

// TestNewErrors tests error handling in New constructor.
func TestNewErrors(t *testing.T) {
	// Test with empty API key - should fail during factory creation
	provider, err := New("")
	assert.Error(t, err, "Empty API key should cause error")
	assert.Nil(t, provider, "Provider should be nil when creation fails")

	// Test with invalid API key format - Split SDK should reject it
	provider, err = New("invalid-key-format-!@#$%")
	// Note: Split SDK might accept any string as API key and only fail on network calls
	_ = provider
	_ = err
}

// =============================================================================
// Metadata Tests
// =============================================================================

func TestMetadataReturnsProviderName(t *testing.T) {
	ofClient := create(t)
	assert.Equal(t, testClientName, ofClient.Metadata().Domain(), "Client name should match")
	assert.Equal(t, providerNameSplit, openfeature.ProviderMetadata().Name, "Provider name should be 'Split'")
}

// =============================================================================
// Logger Configuration Tests
// =============================================================================

// TestLoggerConfiguration verifies all logger configuration scenarios work correctly.
func TestLoggerConfiguration(t *testing.T) {
	baseConfig := func() *conf.SplitSdkConfig {
		cfg := conf.Default()
		cfg.SplitFile = testSplitFile
		cfg.LoggerConfig.LogLevel = logging.LevelNone
		cfg.BlockUntilReady = 10
		return cfg
	}

	tests := []struct {
		setup                     func() (provider *Provider, customSlog *slog.Logger, customSplit *customTestLogger)
		name                      string
		expectSplitLoggerType     string
		expectProviderUsesDefault bool
	}{
		{
			name: "no logger specified uses defaults",
			setup: func() (*Provider, *slog.Logger, *customTestLogger) {
				p, err := New("localhost")
				require.NoError(t, err)
				return p, nil, nil
			},
			expectProviderUsesDefault: true,
			expectSplitLoggerType:     "adapter",
		},
		{
			name: "with logger option uses custom for both",
			setup: func() (*Provider, *slog.Logger, *customTestLogger) {
				var buf strings.Builder
				customLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
				p, err := New("localhost", WithLogger(customLogger))
				require.NoError(t, err)
				return p, customLogger, nil
			},
			expectProviderUsesDefault: false,
			expectSplitLoggerType:     "adapter",
		},
		{
			name: "split config logger only preserves custom split logger",
			setup: func() (*Provider, *slog.Logger, *customTestLogger) {
				customSplitLogger := &customTestLogger{logs: make([]string, 0)}
				cfg := baseConfig()
				cfg.Logger = customSplitLogger
				p, err := New("localhost", WithSplitConfig(cfg))
				require.NoError(t, err)
				return p, nil, customSplitLogger
			},
			expectProviderUsesDefault: true,
			expectSplitLoggerType:     "custom",
		},
		{
			name: "both loggers uses each respectively",
			setup: func() (*Provider, *slog.Logger, *customTestLogger) {
				var buf strings.Builder
				customSlogLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
				customSplitLogger := &customTestLogger{logs: make([]string, 0)}
				cfg := baseConfig()
				cfg.Logger = customSplitLogger
				p, err := New("localhost", WithLogger(customSlogLogger), WithSplitConfig(cfg))
				require.NoError(t, err)
				return p, customSlogLogger, customSplitLogger
			},
			expectProviderUsesDefault: false,
			expectSplitLoggerType:     "custom",
		},
		{
			name: "with logger and empty split config uses custom for both",
			setup: func() (*Provider, *slog.Logger, *customTestLogger) {
				var buf strings.Builder
				customLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
				cfg := baseConfig()
				p, err := New("localhost", WithLogger(customLogger), WithSplitConfig(cfg))
				require.NoError(t, err)
				return p, customLogger, nil
			},
			expectProviderUsesDefault: false,
			expectSplitLoggerType:     "adapter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, _, customSplitLogger := tt.setup()
			defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

			assert.NotNil(t, provider.logger, "Provider logger should be set")
			assert.NotNil(t, provider.splitConfig.Logger, "Split SDK logger should be set")

			switch tt.expectSplitLoggerType {
			case "adapter":
				adapter, ok := provider.splitConfig.Logger.(*SlogToSplitAdapter)
				require.True(t, ok, "Split SDK logger should be SlogToSplitAdapter")
				assert.NotNil(t, adapter.logger, "Adapter should have a logger")

			case "custom":
				assert.Equal(t, customSplitLogger, provider.splitConfig.Logger,
					"Split SDK should preserve custom logger (not overwritten)")
			}
		})
	}
}

// customTestLogger implements the Split SDK logging interface for testing.
// Thread-safe to handle concurrent calls from Split SDK goroutines.
type customTestLogger struct {
	logs []string
	mu   sync.Mutex
}

func (l *customTestLogger) Error(msg ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprint("ERROR: ", msg))
}

func (l *customTestLogger) Warning(msg ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprint("WARN: ", msg))
}

func (l *customTestLogger) Info(msg ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprint("INFO: ", msg))
}

func (l *customTestLogger) Debug(msg ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprint("DEBUG: ", msg))
}

func (l *customTestLogger) Verbose(msg ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprint("VERBOSE: ", msg))
}

// =============================================================================
// splitsChanged Pure Function Tests
// =============================================================================

func TestSplitsChanged(t *testing.T) {
	tests := []struct {
		name    string
		old     map[string]int64
		current map[string]int64
		want    bool
	}{
		{
			name:    "identical maps",
			old:     map[string]int64{"a": 1, "b": 2},
			current: map[string]int64{"a": 1, "b": 2},
			want:    false,
		},
		{
			name:    "both empty",
			old:     map[string]int64{},
			current: map[string]int64{},
			want:    false,
		},
		{
			name:    "split added",
			old:     map[string]int64{"a": 1},
			current: map[string]int64{"a": 1, "b": 2},
			want:    true,
		},
		{
			name:    "split removed",
			old:     map[string]int64{"a": 1, "b": 2},
			current: map[string]int64{"a": 1},
			want:    true,
		},
		{
			name:    "change number updated",
			old:     map[string]int64{"a": 1, "b": 2},
			current: map[string]int64{"a": 1, "b": 3},
			want:    true,
		},
		{
			name:    "split replaced",
			old:     map[string]int64{"a": 1, "b": 2},
			current: map[string]int64{"a": 1, "c": 2},
			want:    true,
		},
		{
			name:    "old nil current empty",
			old:     nil,
			current: map[string]int64{},
			want:    false,
		},
		{
			name:    "old empty current has splits",
			old:     map[string]int64{},
			current: map[string]int64{"a": 1},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitsChanged(tt.old, tt.current)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// WithMonitoringInterval Clamping Tests
// =============================================================================

func TestWithMonitoringIntervalClamping(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	tests := []struct {
		name     string
		interval time.Duration
		expected time.Duration
	}{
		{
			name:     "zero uses default",
			interval: 0,
			expected: 30 * time.Second,
		},
		{
			name:     "below minimum clamped to minimum",
			interval: 1 * time.Second,
			expected: 5 * time.Second,
		},
		{
			name:     "at minimum accepted",
			interval: 5 * time.Second,
			expected: 5 * time.Second,
		},
		{
			name:     "above minimum accepted",
			interval: 60 * time.Second,
			expected: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []Option
			if tt.interval != 0 {
				opts = append(opts, WithMonitoringInterval(tt.interval))
			}
			provider, err := New("localhost", append(opts, WithSplitConfig(cfg))...)
			require.NoError(t, err)
			defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

			assert.Equal(t, tt.expected, provider.monitoringInterval)
		})
	}
}
