package split

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/conf"
	"github.com/splitio/go-toolkit/v5/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLocalhostProvider creates a provider in localhost mode for direct provider-level testing.
// NOTE: This is distinct from the existing `create(t)` helper which returns
// *openfeature.Client for high-level OpenFeature API testing. This helper returns *Provider
// directly, needed for testing ObjectEvaluation, Track, and other provider methods with context options.
// Additional options (e.g. WithLogger) can be passed and are applied during construction,
// before InitWithContext, to avoid data races with background goroutines.
func setupLocalhostProvider(t *testing.T, opts ...Option) *Provider {
	t.Helper()
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	allOpts := append([]Option{WithSplitConfig(cfg)}, opts...)
	provider, err := New("localhost", allOpts...)
	require.NoError(t, err)

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = provider.ShutdownWithContext(ctx)
	})
	return provider
}

// testProviderEvalCtx is the standard flattened evaluation context for direct provider method tests.
var testProviderEvalCtx = openfeature.FlattenedContext{
	openfeature.TargetingKey: "key",
}

func TestObjectEvaluation_ModeIndividual_Localhost(t *testing.T) {
	provider := setupLocalhostProvider(t)

	// EvaluationModeIndividual should work in localhost (it's the default)
	ctx := WithEvaluationMode(context.Background(), EvaluationModeIndividual)

	result := provider.ObjectEvaluation(ctx, flagObj, nil, testProviderEvalCtx)

	flagSet, ok := result.Value.(FlagSetResult)
	require.True(t, ok, "Result should be FlagSetResult")
	assert.Len(t, flagSet, 1)
	assert.Contains(t, flagSet, flagObj)
}

func TestObjectEvaluation_ModeSet_IgnoredInLocalhost(t *testing.T) {
	provider := setupLocalhostProvider(t)

	// Request set mode, but localhost should ignore it and use individual
	ctx := WithEvaluationMode(context.Background(), EvaluationModeSet)

	result := provider.ObjectEvaluation(ctx, flagObj, nil, testProviderEvalCtx)

	// Should still evaluate single flag (localhost ignores EvaluationModeSet)
	flagSet, ok := result.Value.(FlagSetResult)
	require.True(t, ok, "Result should be FlagSetResult")
	assert.Len(t, flagSet, 1)
}

func TestObjectEvaluation_DefaultMode_Localhost(t *testing.T) {
	provider := setupLocalhostProvider(t)

	// Default mode in localhost should use individual
	result := provider.ObjectEvaluation(context.Background(), flagObj, nil, testProviderEvalCtx)

	flagSet, ok := result.Value.(FlagSetResult)
	require.True(t, ok, "Result should be FlagSetResult")
	assert.Len(t, flagSet, 1)
	assert.Contains(t, flagSet, flagObj)
}

func TestObjectEvaluation_LocalhostIgnoresSetMode_WithLogging(t *testing.T) {
	var logBuffer strings.Builder
	customLogger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	provider := setupLocalhostProvider(t, WithLogger(customLogger))

	// Explicitly request set mode
	ctx := WithEvaluationMode(context.Background(), EvaluationModeSet)

	// Localhost should IGNORE the set mode and use individual
	result := provider.ObjectEvaluation(ctx, flagObj, nil, testProviderEvalCtx)

	flagSet, ok := result.Value.(FlagSetResult)
	require.True(t, ok, "Result should be FlagSetResult")
	assert.Len(t, flagSet, 1)

	// Verify debug log was emitted about mode override
	assert.Contains(t, logBuffer.String(), "EvaluationModeSet ignored in localhost mode")
}

func TestImpressionDisabled_LoggedForObjectEvaluation(t *testing.T) {
	var logBuffer strings.Builder
	customLogger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	provider := setupLocalhostProvider(t, WithLogger(customLogger))

	// Set ImpressionDisabled
	ctx := WithImpressionDisabled(context.Background())

	// Should log "not yet supported" but continue evaluation
	result := provider.ObjectEvaluation(ctx, flagObj, nil, testProviderEvalCtx)

	// Evaluation should still succeed
	assert.NotNil(t, result.Value)

	// Verify "not yet supported" was logged (only once)
	assert.Contains(t, logBuffer.String(), "not yet supported by Split Go SDK")
}
