package split

import (
	"context"
	"strconv"

	of "github.com/open-feature/go-sdk/openfeature"
)

// BooleanEvaluation evaluates a feature flag and returns a boolean value.
//
// The method converts Split treatments to boolean values:
//   - "on" → true
//   - "off" → false
//   - Other values → parse error, returns def
//
// A targeting key must be present in ec. Additional attributes in ec
// are passed to Split for targeting rule evaluation.
//
// Context Cancellation Limitation:
// The ctx parameter is checked BEFORE evaluation starts, but the Split SDK does
// not support canceling in-flight evaluations. Once evaluation begins, it runs to
// completion. Evaluations are typically very fast (<1ms from cache), so this is
// rarely an issue. See README "Known Limitations" for details.
//
// Returns the def if:
//   - Context is canceled or deadline exceeded (checked before evaluation)
//   - Targeting key is missing
//   - Flag is not found
//   - Treatment cannot be parsed as boolean
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

	result := p.evaluateTreatmentWithConfig(flag, ec)
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
	case "on":
		value = true
	case "off":
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

// StringEvaluation evaluates a feature flag and returns a string value.
//
// The method returns the Split treatment directly as a string. This is the most
// common evaluation type as Split treatments are inherently string-based.
//
// A targeting key must be present in ec. Additional attributes in ec
// are passed to Split for targeting rule evaluation.
//
// Context Cancellation Limitation:
// The ctx parameter is checked BEFORE evaluation starts, but the Split SDK does
// not support canceling in-flight evaluations. See README "Known Limitations".
//
// Returns the def if:
//   - Context is canceled or deadline exceeded (checked before evaluation)
//   - Targeting key is missing
//   - Flag is not found (treatment is "control" or empty)
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

	result := p.evaluateTreatmentWithConfig(flag, ec)
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

// FloatEvaluation evaluates a feature flag and returns a float64 value.
//
// The method parses the Split treatment as a floating-point number. This is useful
// for flags that control numeric values like pricing, weights, or percentages.
//
// A targeting key must be present in ec. Additional attributes in ec
// are passed to Split for targeting rule evaluation.
//
// Context Cancellation Limitation:
// The ctx parameter is checked BEFORE evaluation starts, but the Split SDK does
// not support canceling in-flight evaluations. See README "Known Limitations".
//
// Returns the def if:
//   - Context is canceled or deadline exceeded (checked before evaluation)
//   - Targeting key is missing
//   - Flag is not found
//   - Treatment cannot be parsed as a valid float64
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

	result := p.evaluateTreatmentWithConfig(flag, ec)
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

// IntEvaluation evaluates a feature flag and returns an int64 value.
//
// The method parses the Split treatment as a 64-bit integer. This is useful for
// flags that control counts, limits, timeouts, or other integer-based values.
//
// A targeting key must be present in ec. Additional attributes in ec
// are passed to Split for targeting rule evaluation.
//
// Context Cancellation Limitation:
// The ctx parameter is checked BEFORE evaluation starts, but the Split SDK does
// not support canceling in-flight evaluations. See README "Known Limitations".
//
// Returns the def if:
//   - Context is canceled or deadline exceeded (checked before evaluation)
//   - Targeting key is missing
//   - Flag is not found
//   - Treatment cannot be parsed as a valid int64
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

	result := p.evaluateTreatmentWithConfig(flag, ec)
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

// ObjectEvaluation evaluates feature flags and returns them as structured objects.
//
// Mode of Operation:
//   - Localhost Mode: Treats flag parameter as a single flag name
//   - Production Mode: Treats flag parameter as a flag set name
//
// Returns map[string]any where each entry contains:
//   - "treatment": string (the Split treatment name)
//   - "config": any (parsed JSON config, supports objects/arrays/primitives, or nil)
//
// Config values support any valid JSON type. Non-object configs (primitives, arrays)
// are returned as-is in the "config" field.
//
// A targeting key must be present in ec. Additional attributes in ec
// are passed to Split for targeting rule evaluation.
//
// Context Cancellation Limitation:
// The ctx parameter is checked BEFORE evaluation starts, but the Split SDK does
// not support canceling in-flight evaluations. See README "Known Limitations".
//
// Returns def if context canceled (before evaluation), targeting key missing, or flag/flag set not found.
//
// Example:
//
//	ctx := of.NewEvaluationContext("user-123", nil)
//	result, _ := client.ObjectValue(context.Background(), "ui-features", nil, ctx)
//	// result = {
//	//   "theme": {"treatment": "dark", "config": {"primary": "#000"}},
//	//   "layout": {"treatment": "grid", "config": null}
//	// }
func (p *Provider) ObjectEvaluation(ctx context.Context, flag string, def any, ec of.FlattenedContext) of.InterfaceResolutionDetail {
	targetingKey, ok := ec[of.TargetingKey].(string)
	if !ok {
		targetingKey = ""
	}
	mode := "flag_set"
	if p.isLocalhostMode() {
		mode = "single_flag"
	}
	p.logger.Debug("evaluating object", "flag", flag, "targeting_key", targetingKey, "mode", mode)

	if validationDetail := p.validateEvaluationContext(ctx, ec); validationDetail.Error() != nil {
		p.logger.Debug("validation failed", "flag", flag, "error", validationDetail.ResolutionError.Error())
		return of.InterfaceResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: validationDetail,
		}
	}

	var results map[string]any

	// Dual-mode: localhost uses single flag, production uses flag sets
	if p.isLocalhostMode() {
		// Localhost mode: treat as single flag name
		p.logger.Debug("evaluating single flag as object", "flag", flag)
		results = p.evaluateSingleFlagAsObject(flag, ec)
	} else {
		// Production mode: treat as flag set name
		p.logger.Debug("evaluating flag set", "flag_set", flag)
		results = p.evaluateTreatmentsByFlagSet(flag, ec)
	}

	if len(results) == 0 {
		p.logger.Debug("no results returned", "flag", flag, "mode", mode)
		return of.InterfaceResolutionDetail{
			Value:                    def,
			ProviderResolutionDetail: resolutionDetailNotFound(""),
		}
	}

	p.logger.Debug("object evaluation successful", "flag", flag, "flag_count", len(results), "mode", mode)
	return of.InterfaceResolutionDetail{
		Value: results,
		ProviderResolutionDetail: of.ProviderResolutionDetail{
			Reason:       of.TargetingMatchReason,
			Variant:      flag,
			FlagMetadata: nil,
		},
	}
}

// Hooks returns the provider's hooks for OpenFeature lifecycle events.
//
// Currently, this provider does not implement any hooks and returns nil.
// Future versions may add hooks for:
//   - Telemetry and metrics collection
//   - Logging and debugging
//   - Custom evaluation context enrichment
func (p *Provider) Hooks() []of.Hook {
	return nil
}
