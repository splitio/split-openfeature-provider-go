package split

import (
	"context"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
)

func TestWithEvalOptions(t *testing.T) {
	ctx := context.Background()

	ctx = WithEvalOptions(ctx, EvalOptions{
		Mode:               EvaluationModeIndividual,
		ImpressionDisabled: true,
	})

	opts := GetEvalOptions(ctx)
	assert.Equal(t, EvaluationModeIndividual, opts.Mode)
	assert.True(t, opts.ImpressionDisabled)
}

func TestWithEvalOptions_NotSet(t *testing.T) {
	ctx := context.Background()
	opts := GetEvalOptions(ctx)

	assert.Equal(t, EvaluationModeDefault, opts.Mode)
	assert.False(t, opts.ImpressionDisabled)
}

func TestWithTrackOptions_MetricValueAbsent(t *testing.T) {
	ctx := context.Background()
	ctx = WithoutMetricValue(ctx)

	opts := GetTrackOptions(ctx)
	assert.True(t, opts.MetricValueAbsent)
}

func TestWithEvalOptions_MultipleCalls_MergesCorrectly(t *testing.T) {
	ctx := context.Background()

	// Set mode first
	ctx = WithEvaluationMode(ctx, EvaluationModeIndividual)

	// Set ImpressionDisabled second - should preserve mode
	ctx = WithImpressionDisabled(ctx)

	opts := GetEvalOptions(ctx)
	assert.Equal(t, EvaluationModeIndividual, opts.Mode)
	assert.True(t, opts.ImpressionDisabled)
}

func TestWithTrackOptions_MultipleCalls_MergesCorrectly(t *testing.T) {
	ctx := context.Background()

	ctx = WithoutMetricValue(ctx)

	opts := GetTrackOptions(ctx)
	assert.True(t, opts.MetricValueAbsent)
}

func TestContext_PassedThroughMiddlewareChain(t *testing.T) {
	ctx := context.Background()
	ctx = WithEvaluationMode(ctx, EvaluationModeIndividual)
	ctx = WithImpressionDisabled(ctx)
	ctx = WithoutMetricValue(ctx)

	evalOpts := GetEvalOptions(ctx)
	trackOpts := GetTrackOptions(ctx)

	assert.Equal(t, EvaluationModeIndividual, evalOpts.Mode)
	assert.True(t, evalOpts.ImpressionDisabled)
	assert.True(t, trackOpts.MetricValueAbsent)
}

func TestTrack_MetricValueAbsent_TakesPrecedence(t *testing.T) {
	// Set MetricValueAbsent even though details has a non-zero value
	ctx := WithoutMetricValue(context.Background())
	details := openfeature.NewTrackingEventDetails(99.99) // Has value!

	// Verify that MetricValueAbsent takes precedence in context
	trackOpts := GetTrackOptions(ctx)
	assert.True(t, trackOpts.MetricValueAbsent, "MetricValueAbsent should be true")

	// The value in details doesn't affect the context option
	assert.Equal(t, 99.99, details.Value(), "Details still has the value")
}
