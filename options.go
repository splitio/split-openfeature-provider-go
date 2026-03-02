package split

import "context"

type evalOptionsKeyType struct{}
type trackOptionsKeyType struct{}

var (
	evalOptionsKey  = evalOptionsKeyType{}
	trackOptionsKey = trackOptionsKeyType{}
)

// EvaluationMode controls how ObjectEvaluation interprets the flag parameter.
type EvaluationMode string

const (
	// EvaluationModeDefault uses the provider's default behavior.
	// The actual mode is determined at evaluation time based on provider mode:
	// - Cloud mode: flag set evaluation (TreatmentsWithConfigByFlagSet)
	// - Localhost mode: individual flag evaluation (TreatmentWithConfig)
	EvaluationModeDefault EvaluationMode = ""

	// EvaluationModeSet treats the flag parameter as a flag set name.
	// Uses TreatmentsWithConfigByFlagSet.
	EvaluationModeSet EvaluationMode = "set"

	// EvaluationModeIndividual treats the flag parameter as a single flag name.
	// Uses TreatmentWithConfig.
	EvaluationModeIndividual EvaluationMode = "individual"
)

// EvalOptions contains per-request evaluation options.
type EvalOptions struct {
	// Mode controls set vs individual evaluation in ObjectEvaluation.
	// Ignored in localhost mode (always individual).
	Mode EvaluationMode

	// ImpressionDisabled disables impression tracking for this evaluation.
	// Useful for health checks, load tests, internal tools.
	//
	// This mirrors Split Evaluator's per-request `impressionsDisabled` parameter.
	// Split SDK-level impression modes (OPTIMIZED/DEBUG/NONE) are configured at
	// initialization time via cfg.ImpressionsMode - those are NOT per-request.
	//
	// NOTE: Forward-looking API - not yet supported by Split Go SDK.
	// Will be logged but not enforced until SDK adds per-evaluation support.
	ImpressionDisabled bool
}

// WithEvalOptions adds evaluation options to context.
func WithEvalOptions(ctx context.Context, opts EvalOptions) context.Context {
	return context.WithValue(ctx, evalOptionsKey, opts)
}

// GetEvalOptions extracts evaluation options from context.
// Returns zero value EvalOptions if not set.
func GetEvalOptions(ctx context.Context) EvalOptions {
	if opts, ok := ctx.Value(evalOptionsKey).(EvalOptions); ok {
		return opts
	}
	return EvalOptions{}
}

// WithEvaluationMode sets only the evaluation mode.
func WithEvaluationMode(ctx context.Context, mode EvaluationMode) context.Context {
	opts := GetEvalOptions(ctx)
	opts.Mode = mode
	return WithEvalOptions(ctx, opts)
}

// WithImpressionDisabled disables impression tracking for evaluations on this context.
// NOTE: Forward-looking API - not yet enforced by Split Go SDK.
func WithImpressionDisabled(ctx context.Context) context.Context {
	opts := GetEvalOptions(ctx)
	opts.ImpressionDisabled = true
	return WithEvalOptions(ctx, opts)
}

// TrackOptions contains per-request tracking options.
type TrackOptions struct {
	// MetricValueAbsent indicates that no metric value was provided.
	// When true, the provider passes nil to Split instead of 0.
	// This prevents polluting sum/average metrics with zeros.
	MetricValueAbsent bool
}

// WithTrackOptions adds tracking options to context.
func WithTrackOptions(ctx context.Context, opts TrackOptions) context.Context {
	return context.WithValue(ctx, trackOptionsKey, opts)
}

// GetTrackOptions extracts tracking options from context.
// Returns zero value TrackOptions if not set.
func GetTrackOptions(ctx context.Context) TrackOptions {
	if opts, ok := ctx.Value(trackOptionsKey).(TrackOptions); ok {
		return opts
	}
	return TrackOptions{}
}

// WithoutMetricValue marks that no metric value should be sent.
func WithoutMetricValue(ctx context.Context) context.Context {
	opts := GetTrackOptions(ctx)
	opts.MetricValueAbsent = true
	return WithTrackOptions(ctx, opts)
}
