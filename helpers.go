package split

import (
	"context"
	"encoding/json"
	"sync/atomic"

	of "github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/client"
)

// Factory returns the underlying Split SDK factory for advanced use cases.
//
// The provider owns the SDK lifecycle. Do not call factory.Client().Destroy()
// or factory.Client().BlockUntilReady() directly. The factory is only valid
// between Init and Shutdown. Uses RLock because the client field is nilled
// and the factory's ready state changes during Shutdown.
//
// Example:
//
//	factory := provider.Factory()
//	// Use factory for Split-specific features not available in OpenFeature
func (p *Provider) Factory() *client.SplitFactory {
	p.mtx.RLock()
	defer p.mtx.RUnlock()
	return p.factory
}

// buildSplitAttributes creates an attributes map from FlattenedContext for Split SDK calls.
// Excludes OpenFeature-specific keys that have dedicated uses:
//   - targetingKey: used as Split's key parameter (user identifier)
//   - trafficType: used for Track() traffic type, not a targeting attribute
func buildSplitAttributes(ec of.FlattenedContext) map[string]any {
	attributes := make(map[string]any)
	for k, v := range ec {
		if k != of.TargetingKey && k != TrafficTypeKey {
			attributes[k] = v
		}
	}
	return attributes
}

// evaluateTreatmentWithConfig evaluates a flag and returns the complete treatment result.
// Returns TreatmentResult{Treatment: "control"} if shut down, or targeting key is missing/invalid.
// Uses RLock to prevent race with ShutdownWithContext; checks shutdown atomically before lock.
func (p *Provider) evaluateTreatmentWithConfig(ctx context.Context, flag string, ec of.FlattenedContext) *client.TreatmentResult {
	// Fast fail to avoid lock overhead during shutdown
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return &client.TreatmentResult{Treatment: controlTreatment, Config: nil}
	}

	p.logImpressionDisabledNotSupported(ctx, flag)

	key, ok := ec[of.TargetingKey]
	if !ok {
		p.logger.Debug("targeting key missing", "flag", flag)
		return &client.TreatmentResult{Treatment: controlTreatment, Config: nil}
	}

	keyStr, ok := key.(string)
	if !ok {
		p.logger.Debug("targeting key not a string", "flag", flag)
		return &client.TreatmentResult{Treatment: controlTreatment, Config: nil}
	}

	attributes := buildSplitAttributes(ec)

	p.mtx.RLock()
	defer p.mtx.RUnlock()

	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return &client.TreatmentResult{Treatment: controlTreatment, Config: nil}
	}

	result := p.client.TreatmentWithConfig(keyStr, flag, attributes)
	return &result
}

// evaluateTreatmentsByFlagSet evaluates all flags in a flag set and returns treatments with configs.
// Returns empty FlagSetResult on error or if the flag set has no flags (caller cannot distinguish).
// Concurrency: see evaluateTreatmentWithConfig.
func (p *Provider) evaluateTreatmentsByFlagSet(ctx context.Context, flagSet string, ec of.FlattenedContext) FlagSetResult {
	// Fast fail to avoid lock overhead during shutdown
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return make(FlagSetResult)
	}

	p.logImpressionDisabledNotSupported(ctx, flagSet)

	keyStr, ok := ec[of.TargetingKey].(string)
	if !ok {
		p.logger.Error("targeting key not a string (validation invariant violated)", "flag_set", flagSet)
		return make(FlagSetResult)
	}

	attributes := buildSplitAttributes(ec)

	p.mtx.RLock()
	defer p.mtx.RUnlock()

	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return make(FlagSetResult)
	}

	results := p.client.TreatmentsWithConfigByFlagSet(keyStr, flagSet, attributes)

	transformed := make(FlagSetResult, len(results))
	for flagName, result := range results {
		flagResult := FlagResult{
			Treatment: result.Treatment,
		}

		flagResult.Config = p.parseConfigJSON(flagName, result.Config)
		transformed[flagName] = flagResult
	}

	return transformed
}

// isLocalhostMode reports whether the provider is in localhost mode.
func (p *Provider) isLocalhostMode() bool {
	return p.splitConfig != nil && p.splitConfig.OperationMode == "localhost"
}

// evaluateSingleFlagAsObject evaluates a single flag and returns it in flag set structure.
// Returns FlagSetResult or empty map if flag not found. Concurrency: see evaluateTreatmentWithConfig.
func (p *Provider) evaluateSingleFlagAsObject(ctx context.Context, flag string, ec of.FlattenedContext) FlagSetResult {
	// Fast fail to avoid lock overhead during shutdown
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return make(FlagSetResult)
	}

	p.logImpressionDisabledNotSupported(ctx, flag)

	keyStr, ok := ec[of.TargetingKey].(string)
	if !ok {
		p.logger.Error("targeting key not a string (validation invariant violated)", "flag", flag)
		return make(FlagSetResult)
	}

	attributes := buildSplitAttributes(ec)

	p.mtx.RLock()
	defer p.mtx.RUnlock()

	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return make(FlagSetResult)
	}

	result := p.client.TreatmentWithConfig(keyStr, flag, attributes)

	if noTreatment(result.Treatment) {
		return make(FlagSetResult)
	}

	flagResult := FlagResult{
		Treatment: result.Treatment,
		Config:    p.parseConfigJSON(flag, result.Config),
	}

	return FlagSetResult{
		flag: flagResult,
	}
}

// validateEvaluationContext validates the context and evaluation context for common error conditions.
// Returns a ProviderResolutionDetail with an error if validation fails, or an empty detail if valid.
// The caller should check if Error() is not nil to determine if validation failed.
func (p *Provider) validateEvaluationContext(ctx context.Context, ec of.FlattenedContext) of.ProviderResolutionDetail {
	if p.Status() != of.ReadyState {
		return resolutionDetailProviderNotReady()
	}

	if err := ctx.Err(); err != nil {
		return resolutionDetailContextCancelled(err)
	}

	key, ok := ec[of.TargetingKey]
	if !ok {
		return resolutionDetailTargetingKeyMissing()
	}

	if _, ok := key.(string); !ok {
		return resolutionDetailInvalidContext("targeting key must be a string")
	}

	return of.ProviderResolutionDetail{}
}

// noTreatment checks if a treatment is empty or the control treatment.
func noTreatment(treatment string) bool {
	return treatment == "" || treatment == controlTreatment
}

// Resolution detail helpers map Split SDK states to OpenFeature error codes.
// See https://openfeature.dev/specification/types/#error-code
//
// TYPE_MISMATCH is not used because Split treatments are untyped strings;
// parse failures are PARSE_ERROR, not a type system mismatch.
// PROVIDER_FATAL is not used because the Split SDK returns "control" on
// fatal errors (auth failure, SDK destroyed), which maps to FLAG_NOT_FOUND.

// resolutionDetailNotFound creates a resolution detail for a flag not found error.
func resolutionDetailNotFound(variant string) of.ProviderResolutionDetail {
	return providerResolutionDetailError(
		of.NewFlagNotFoundResolutionError("flag not found"),
		of.DefaultReason,
		variant)
}

// resolutionDetailParseError creates a resolution detail for a parse error.
func resolutionDetailParseError(variant string) of.ProviderResolutionDetail {
	return providerResolutionDetailError(
		of.NewParseErrorResolutionError("cannot parse treatment to given type"),
		of.ErrorReason,
		variant)
}

// resolutionDetailTargetingKeyMissing creates a resolution detail for missing targeting key.
func resolutionDetailTargetingKeyMissing() of.ProviderResolutionDetail {
	return providerResolutionDetailError(
		of.NewTargetingKeyMissingResolutionError("targeting key missing"),
		of.ErrorReason,
		"")
}

// resolutionDetailContextCancelled creates a resolution detail for canceled context.
func resolutionDetailContextCancelled(err error) of.ProviderResolutionDetail {
	return providerResolutionDetailError(
		of.NewGeneralResolutionError(err.Error()),
		of.ErrorReason,
		"")
}

// resolutionDetailInvalidContext creates a resolution detail for invalid context.
func resolutionDetailInvalidContext(msg string) of.ProviderResolutionDetail {
	return providerResolutionDetailError(
		of.NewInvalidContextResolutionError(msg),
		of.ErrorReason,
		"")
}

// resolutionDetailProviderNotReady creates a resolution detail for provider not ready.
func resolutionDetailProviderNotReady() of.ProviderResolutionDetail {
	return providerResolutionDetailError(
		of.NewProviderNotReadyResolutionError("provider not initialized"),
		of.ErrorReason,
		"")
}

// providerResolutionDetailError creates a resolution detail with an error.
func providerResolutionDetailError(resErr of.ResolutionError, reason of.Reason, variant string) of.ProviderResolutionDetail {
	return of.ProviderResolutionDetail{
		ResolutionError: resErr,
		Reason:          reason,
		Variant:         variant,
	}
}

// resolutionDetailWithConfig creates resolution detail with Dynamic Configuration.
// Parses config JSON and adds to FlagMetadata as {"value": ...}.
// Uses TARGETING_MATCH for all successful evaluations because the Split SDK
// does not expose evaluation reason in TreatmentResult.
func (p *Provider) resolutionDetailWithConfig(flagName, variant string, config *string) of.ProviderResolutionDetail {
	detail := of.ProviderResolutionDetail{
		Reason:  of.TargetingMatchReason,
		Variant: variant,
	}

	if configData := p.parseConfigJSON(flagName, config); configData != nil {
		detail.FlagMetadata = of.FlagMetadata{"value": configData}
	}

	return detail
}

// logImpressionDisabledNotSupported logs a one-time info message if ImpressionDisabled is set.
// Called from evaluation helpers to inform users the option isn't enforced yet.
func (p *Provider) logImpressionDisabledNotSupported(ctx context.Context, _ string) {
	evalOpts := GetEvalOptions(ctx)
	if evalOpts.ImpressionDisabled {
		p.logOnce("impression_disabled", func() {
			p.logger.Info("ImpressionDisabled option detected but not yet supported by Split Go SDK",
				"note", "will be honored when SDK adds per-evaluation impression control")
		})
	}
}

// parseConfigJSON parses a Split config string into a Go value.
// Returns nil if config is nil, empty, or malformed JSON.
// Logs a warning on parse failure since it indicates invalid configuration in Split UI.
func (p *Provider) parseConfigJSON(flagName string, config *string) any {
	if config == nil || *config == "" {
		return nil
	}
	var configData any
	if err := json.Unmarshal([]byte(*config), &configData); err != nil {
		p.logger.Warn("failed to parse dynamic configuration JSON",
			"flag", flagName,
			"error", err,
			"config_preview", truncateString(*config, 100))
		return nil
	}
	return configData
}

// truncateString truncates a string to maxLen bytes, adding "..." if truncated.
// Used for logging previews of potentially large config strings.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
