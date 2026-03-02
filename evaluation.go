package split

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"

	of "github.com/open-feature/go-sdk/openfeature"
)

// FlagResult represents a single flag evaluation result.
type FlagResult struct {
	Config    any    // Parsed JSON config, or nil
	Treatment string // Split treatment name (e.g., "on", "off", "v1")
}

// FlagSetResult maps flag names to their evaluation results.
// Returned by ObjectEvaluation for flag sets and individual flag evaluations.
type FlagSetResult map[string]FlagResult

// BooleanEvaluation evaluates a feature flag and returns a boolean value.
//
// Converts Split treatments to boolean: "on" → true, "off" → false.
// Other values (including "true", "false", "1", "0") return a parse error.
// Returns def if the provider is not ready, context is canceled, targeting key is
// missing/invalid, the flag is not found, or the treatment is not "on"/"off".
//
// Context is checked before evaluation but cannot cancel in-flight SDK calls.
// See README "Known Limitations" for details.
func (p *Provider) BooleanEvaluation(ctx context.Context, flag string, def bool, ec of.FlattenedContext) of.BoolResolutionDetail {
	targetingKey, ok := ec[of.TargetingKey].(string)
	if !ok {
		targetingKey = ""
	}
	p.logger.Debug("evaluating boolean flag", "flag", flag, "targeting_key", targetingKey, "default", def)

	if validationDetail := p.validateEvaluationContext(ctx, ec); validationDetail.Error() != nil {
		p.logger.Debug("validation failed", "flag", flag, "error", validationDetail.ResolutionError.Error())
		return of.BoolResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: validationDetail,
		}
	}

	result := p.evaluateTreatmentWithConfig(ctx, flag, ec)
	p.logger.Debug("Split treatment received", "flag", flag, "treatment", result.Treatment, "has_config", result.Config != nil)

	if noTreatment(result.Treatment) {
		p.logger.Debug("flag not found or control treatment", "flag", flag, "treatment", result.Treatment)
		return of.BoolResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: resolutionDetailNotFound(result.Treatment),
		}
	}
	var value bool
	switch result.Treatment {
	case treatmentOn:
		value = true
	case treatmentOff:
		value = false
	default:
		p.logger.Warn("cannot parse treatment as boolean", "flag", flag, "treatment", result.Treatment, "returning_default", def)
		return of.BoolResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: resolutionDetailParseError(result.Treatment),
		}
	}
	p.logger.Debug("boolean evaluation successful", "flag", flag, "value", value, "treatment", result.Treatment)
	return of.BoolResolutionDetail{
		Value:                    value,
		ProviderResolutionDetail: p.resolutionDetailWithConfig(flag, result.Treatment, result.Config),
	}
}

// StringEvaluation evaluates a feature flag and returns the Split treatment as a string.
// Returns def if the provider is not ready, context is canceled, targeting key is
// missing/invalid, or the flag is not found.
func (p *Provider) StringEvaluation(ctx context.Context, flag, def string, ec of.FlattenedContext) of.StringResolutionDetail {
	targetingKey, ok := ec[of.TargetingKey].(string)
	if !ok {
		targetingKey = ""
	}
	p.logger.Debug("evaluating string flag", "flag", flag, "targeting_key", targetingKey, "default", def)

	if validationDetail := p.validateEvaluationContext(ctx, ec); validationDetail.Error() != nil {
		p.logger.Debug("validation failed", "flag", flag, "error", validationDetail.ResolutionError.Error())
		return of.StringResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: validationDetail,
		}
	}

	result := p.evaluateTreatmentWithConfig(ctx, flag, ec)
	p.logger.Debug("Split treatment received", "flag", flag, "treatment", result.Treatment, "has_config", result.Config != nil)

	if noTreatment(result.Treatment) {
		p.logger.Debug("flag not found or control treatment", "flag", flag, "treatment", result.Treatment)
		return of.StringResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: resolutionDetailNotFound(result.Treatment),
		}
	}
	p.logger.Debug("string evaluation successful", "flag", flag, "value", result.Treatment, "treatment", result.Treatment)
	return of.StringResolutionDetail{
		Value:                    result.Treatment,
		ProviderResolutionDetail: p.resolutionDetailWithConfig(flag, result.Treatment, result.Config),
	}
}

// FloatEvaluation evaluates a feature flag and parses the Split treatment as float64.
// Returns def if the provider is not ready, context is canceled, targeting key is
// missing/invalid, the flag is not found, or the treatment is not a valid float.
func (p *Provider) FloatEvaluation(ctx context.Context, flag string, def float64, ec of.FlattenedContext) of.FloatResolutionDetail {
	targetingKey, ok := ec[of.TargetingKey].(string)
	if !ok {
		targetingKey = ""
	}
	p.logger.Debug("evaluating float flag", "flag", flag, "targeting_key", targetingKey, "default", def)

	if validationDetail := p.validateEvaluationContext(ctx, ec); validationDetail.Error() != nil {
		p.logger.Debug("validation failed", "flag", flag, "error", validationDetail.ResolutionError.Error())
		return of.FloatResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: validationDetail,
		}
	}

	result := p.evaluateTreatmentWithConfig(ctx, flag, ec)
	p.logger.Debug("Split treatment received", "flag", flag, "treatment", result.Treatment, "has_config", result.Config != nil)

	if noTreatment(result.Treatment) {
		p.logger.Debug("flag not found or control treatment", "flag", flag, "treatment", result.Treatment)
		return of.FloatResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: resolutionDetailNotFound(result.Treatment),
		}
	}
	floatEvaluated, parseErr := strconv.ParseFloat(result.Treatment, 64)
	if parseErr != nil {
		p.logger.Warn("cannot parse treatment as float", "flag", flag, "treatment", result.Treatment, "error", parseErr, "returning_default", def)
		return of.FloatResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: resolutionDetailParseError(result.Treatment),
		}
	}
	p.logger.Debug("float evaluation successful", "flag", flag, "value", floatEvaluated, "treatment", result.Treatment)
	return of.FloatResolutionDetail{
		Value:                    floatEvaluated,
		ProviderResolutionDetail: p.resolutionDetailWithConfig(flag, result.Treatment, result.Config),
	}
}

// IntEvaluation evaluates a feature flag and parses the Split treatment as int64.
// Returns def if the provider is not ready, context is canceled, targeting key is
// missing/invalid, the flag is not found, or the treatment is not a valid integer.
func (p *Provider) IntEvaluation(ctx context.Context, flag string, def int64, ec of.FlattenedContext) of.IntResolutionDetail {
	targetingKey, ok := ec[of.TargetingKey].(string)
	if !ok {
		targetingKey = ""
	}
	p.logger.Debug("evaluating int flag", "flag", flag, "targeting_key", targetingKey, "default", def)

	if validationDetail := p.validateEvaluationContext(ctx, ec); validationDetail.Error() != nil {
		p.logger.Debug("validation failed", "flag", flag, "error", validationDetail.ResolutionError.Error())
		return of.IntResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: validationDetail,
		}
	}

	result := p.evaluateTreatmentWithConfig(ctx, flag, ec)
	p.logger.Debug("Split treatment received", "flag", flag, "treatment", result.Treatment, "has_config", result.Config != nil)

	if noTreatment(result.Treatment) {
		p.logger.Debug("flag not found or control treatment", "flag", flag, "treatment", result.Treatment)
		return of.IntResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: resolutionDetailNotFound(result.Treatment),
		}
	}
	intEvaluated, parseErr := strconv.ParseInt(result.Treatment, 10, 64)
	if parseErr != nil {
		p.logger.Warn("cannot parse treatment as int", "flag", flag, "treatment", result.Treatment, "error", parseErr, "returning_default", def)
		return of.IntResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: resolutionDetailParseError(result.Treatment),
		}
	}
	p.logger.Debug("int evaluation successful", "flag", flag, "value", intEvaluated, "treatment", result.Treatment)
	return of.IntResolutionDetail{
		Value:                    intEvaluated,
		ProviderResolutionDetail: p.resolutionDetailWithConfig(flag, result.Treatment, result.Config),
	}
}

// ObjectEvaluation evaluates feature flags and returns them as a FlagSetResult.
// Returns def if the provider is not ready, context is canceled, targeting key is
// missing/invalid, or no flags are found in the set.
//
// Mode of Operation:
//   - Localhost Mode: Always treats flag parameter as a single flag name
//   - Cloud Mode: Treats flag parameter as a flag set name by default;
//     use WithEvaluationMode(EvaluationModeIndividual) to evaluate as a single flag
//
// Returns FlagSetResult (map[string]FlagResult) where each FlagResult contains:
//   - Treatment: string (the Split treatment name)
//   - Config: any (parsed JSON config, supports objects/arrays/primitives, or nil)
//
// Config values support any valid JSON type. Non-object configs (primitives, arrays)
// are returned as-is in the Config field.
//
// A targeting key must be present in ec. Additional attributes in ec
// are passed to Split for targeting rule evaluation.
//
// Context is checked before evaluation but cannot cancel in-flight SDK calls.
//
// Example:
//
//	evalCtx := openfeature.NewEvaluationContext("user-123", nil)
//	result, _ := client.ObjectValue(ctx, "ui-features", split.FlagSetResult{}, evalCtx)
//	flags := result.(split.FlagSetResult)
//	theme := flags["theme"]
//	fmt.Println(theme.Treatment) // "dark"
//	fmt.Println(theme.Config)    // map[string]any{"primary": "#000"}
func (p *Provider) ObjectEvaluation(ctx context.Context, flag string, def any, ec of.FlattenedContext) of.InterfaceResolutionDetail {
	targetingKey, ok := ec[of.TargetingKey].(string)
	if !ok {
		targetingKey = ""
	}
	p.logger.Debug("evaluating object flag", "flag", flag, "targeting_key", targetingKey)

	if validationDetail := p.validateEvaluationContext(ctx, ec); validationDetail.Error() != nil {
		p.logger.Debug("validation failed", "flag", flag, "error", validationDetail.ResolutionError.Error())
		return of.InterfaceResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: validationDetail,
		}
	}

	var results FlagSetResult

	evalOpts := GetEvalOptions(ctx)

	var mode EvaluationMode
	if p.isLocalhostMode() {
		// Flag sets not supported in localhost mode
		if evalOpts.Mode == EvaluationModeSet {
			p.logger.Warn("EvaluationModeSet ignored in localhost mode, using individual evaluation",
				"requested_mode", evalOpts.Mode,
				"flag", flag)
		}
		mode = EvaluationModeIndividual
	} else {
		mode = evalOpts.Mode
		if mode == EvaluationModeDefault {
			mode = EvaluationModeSet
		}
	}

	switch mode {
	case EvaluationModeIndividual:
		p.logger.Debug("evaluating single flag as object", "flag", flag)
		results = p.evaluateSingleFlagAsObject(ctx, flag, ec)
	case EvaluationModeSet:
		p.logger.Debug("evaluating flag set", "flag_set", flag)
		results = p.evaluateTreatmentsByFlagSet(ctx, flag, ec)
	default:
		p.logger.Error("unknown evaluation mode", "mode", mode, "flag", flag)
		return of.InterfaceResolutionDetail{
			Value: def,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: of.NewGeneralResolutionError(fmt.Sprintf("unknown evaluation mode: %s", mode)),
				Reason:          of.ErrorReason,
			},
		}
	}

	if len(results) == 0 {
		p.logger.Debug("no results returned", "flag", flag, "mode", mode)
		return of.InterfaceResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: resolutionDetailNotFound(""),
		}
	}

	p.logger.Debug("object evaluation successful", "flag", flag, "flag_count", len(results), "mode", mode)
	// FlagMetadata not set; configs are in FlagResult.Config.
	return of.InterfaceResolutionDetail{
		Value: results,
		ProviderResolutionDetail: of.ProviderResolutionDetail{
			Reason:  of.TargetingMatchReason,
			Variant: flag,
		},
	}
}

// Hooks returns the provider's hooks for OpenFeature lifecycle events.
//
// Currently returns nil (no hooks implemented).
func (p *Provider) Hooks() []of.Hook {
	return nil
}

// Track sends a tracking event to Split for experimentation and analytics.
//
// Required: targetingKey in evaluationContext. Optional: trafficType attribute (defaults to "user").
// Event names must match ^[a-zA-Z0-9][-_.:a-zA-Z0-9]{0,79}$ (alphanumeric, max 80 chars,
// dash/underscore/period/colon separators). The SDK validates and returns an error on mismatch.
//
// If the provider is not ready, context is canceled, or targeting key is empty,
// the call is logged and ignored (Tracker interface defines no error return).
// Split SDK errors (invalid event name, queue full) are logged but not returned.
//
// In localhost mode, events are accepted but not persisted.
//
// Example:
//
//	evalCtx := openfeature.NewEvaluationContext("user-123", map[string]any{
//	    "trafficType": "account",
//	})
//	details := openfeature.NewTrackingEventDetails(99.99).
//	    Add("currency", "USD").
//	    Add("item_count", 3)
//	client.Track(ctx, "purchase", evalCtx, details)
func (p *Provider) Track(ctx context.Context, trackingEventName string, evaluationContext of.EvaluationContext, details of.TrackingEventDetails) {
	// Fast fail to avoid lock overhead during shutdown
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		p.logger.Debug("tracking event ignored, provider not ready",
			"event", trackingEventName)
		return
	}

	if p.Status() != of.ReadyState {
		p.logger.Debug("tracking event ignored, provider not ready",
			"event", trackingEventName)
		return
	}

	if err := ctx.Err(); err != nil {
		p.logger.Debug("tracking event ignored, context canceled",
			"event", trackingEventName,
			"error", err)
		return
	}

	key := evaluationContext.TargetingKey()
	if key == "" {
		p.logger.Warn("tracking event ignored, empty targeting key",
			"event", trackingEventName,
			"hint", "ensure evaluationContext has a non-empty TargetingKey")
		return
	}

	// Traffic type must match a defined type in Split
	trafficType := DefaultTrafficType
	if attrs := evaluationContext.Attributes(); attrs != nil {
		if tt, ok := attrs[TrafficTypeKey].(string); ok && tt != "" {
			trafficType = tt
		}
	}

	trackOpts := GetTrackOptions(ctx)

	// Split excludes nil from sum/average calculations but includes 0
	var value interface{}
	if trackOpts.MetricValueAbsent {
		value = nil
	} else {
		value = details.Value()
	}

	var properties map[string]interface{}
	attrs := details.Attributes()
	if len(attrs) > 0 {
		properties = make(map[string]interface{}, len(attrs))
		for k, v := range attrs {
			properties[k] = v
		}
	}

	p.mtx.RLock()
	defer p.mtx.RUnlock()

	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		p.logger.Debug("tracking event ignored, provider shutting down",
			"event", trackingEventName)
		return
	}

	if err := p.client.Track(key, trafficType, trackingEventName, value, properties); err != nil {
		p.logger.Error("tracking event failed",
			"event", trackingEventName,
			"key", key,
			"traffic_type", trafficType,
			"error", err)
		return
	}

	p.logger.Debug("tracking event sent",
		"event", trackingEventName,
		"key", key,
		"traffic_type", trafficType,
		"value", value,
		"value_omitted", trackOpts.MetricValueAbsent)
}
