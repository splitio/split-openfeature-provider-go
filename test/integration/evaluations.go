// evaluations.go contains flag evaluation tests for all types.
// Tests cover boolean, string, int, float, and object evaluations,
// as well as evaluation details, flag metadata, flag sets, targeting,
// context cancellation, and error handling.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
)

// testBooleanEvaluations tests boolean flag evaluations (on/off)
func testBooleanEvaluations(ctx context.Context, client *openfeature.Client) {
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	tests := []struct {
		flag     string
		expected bool
	}{
		{"feature_boolean_on", true},
		{"feature_boolean_off", false},
	}

	for _, tt := range tests {
		value, err := client.BooleanValue(ctx, tt.flag, !tt.expected, evalCtx)
		if err != nil {
			results.Fail(fmt.Sprintf("Boolean(%s)", tt.flag), err.Error())
			continue
		}

		if value != tt.expected {
			results.Fail(fmt.Sprintf("Boolean(%s)", tt.flag),
				fmt.Sprintf("expected %v, got %v", tt.expected, value))
		} else {
			results.Pass(fmt.Sprintf("Boolean(%s)", tt.flag))
		}
	}
}

// testStringEvaluations tests string flag evaluations
func testStringEvaluations(ctx context.Context, client *openfeature.Client) {
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	tests := []struct {
		flag     string
		expected string
	}{
		{"ui_theme", "dark"},
		{"api_version", "v2"},
		{"homepage_variant", "variant_b"},
	}

	for _, tt := range tests {
		value, err := client.StringValue(ctx, tt.flag, "", evalCtx)
		if err != nil {
			results.Fail(fmt.Sprintf("String(%s)", tt.flag), err.Error())
			continue
		}

		if value != tt.expected {
			results.Fail(fmt.Sprintf("String(%s)", tt.flag),
				fmt.Sprintf("expected %s, got %s", tt.expected, value))
		} else {
			results.Pass(fmt.Sprintf("String(%s)", tt.flag))
		}
	}
}

// testIntEvaluations tests integer flag evaluations
func testIntEvaluations(ctx context.Context, client *openfeature.Client) {
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	tests := []struct {
		flag     string
		expected int64
	}{
		{"max_retries", 5},
		{"page_size", 50},
		{"timeout_seconds", 30},
	}

	for _, tt := range tests {
		value, err := client.IntValue(ctx, tt.flag, 0, evalCtx)
		if err != nil {
			results.Fail(fmt.Sprintf("Int(%s)", tt.flag), err.Error())
			continue
		}

		if value != tt.expected {
			results.Fail(fmt.Sprintf("Int(%s)", tt.flag),
				fmt.Sprintf("expected %d, got %d", tt.expected, value))
		} else {
			results.Pass(fmt.Sprintf("Int(%s)", tt.flag))
		}
	}
}

// testFloatEvaluations tests float flag evaluations
func testFloatEvaluations(ctx context.Context, client *openfeature.Client) {
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	tests := []struct {
		flag     string
		expected float64
	}{
		{"discount_rate", 0.15},
		{"cache_hit_ratio", 0.85},
		{"sampling_rate", 0.01},
	}

	for _, tt := range tests {
		value, err := client.FloatValue(ctx, tt.flag, 0.0, evalCtx)
		if err != nil {
			results.Fail(fmt.Sprintf("Float(%s)", tt.flag), err.Error())
			continue
		}

		if value != tt.expected {
			results.Fail(fmt.Sprintf("Float(%s)", tt.flag),
				fmt.Sprintf("expected %.4f, got %.4f", tt.expected, value))
		} else {
			results.Pass(fmt.Sprintf("Float(%s)", tt.flag))
		}
	}
}

// testObjectEvaluations tests object flag evaluations (localhost mode only)
func testObjectEvaluations(ctx context.Context, client *openfeature.Client) {
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	// Test 1: Single flag evaluation (localhost mode)
	// Returns: {"premium_features": {"treatment": "on", "config": {...}}}
	value, err := client.ObjectValue(ctx, "premium_features", nil, evalCtx)
	if err != nil {
		results.Fail("Object(premium_features)", err.Error())
	} else {
		// Verify it's a map
		objMap, ok := value.(map[string]any)
		if !ok {
			results.Fail("Object(premium_features)", "not a map")
			return
		}

		// Check structure: should have flag name as key
		flagData, ok := objMap["premium_features"].(map[string]any)
		if !ok {
			results.Fail("Object(premium_features)", "flag data not found or invalid")
			return
		}

		// Verify treatment field
		treatment, ok := flagData["treatment"].(string)
		if !ok {
			results.Fail("Object(premium_features)", "treatment not a string")
			return
		}

		// Verify config field exists (can be nil or map)
		_, hasConfig := flagData["config"]
		if !hasConfig {
			results.Fail("Object(premium_features)", "config field missing")
			return
		}

		slog.Info("object evaluation result",
			"flag", "premium_features",
			"treatment", treatment,
			"has_config", flagData["config"] != nil)

		results.Pass("Object(premium_features)")
	}

	// Test 2: Object with configuration
	// This demonstrates accessing JSON config data attached to treatments
	value, err = client.ObjectValue(ctx, "feature_config", nil, evalCtx)
	if err != nil {
		results.Fail("Object(feature_config)", err.Error())
	} else {
		objMap, ok := value.(map[string]any)
		if !ok {
			results.Fail("Object(feature_config)", "not a map")
			return
		}

		flagData, ok := objMap["feature_config"].(map[string]any)
		if !ok {
			results.Fail("Object(feature_config)", "flag data not found")
			return
		}

		// Check if config is present and valid
		if config, ok := flagData["config"].(map[string]any); ok {
			slog.Info("config data received",
				"flag", "feature_config",
				"config_keys", len(config))
			results.Pass("Object(feature_config)")
		} else {
			results.Pass("Object(feature_config) - no config")
		}
	}
}

// testEvaluationDetails tests evaluation details (variant, reason, flagKey)
func testEvaluationDetails(ctx context.Context, client *openfeature.Client) {
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	details, err := client.BooleanValueDetails(ctx, "feature_boolean_on", false, evalCtx)
	if err != nil {
		results.Fail("BooleanDetails(variant)", err.Error())
		return
	}

	if details.Variant == "" {
		results.Fail("BooleanDetails(variant)", "variant is empty")
	} else {
		results.Pass("BooleanDetails(variant)")
	}

	if details.Reason == "" {
		results.Fail("BooleanDetails(reason)", "reason is empty")
	} else {
		results.Pass("BooleanDetails(reason)")
	}

	if details.FlagKey != "feature_boolean_on" {
		results.Fail("BooleanDetails(flagKey)", fmt.Sprintf("expected feature_boolean_on, got %s", details.FlagKey))
	} else {
		results.Pass("BooleanDetails(flagKey)")
	}
}

// testFlagMetadata tests flag metadata (configurations attached to flags)
func testFlagMetadata(ctx context.Context, client *openfeature.Client) {
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	stringDetails, err := client.StringValueDetails(ctx, "ui_theme", "light", evalCtx)
	if err != nil {
		results.Fail("FlagMetadata(string)", err.Error())
		return
	}

	if stringDetails.FlagMetadata != nil && len(stringDetails.FlagMetadata) > 0 {
		slog.Info("flag metadata in StringDetails",
			"flag", "ui_theme",
			"treatment", stringDetails.Value,
			"metadata_keys", len(stringDetails.FlagMetadata))

		if configValue, ok := stringDetails.FlagMetadata["value"]; ok {
			if configMap, ok := configValue.(map[string]any); ok {
				if primaryColor, ok := configMap["primary_color"]; ok {
					slog.Info("config field accessible", "primary_color", primaryColor)
					results.Pass("FlagMetadata(string)")
				} else {
					results.Pass("FlagMetadata(string) - no primary_color")
				}
			} else {
				results.Pass("FlagMetadata(string) - non-object config")
			}
		} else {
			results.Pass("FlagMetadata(string) - no config")
		}
	} else {
		results.Pass("FlagMetadata(string) - no metadata")
	}

	boolDetails, err := client.BooleanValueDetails(ctx, "feature_boolean_on", false, evalCtx)
	if err != nil {
		results.Fail("FlagMetadata(boolean)", err.Error())
		return
	}

	if boolDetails.FlagMetadata != nil {
		slog.Info("flag metadata in BooleanDetails",
			"flag", "feature_boolean_on",
			"treatment", boolDetails.Variant,
			"has_metadata", len(boolDetails.FlagMetadata) > 0)
		results.Pass("FlagMetadata(boolean)")
	} else {
		results.Pass("FlagMetadata(boolean) - no metadata")
	}

	intDetails, err := client.IntValueDetails(ctx, "max_retries", 3, evalCtx)
	if err != nil {
		results.Fail("FlagMetadata(int)", err.Error())
		return
	}

	if intDetails.FlagMetadata != nil {
		slog.Info("flag metadata in IntDetails",
			"flag", "max_retries",
			"value", intDetails.Value,
			"has_metadata", len(intDetails.FlagMetadata) > 0)
		results.Pass("FlagMetadata(int)")
	} else {
		results.Pass("FlagMetadata(int) - no metadata")
	}

	floatDetails, err := client.FloatValueDetails(ctx, "timeout_seconds", 5.0, evalCtx)
	if err != nil {
		results.Fail("FlagMetadata(float)", err.Error())
		return
	}

	if floatDetails.FlagMetadata != nil {
		slog.Info("flag metadata in FloatDetails",
			"flag", "timeout_seconds",
			"value", floatDetails.Value,
			"has_metadata", len(floatDetails.FlagMetadata) > 0)
		results.Pass("FlagMetadata(float)")
	} else {
		results.Pass("FlagMetadata(float) - no metadata")
	}

	details, err := client.StringValueDetails(ctx, "api_version", "v1", evalCtx)
	if err != nil {
		results.Fail("FlagMetadata(wrapped)", err.Error())
		return
	}

	if details.FlagMetadata != nil {
		if wrappedValue, ok := details.FlagMetadata["value"]; ok {
			slog.Info("config accessible via 'value' key",
				"flag", "api_version",
				"value", wrappedValue)
			results.Pass("FlagMetadata(wrapped)")
		} else {
			results.Pass("FlagMetadata(wrapped) - no value key")
		}
	} else {
		results.Pass("FlagMetadata(wrapped) - no metadata")
	}
}

// testFlagSetEvaluation tests flag set evaluation (cloud mode only)
func testFlagSetEvaluation(ctx context.Context, client *openfeature.Client) {
	evalCtx := openfeature.NewEvaluationContext("test-user", nil)

	// ============================================================
	// Test 1: Basic flag set evaluation
	// ============================================================
	flagSet := "split_provider_test"
	slog.Info("evaluating flag set", "flag_set", flagSet)

	result, err := client.ObjectValue(ctx, flagSet, nil, evalCtx)
	if err != nil {
		results.Fail("FlagSet(evaluation)", err.Error())
		return
	}

	flagSetData, ok := result.(map[string]any)
	if !ok {
		results.Fail("FlagSet(type)", fmt.Sprintf("unexpected result type: %T", result))
		return
	}

	// Should have at least 2 flags (ui_theme and api_version)
	if len(flagSetData) < 2 {
		results.Fail("FlagSet(count)", fmt.Sprintf("expected at least 2 flags, got %d", len(flagSetData)))
		return
	}
	results.Pass(fmt.Sprintf("FlagSet(count=%d)", len(flagSetData)))

	// ============================================================
	// Test 2: Verify flag structure (treatment and config fields)
	// ============================================================
	if uiTheme, ok := flagSetData["ui_theme"].(map[string]any); ok {
		// Verify treatment field exists and is a string
		if treatment, ok := uiTheme["treatment"].(string); ok {
			slog.Info("flag in set", "flag", "ui_theme", "treatment", treatment)
			results.Pass("FlagSet(ui_theme_treatment)")
		} else {
			results.Fail("FlagSet(ui_theme_treatment)", "treatment not a string")
		}

		// Verify config field exists (can be nil or any type)
		if _, hasConfig := uiTheme["config"]; hasConfig {
			results.Pass("FlagSet(ui_theme_config)")
		} else {
			results.Fail("FlagSet(ui_theme_config)", "config field missing")
		}
	} else {
		results.Fail("FlagSet(ui_theme)", "flag not found in set")
	}

	// ============================================================
	// Test 3: Verify second flag in set
	// ============================================================
	if apiVersion, ok := flagSetData["api_version"].(map[string]any); ok {
		if treatment, ok := apiVersion["treatment"].(string); ok {
			slog.Info("flag in set", "flag", "api_version", "treatment", treatment)
			results.Pass("FlagSet(api_version)")
		} else {
			results.Fail("FlagSet(api_version)", "treatment not found")
		}
	} else {
		results.Fail("FlagSet(api_version)", "flag not found in set")
	}

	// ============================================================
	// Test 4: Flag set with targeting attributes
	// ============================================================
	evalCtxWithAttr := openfeature.NewEvaluationContext("test-user-2", map[string]any{
		"variant": "two",
	})

	result2, err := client.ObjectValue(ctx, flagSet, nil, evalCtxWithAttr)
	if err != nil {
		results.Fail("FlagSet(targeting)", err.Error())
		return
	}

	flagSetData2, ok := result2.(map[string]any)
	if !ok {
		results.Fail("FlagSet(targeting_type)", fmt.Sprintf("unexpected result type: %T", result2))
		return
	}

	// Verify ui_theme returns "light" when variant=two (targeting rule)
	if uiTheme, ok := flagSetData2["ui_theme"].(map[string]any); ok {
		if treatment, ok := uiTheme["treatment"].(string); ok {
			if treatment == "light" {
				results.Pass("FlagSet(targeting_ui_theme)")
			} else {
				results.Fail("FlagSet(targeting_ui_theme)", fmt.Sprintf("expected light, got %s", treatment))
			}
		} else {
			results.Fail("FlagSet(targeting_ui_theme)", "treatment not found")
		}
	} else {
		results.Fail("FlagSet(targeting_ui_theme)", "flag not found")
	}

	// Verify api_version returns "v1" when variant=two (targeting rule)
	if apiVersion, ok := flagSetData2["api_version"].(map[string]any); ok {
		if treatment, ok := apiVersion["treatment"].(string); ok {
			if treatment == "v1" {
				results.Pass("FlagSet(targeting_api_version)")
			} else {
				results.Fail("FlagSet(targeting_api_version)", fmt.Sprintf("expected v1, got %s", treatment))
			}
		} else {
			results.Fail("FlagSet(targeting_api_version)", "treatment not found")
		}
	} else {
		results.Fail("FlagSet(targeting_api_version)", "flag not found")
	}

	// ============================================================
	// Test 5: Non-existent flag set returns default
	// ============================================================
	defaultValue := map[string]any{"fallback": true}
	result3, err := client.ObjectValue(ctx, "non_existent_flag_set", defaultValue, evalCtx)
	if err != nil {
		// Error is acceptable for non-existent flag set
		results.Pass("FlagSet(non_existent_error)")
	} else {
		// Should return default value
		if resultMap, ok := result3.(map[string]any); ok {
			if _, hasFallback := resultMap["fallback"]; hasFallback {
				results.Pass("FlagSet(non_existent_default)")
			} else if len(resultMap) == 0 {
				// Empty map is also acceptable (no flags in set)
				results.Pass("FlagSet(non_existent_empty)")
			} else {
				results.Fail("FlagSet(non_existent)", "unexpected result")
			}
		} else {
			results.Fail("FlagSet(non_existent)", fmt.Sprintf("unexpected type: %T", result3))
		}
	}

	// ============================================================
	// Test 6: ObjectValueDetails for flag set
	// ============================================================
	details, err := client.ObjectValueDetails(ctx, flagSet, nil, evalCtx)
	if err != nil {
		results.Fail("FlagSet(details)", err.Error())
		return
	}

	// Verify reason is TARGETING_MATCH
	if details.Reason == openfeature.TargetingMatchReason {
		results.Pass("FlagSet(details_reason)")
	} else {
		results.Fail("FlagSet(details_reason)", fmt.Sprintf("expected TARGETING_MATCH, got %s", details.Reason))
	}

	// Verify variant is the flag set name
	if details.Variant == flagSet {
		results.Pass("FlagSet(details_variant)")
	} else {
		results.Fail("FlagSet(details_variant)", fmt.Sprintf("expected %s, got %s", flagSet, details.Variant))
	}

	// Verify value is a map with flags
	if detailsValue, ok := details.Value.(map[string]any); ok {
		if len(detailsValue) >= 2 {
			results.Pass("FlagSet(details_value)")
		} else {
			results.Fail("FlagSet(details_value)", fmt.Sprintf("expected at least 2 flags, got %d", len(detailsValue)))
		}
	} else {
		results.Fail("FlagSet(details_value)", "value not a map")
	}
}

// testAttributeTargeting tests targeting with evaluation context attributes
func testAttributeTargeting(ctx context.Context, client *openfeature.Client) {
	evalCtx1 := openfeature.NewEvaluationContext("test-user", map[string]any{
		"email": "vip@example.com",
		"plan":  "enterprise",
		"age":   int64(30),
	})

	value, err := client.StringValue(ctx, "ui_theme", "light", evalCtx1)
	if err != nil {
		results.Fail("Attributes(with_attrs)", err.Error())
	} else if value != "dark" {
		results.Fail("Attributes(with_attrs)", fmt.Sprintf("expected dark, got %s", value))
	} else {
		results.Pass("Attributes(with_attrs)")
	}

	evalCtx2 := openfeature.NewEvaluationContext("another-user", map[string]any{
		"email":   "user@example.com",
		"plan":    "basic",
		"premium": false,
	})

	value, err = client.StringValue(ctx, "api_version", "v1", evalCtx2)
	if err != nil {
		results.Fail("Attributes(different_user)", err.Error())
	} else if value != "v2" {
		results.Fail("Attributes(different_user)", fmt.Sprintf("expected v2, got %s", value))
	} else {
		results.Pass("Attributes(different_user)")
	}
}

// testContextCancellation tests behavior when context is cancelled
func testContextCancellation(client *openfeature.Client) {

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	evalCtx := openfeature.NewEvaluationContext("test-user", nil)
	value, err := client.BooleanValue(ctx, "feature_boolean_on", false, evalCtx)

	if err == nil {
		results.Fail("Context(cancellation)", "expected error for cancelled context")
	} else if value != false {
		results.Fail("Context(cancellation)", "should return default value on cancellation")
	} else {
		results.Pass("Context(cancellation)")
	}
}

// testErrorHandling tests error handling for invalid inputs
func testErrorHandling(ctx context.Context, client *openfeature.Client) {
	evalCtx := openfeature.NewEvaluationContext("", nil)
	value, err := client.BooleanValue(ctx, "feature_boolean_on", false, evalCtx)

	if err == nil {
		results.Fail("Error(missing_key)", "expected error for empty targeting key")
	} else if value != false {
		results.Fail("Error(missing_key)", "should return default on error")
	} else {
		results.Pass("Error(missing_key)")
	}

	evalCtx2 := openfeature.NewEvaluationContext("test-user", nil)
	value, err = client.BooleanValue(ctx, "non_existent_flag", true, evalCtx2)

	if err == nil {
		results.Fail("Error(non_existent)", "expected error for non-existent flag")
	} else if value != true {
		results.Fail("Error(non_existent)", "should return default for non-existent flag")
	} else {
		results.Pass("Error(non_existent)")
	}
}
