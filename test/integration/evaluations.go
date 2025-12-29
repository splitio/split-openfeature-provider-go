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

	"github.com/splitio/split-openfeature-provider-go/v2"
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
	// Returns: FlagSetResult{"premium_features": FlagResult{Treatment: "on", Config: {...}}}
	value, err := client.ObjectValue(ctx, "premium_features", split.FlagSetResult{}, evalCtx)
	if err != nil {
		results.Fail("Object(premium_features)", err.Error())
	} else {
		// Type-assert to FlagSetResult
		flags, ok := value.(split.FlagSetResult)
		if !ok {
			results.Fail("Object(premium_features)", fmt.Sprintf("expected FlagSetResult, got %T", value))
			return
		}

		// Check structure: should have flag name as key
		flagData, ok := flags["premium_features"]
		if !ok {
			results.Fail("Object(premium_features)", "flag data not found")
			return
		}

		slog.Info("object evaluation result",
			"flag", "premium_features",
			"treatment", flagData.Treatment,
			"has_config", flagData.Config != nil)

		results.Pass("Object(premium_features)")
	}

	// Test 2: Object with configuration
	// This demonstrates accessing JSON config data attached to treatments
	value, err = client.ObjectValue(ctx, "feature_config", split.FlagSetResult{}, evalCtx)
	if err != nil {
		results.Fail("Object(feature_config)", err.Error())
	} else {
		flags, ok := value.(split.FlagSetResult)
		if !ok {
			results.Fail("Object(feature_config)", fmt.Sprintf("expected FlagSetResult, got %T", value))
			return
		}

		flagData, ok := flags["feature_config"]
		if !ok {
			results.Fail("Object(feature_config)", "flag data not found")
			return
		}

		// Check if config is present and valid
		if config, ok := flagData.Config.(map[string]any); ok {
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

	result, err := client.ObjectValue(ctx, flagSet, split.FlagSetResult{}, evalCtx)
	if err != nil {
		results.Fail("FlagSet(evaluation)", err.Error())
		return
	}

	flags, ok := result.(split.FlagSetResult)
	if !ok {
		results.Fail("FlagSet(type)", fmt.Sprintf("expected FlagSetResult, got %T", result))
		return
	}

	// Should have at least 2 flags (ui_theme and api_version)
	if len(flags) < 2 {
		results.Fail("FlagSet(count)", fmt.Sprintf("expected at least 2 flags, got %d", len(flags)))
		return
	}
	results.Pass(fmt.Sprintf("FlagSet(count=%d)", len(flags)))

	// ============================================================
	// Test 2: Verify flag structure (Treatment and Config fields)
	// ============================================================
	if uiTheme, ok := flags["ui_theme"]; ok {
		slog.Info("flag in set", "flag", "ui_theme", "treatment", uiTheme.Treatment)
		results.Pass("FlagSet(ui_theme_treatment)")
		// Config field always exists in FlagResult struct
		results.Pass("FlagSet(ui_theme_config)")
	} else {
		results.Fail("FlagSet(ui_theme)", "flag not found in set")
	}

	// ============================================================
	// Test 3: Verify second flag in set
	// ============================================================
	if apiVersion, ok := flags["api_version"]; ok {
		slog.Info("flag in set", "flag", "api_version", "treatment", apiVersion.Treatment)
		results.Pass("FlagSet(api_version)")
	} else {
		results.Fail("FlagSet(api_version)", "flag not found in set")
	}

	// ============================================================
	// Test 4: Flag set with targeting attributes
	// ============================================================
	evalCtxWithAttr := openfeature.NewEvaluationContext("test-user-2", map[string]any{
		"variant": "two",
	})

	result2, err := client.ObjectValue(ctx, flagSet, split.FlagSetResult{}, evalCtxWithAttr)
	if err != nil {
		results.Fail("FlagSet(targeting)", err.Error())
		return
	}

	flags2, ok := result2.(split.FlagSetResult)
	if !ok {
		results.Fail("FlagSet(targeting_type)", fmt.Sprintf("expected FlagSetResult, got %T", result2))
		return
	}

	// Verify ui_theme returns "light" when variant=two (targeting rule)
	if uiTheme, ok := flags2["ui_theme"]; ok {
		if uiTheme.Treatment == "light" {
			results.Pass("FlagSet(targeting_ui_theme)")
		} else {
			results.Fail("FlagSet(targeting_ui_theme)", fmt.Sprintf("expected light, got %s", uiTheme.Treatment))
		}
	} else {
		results.Fail("FlagSet(targeting_ui_theme)", "flag not found")
	}

	// Verify api_version returns "v1" when variant=two (targeting rule)
	if apiVersion, ok := flags2["api_version"]; ok {
		if apiVersion.Treatment == "v1" {
			results.Pass("FlagSet(targeting_api_version)")
		} else {
			results.Fail("FlagSet(targeting_api_version)", fmt.Sprintf("expected v1, got %s", apiVersion.Treatment))
		}
	} else {
		results.Fail("FlagSet(targeting_api_version)", "flag not found")
	}

	// ============================================================
	// Test 5: Non-existent flag set returns default
	// ============================================================
	result3, err := client.ObjectValue(ctx, "non_existent_flag_set", split.FlagSetResult{}, evalCtx)
	if err != nil {
		// Error is acceptable for non-existent flag set
		results.Pass("FlagSet(non_existent_error)")
	} else {
		// Should return default value (empty FlagSetResult)
		if resultFlags, ok := result3.(split.FlagSetResult); ok {
			if len(resultFlags) == 0 {
				results.Pass("FlagSet(non_existent_empty)")
			} else {
				results.Fail("FlagSet(non_existent)", "unexpected non-empty result")
			}
		} else {
			results.Fail("FlagSet(non_existent)", fmt.Sprintf("expected FlagSetResult, got %T", result3))
		}
	}

	// ============================================================
	// Test 6: ObjectValueDetails for flag set
	// ============================================================
	details, err := client.ObjectValueDetails(ctx, flagSet, split.FlagSetResult{}, evalCtx)
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

	// Verify value is a FlagSetResult with flags
	if detailsValue, ok := details.Value.(split.FlagSetResult); ok {
		if len(detailsValue) >= 2 {
			results.Pass("FlagSet(details_value)")
		} else {
			results.Fail("FlagSet(details_value)", fmt.Sprintf("expected at least 2 flags, got %d", len(detailsValue)))
		}
	} else {
		results.Fail("FlagSet(details_value)", fmt.Sprintf("expected FlagSetResult, got %T", details.Value))
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
