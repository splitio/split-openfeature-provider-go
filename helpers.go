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
// ⚠️  ADVANCED USAGE - Lifecycle Management Warning:
//
// The provider manages the Split SDK lifecycle (initialization, shutdown, cleanup).
// When using Factory() directly, you must be aware of these constraints:
//
//  1. DO NOT call factory.Client().Destroy() - the provider owns SDK lifecycle
//  2. DO NOT call factory.Client().BlockUntilReady() - use provider.Status() instead
//  3. The factory is only valid between Init and Shutdown
//  4. After Shutdown(), the factory and client are destroyed - any direct usage will fail
//
// See https://github.com/splitio/go-client for Split SDK documentation.
//
// Concurrency Safety:
// Uses read lock for consistency with Status() and Metrics() methods.
// Even though factory is never reassigned after New(), synchronization is required
// to prevent data race warnings when other goroutines hold write locks.
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

// evaluateTreatmentWithConfig evaluates a flag and returns the complete treatment result.
// Returns TreatmentResult{Treatment: "control", Config: nil} if targeting key is missing or invalid.
//
// Concurrency Safety:
// Uses read lock during client call to prevent race with ShutdownWithContext.
// This ensures the client is not destroyed while an evaluation is in progress.
// Checks shutdown flag atomically before acquiring lock for fast-fail during shutdown.
func (p *Provider) evaluateTreatmentWithConfig(flag string, ec of.FlattenedContext) *client.TreatmentResult {
	// Check shutdown first (fast fail before lock to prevent deadlock)
	// If shutdown is in progress, return control treatment immediately
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return &client.TreatmentResult{Treatment: controlTreatment, Config: nil}
	}

	key, ok := ec[of.TargetingKey]
	if !ok {
		return &client.TreatmentResult{Treatment: controlTreatment, Config: nil}
	}

	keyStr, ok := key.(string)
	if !ok {
		return &client.TreatmentResult{Treatment: controlTreatment, Config: nil}
	}

	// Build attributes map (excluding targeting key)
	attributes := make(map[string]any)
	for k, v := range ec {
		if k != of.TargetingKey {
			attributes[k] = v
		}
	}

	// Acquire read lock for client access to prevent concurrent shutdown
	// This prevents client.Destroy() from being called during evaluation
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	// Double-check shutdown after acquiring lock to prevent nil pointer dereference
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return &client.TreatmentResult{Treatment: controlTreatment, Config: nil}
	}

	result := p.client.TreatmentWithConfig(keyStr, flag, attributes)
	return &result
}

// evaluateTreatmentsByFlagSet evaluates all flags in a flag set and returns treatments with configs.
// Returns map[flagName]{"treatment": string, "config": any}.
// Config supports any valid JSON type (objects, arrays, primitives).
// Assumes targeting key validated by caller as string.
//
// Concurrency Safety:
// Uses read lock during client call to prevent race with ShutdownWithContext.
// This ensures the client is not destroyed while an evaluation is in progress.
// Checks shutdown flag atomically before acquiring lock for fast-fail during shutdown.
func (p *Provider) evaluateTreatmentsByFlagSet(flagSet string, ec of.FlattenedContext) map[string]any {
	// Check shutdown first (fast fail before lock to prevent deadlock)
	// If shutdown is in progress, return empty map immediately
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return make(map[string]any)
	}

	// Extract targeting key (already validated by caller as string)
	keyStr, ok := ec[of.TargetingKey].(string)
	if !ok {
		// Should never happen due to validation, but be defensive
		return make(map[string]any)
	}

	// Build attributes map (excluding targeting key)
	attributes := make(map[string]any)
	for k, v := range ec {
		if k != of.TargetingKey {
			attributes[k] = v
		}
	}

	// Acquire read lock for client access to prevent concurrent shutdown
	// This prevents client.Destroy() from being called during evaluation
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	// Double-check shutdown after acquiring lock to prevent nil pointer dereference
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return make(map[string]any)
	}

	results := p.client.TreatmentsWithConfigByFlagSet(keyStr, flagSet, attributes)

	// Transform the results: parse config strings into any valid JSON
	transformed := make(map[string]any, len(results))
	for flagName, result := range results {
		flagResult := map[string]any{
			"treatment": result.Treatment,
		}

		// Parse config string into any valid JSON value if present
		if result.Config != nil && *result.Config != "" {
			var configData any
			if err := json.Unmarshal([]byte(*result.Config), &configData); err == nil {
				flagResult["config"] = configData
			} else {
				// Log warning for malformed JSON config - this indicates invalid configuration in Split UI
				p.logger.Warn("failed to parse dynamic configuration JSON",
					"flag", flagName,
					"error", err,
					"config_preview", truncateString(*result.Config, 100))
				flagResult["config"] = nil
			}
		} else {
			flagResult["config"] = nil
		}

		transformed[flagName] = flagResult
	}

	return transformed
}

// isLocalhostMode checks if the provider is running in localhost mode.
// Localhost mode is detected by checking the OperationMode set by the Split SDK.
// When API key is "localhost", Split SDK automatically sets OperationMode to "localhost".
// This method is concurrent-safe as it only reads the immutable splitConfig.
func (p *Provider) isLocalhostMode() bool {
	return p.splitConfig != nil && p.splitConfig.OperationMode == "localhost"
}

// evaluateSingleFlagAsObject evaluates a single flag and returns it in flag set structure.
// Returns map[flagName]{"treatment": string, "config": any} or empty map if flag not found.
// Assumes targeting key validated by caller as string.
//
// Concurrency Safety:
// Uses read lock during client call to prevent race with ShutdownWithContext.
// This ensures the client is not destroyed while an evaluation is in progress.
// Checks shutdown flag atomically before acquiring lock for fast-fail during shutdown.
func (p *Provider) evaluateSingleFlagAsObject(flag string, ec of.FlattenedContext) map[string]any {
	// Check shutdown first (fast fail before lock to prevent deadlock)
	// If shutdown is in progress, return empty map immediately
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return make(map[string]any)
	}

	// Extract targeting key (already validated by caller as string)
	keyStr, ok := ec[of.TargetingKey].(string)
	if !ok {
		// Should never happen due to validation, but be defensive
		return make(map[string]any)
	}

	// Build attributes map (excluding targeting key)
	attributes := make(map[string]any)
	for k, v := range ec {
		if k != of.TargetingKey {
			attributes[k] = v
		}
	}

	// Acquire read lock for client access to prevent concurrent shutdown
	// This prevents client.Destroy() from being called during evaluation
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	// Double-check shutdown after acquiring lock to prevent nil pointer dereference
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return make(map[string]any)
	}

	result := p.client.TreatmentWithConfig(keyStr, flag, attributes)

	// If treatment is control or empty, return empty map (flag not found)
	if noTreatment(result.Treatment) {
		return make(map[string]any)
	}

	// Build result in same structure as flag sets: map[flagName]map[treatment+config]
	flagResult := map[string]any{
		"treatment": result.Treatment,
	}

	// Parse config string into any valid JSON value if present
	if result.Config != nil && *result.Config != "" {
		var configData any
		if err := json.Unmarshal([]byte(*result.Config), &configData); err == nil {
			flagResult["config"] = configData
		} else {
			// Log warning for malformed JSON config - this indicates invalid configuration in Split UI
			p.logger.Warn("failed to parse dynamic configuration JSON",
				"flag", flag,
				"error", err,
				"config_preview", truncateString(*result.Config, 100))
			flagResult["config"] = nil
		}
	} else {
		flagResult["config"] = nil
	}

	// Return single-entry map with flag name as key
	return map[string]any{
		flag: flagResult,
	}
}

// validateEvaluationContext validates the context and evaluation context for common error conditions.
// Returns a ProviderResolutionDetail with an error if validation fails, or an empty detail if valid.
// The caller should check if Error() is not nil to determine if validation failed.
// Note: This is a method on Provider to access Status(), but takes ctx and ec as parameters.
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

// ========================================
// OpenFeature Error Code Implementation
// ========================================
//
// This provider implements all applicable OpenFeature error codes per the spec:
// https://openfeature.dev/specification/types/#error-code
//
// IMPLEMENTED ERROR CODES:
//
// 1. PROVIDER_NOT_READY - Provider has not been initialized or is shut down
//    Used in: validateEvaluationContext when p.Status() != ReadyState
//
// 2. FLAG_NOT_FOUND - Flag does not exist in Split
//    Used in: All evaluation methods when Split returns "control" treatment
//
// 3. PARSE_ERROR - Treatment value cannot be parsed to requested type
//    Used in: Boolean/Int/Float evaluation when strconv.Parse* fails
//    Note: Split treatments are always strings, so this is correct for parse failures
//
// 4. TARGETING_KEY_MISSING - Evaluation context has no targeting key
//    Used in: validateEvaluationContext when ec[TargetingKey] is not present
//
// 5. INVALID_CONTEXT - Evaluation context is malformed
//    Used in: validateEvaluationContext when targeting key exists but is not a string
//
// 6. GENERAL - Context canceled, deadline exceeded, or other errors
//    Used in: validateEvaluationContext when ctx.Err() != nil
//
// NOT APPLICABLE ERROR CODES:
//
// 7. TYPE_MISMATCH - Flag value type does not match expected type
//    Why not used: Split treatments are untyped strings. We always attempt to parse
//    them to the requested type. When parsing fails, it's a PARSE_ERROR (the string
//    cannot be parsed), not a TYPE_MISMATCH (Split doesn't have a native type system
//    where a flag could be "configured as a boolean" vs "configured as a string").
//    TYPE_MISMATCH would be appropriate for providers with typed flag systems.
//
// 8. PROVIDER_FATAL - Provider encountered an unrecoverable error
//    Why not used: Split SDK does not expose fatal runtime errors during evaluation.
//    When the SDK cannot evaluate (auth failure, network issues, SDK destroyed), it
//    returns the "control" treatment which we handle as FLAG_NOT_FOUND. Provider
//    initialization failures are handled by returning errors from New()/Init(), not
//    by returning PROVIDER_FATAL during evaluations.

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
// Parses config JSON and adds to FlagMetadata. Non-object configs (primitives, arrays)
// are wrapped as {"value": ...} to satisfy FlagMetadata's map[string]any requirement.
// This is a receiver method (unlike other resolutionDetail* helpers) to enable logging
// of malformed JSON warnings.
//
// ENHANCEMENT NOTE for Split SDK:
// OpenFeature defines 8 semantic reason codes to indicate WHY a flag value was returned:
//   - TARGETING_MATCH: Dynamic evaluation based on user targeting rules
//   - SPLIT: Pseudorandom assignment (A/B test, traffic allocation)
//   - STATIC: Static value with no dynamic evaluation
//   - CACHED: Value retrieved from cache
//   - DEFAULT: Flag not found, returned default value
//   - DISABLED: Flag disabled in management system
//   - UNKNOWN: Reason could not be determined
//   - ERROR: Error occurred during evaluation
//
// Currently, we use TARGETING_MATCH for ALL successful evaluations because the Split SDK
// does not expose the evaluation reason in its TreatmentResult. The SDK internally knows
// whether the treatment came from:
//   - Targeted rule matching (user attributes matched targeting rules) → TARGETING_MATCH
//   - Traffic allocation / A/B test (pseudorandom split) → SPLIT
//   - Default treatment (no targeting, simple value) → STATIC
//   - Cached value (serving from local cache) → CACHED
//
// To properly implement OpenFeature reason codes, the Split Go SDK would need to expose
// this information, perhaps by adding a "Reason" field to the TreatmentResult struct
// returned by GetTreatmentWithConfig(). This would enable OpenFeature providers to
// accurately report the semantic reason for each evaluation.
func (p *Provider) resolutionDetailWithConfig(flagName, variant string, config *string) of.ProviderResolutionDetail {
	detail := of.ProviderResolutionDetail{
		Reason:  of.TargetingMatchReason, // See ENHANCEMENT NOTE above
		Variant: variant,
	}

	// If Dynamic Configuration is present, parse it and add to FlagMetadata
	if config != nil && *config != "" {
		var configData any
		if err := json.Unmarshal([]byte(*config), &configData); err == nil {
			detail.FlagMetadata = of.FlagMetadata{"value": configData}
		} else {
			p.logger.Warn("failed to parse dynamic configuration JSON",
				"flag", flagName,
				"error", err,
				"config_preview", truncateString(*config, 100))
		}
	}

	return detail
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated.
// Used for logging previews of potentially large config strings.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
