//nolint:dupl,gocognit // Test patterns: type-specific tests have similar structure, comprehensive tests have higher complexity
package split

import (
	"context"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/conf"
	"github.com/splitio/go-toolkit/v5/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluationReturnsDefaultValueWhenFlagNotFound(t *testing.T) {
	ofClient := create(t)
	flagName := flagNonExistent
	evalCtx := evaluationContext()

	// Test with default value false
	result, err := ofClient.BooleanValue(context.TODO(), flagName, false, evalCtx)
	assert.Error(t, err, "Should return error for non-existent flag")
	assert.Contains(t, err.Error(), string(openfeature.FlagNotFoundCode), "Error should be FlagNotFoundCode")
	assert.False(t, result, "Should return default value (false)")

	// Test with default value true
	result, err = ofClient.BooleanValue(context.TODO(), flagName, true, evalCtx)
	assert.Error(t, err, "Should return error for non-existent flag")
	assert.Contains(t, err.Error(), string(openfeature.FlagNotFoundCode), "Error should be FlagNotFoundCode")
	assert.True(t, result, "Should return default value (true)")
}

func TestMissingTargetingKey(t *testing.T) {
	ofClient := create(t)
	flagName := flagNonExistent

	result, err := ofClient.BooleanValue(context.TODO(), flagName, false, openfeature.NewEvaluationContext("", nil))
	assert.Error(t, err, "Should return error when targeting key is missing")
	assert.Contains(t, err.Error(), string(openfeature.TargetingKeyMissingCode), "Error should be TargetingKeyMissingCode")
	assert.False(t, result, "Should return default value (false)")
}

func TestBooleanEvaluationReturnsControlVariantForNonExistentFlag(t *testing.T) {
	ofClient := create(t)
	flagName := "random-non-existent-feature"
	evalCtx := evaluationContext()

	result, err := ofClient.BooleanValueDetails(context.TODO(), flagName, false, evalCtx)
	assert.Error(t, err, "Should return error for non-existent flag")
	assert.Contains(t, err.Error(), string(openfeature.FlagNotFoundCode), "Error should be FlagNotFoundCode")
	assert.False(t, result.Value, "Should return default value (false)")
	assert.Equal(t, "control", result.Variant, "Variant should be 'control' for non-existent flag")
}

func TestBooleanEvaluationReturnsCorrectValue(t *testing.T) {
	ofClient := create(t)
	flagName := flagSomeOther
	evalCtx := evaluationContext()

	result, err := ofClient.BooleanValue(context.TODO(), flagName, true, evalCtx)
	assert.NoError(t, err, "Should not return error for valid flag")
	assert.False(t, result, "Should return false for 'some_other_feature'")
}

func TestBooleanEvaluationWithTargetingKey(t *testing.T) {
	ofClient := create(t)
	flagName := flagMyFeature
	evalCtx := evaluationContext()

	// Test with targeting key "key" - should return true
	result, err := ofClient.BooleanValue(context.TODO(), flagName, false, evalCtx)
	assert.NoError(t, err, "Should not return error for valid flag")
	assert.True(t, result, "Should return true for 'my_feature' with key='key'")

	// Test with different targeting key - should return false
	evalCtx = openfeature.NewEvaluationContext("randomKey", nil)
	result, err = ofClient.BooleanValue(context.TODO(), flagName, true, evalCtx)
	assert.NoError(t, err, "Should not return error for valid flag")
	assert.False(t, result, "Should return false for 'my_feature' with key='randomKey'")
}

func TestStringEvaluationReturnsCorrectValue(t *testing.T) {
	ofClient := create(t)
	flagName := flagSomeOther
	evalCtx := evaluationContext()

	result, err := ofClient.StringValue(context.TODO(), flagName, "on", evalCtx)
	assert.NoError(t, err, "Should not return error for valid flag")
	assert.Equal(t, treatmentOff, result, "Should return 'off' treatment")
}

func TestIntEvaluationReturnsCorrectValue(t *testing.T) {
	ofClient := create(t)
	flagName := flagInt
	evalCtx := evaluationContext()

	result, err := ofClient.IntValue(context.TODO(), flagName, 0, evalCtx)
	assert.NoError(t, err, "Should not return error for valid flag")
	assert.Equal(t, int64(32), result, "Should return 32")
}

func TestObjectEvaluationReturnsCorrectValue(t *testing.T) {
	ofClient := create(t)
	flagName := flagObj
	evalCtx := evaluationContext()

	result, err := ofClient.ObjectValue(context.TODO(), flagName, FlagSetResult{}, evalCtx)
	assert.NoError(t, err, "Should not return error for valid flag")

	flags, ok := result.(FlagSetResult)
	require.True(t, ok, "Result should be FlagSetResult")
	require.Contains(t, flags, "obj_feature", "Should contain obj_feature flag")

	flagResult := flags["obj_feature"]
	assert.Equal(t, "on", flagResult.Treatment, "Should return correct treatment")
	assert.Equal(t, map[string]any{"key": "value"}, flagResult.Config, "Should return correct config")
}

func TestFloatEvaluationReturnsCorrectValue(t *testing.T) {
	ofClient := create(t)
	flagName := flagInt
	evalCtx := evaluationContext()

	result, err := ofClient.FloatValue(context.TODO(), flagName, 0, evalCtx)
	assert.NoError(t, err, "Should not return error for valid flag")
	assert.Equal(t, float64(32), result, "Should return 32.0")
}

// =============================================================================
// Evaluation Details Tests
// =============================================================================

func TestBooleanDetails(t *testing.T) {
	ofClient := create(t)
	flagName := flagSomeOther
	evalCtx := evaluationContext()

	result, err := ofClient.BooleanValueDetails(context.TODO(), flagName, true, evalCtx)
	require.NoError(t, err, "Should not return error") // Use require to prevent panic when accessing result fields
	assert.Equal(t, flagName, result.FlagKey, "Flag key should match")
	assert.Contains(t, string(result.Reason), string(openfeature.TargetingMatchReason), "Reason should be TargetingMatchReason")
	assert.False(t, result.Value, "Value should be false")
	assert.Equal(t, treatmentOff, result.Variant, "Variant should be 'off'")
	assert.Empty(t, result.ErrorCode, "ErrorCode should be empty")
}

func TestIntegerDetails(t *testing.T) {
	ofClient := create(t)
	flagName := flagInt
	evalCtx := evaluationContext()

	result, err := ofClient.IntValueDetails(context.TODO(), flagName, 0, evalCtx)
	require.NoError(t, err, "Should not return error") // Use require to prevent panic when accessing result fields
	assert.Equal(t, flagName, result.FlagKey, "Flag key should match")
	assert.Contains(t, string(result.Reason), string(openfeature.TargetingMatchReason), "Reason should be TargetingMatchReason")
	assert.Equal(t, int64(32), result.Value, "Value should be 32")
	assert.Equal(t, "32", result.Variant, "Variant should be '32'")
	assert.Empty(t, result.ErrorCode, "ErrorCode should be empty")
}

func TestStringDetails(t *testing.T) {
	ofClient := create(t)
	flagName := flagSomeOther
	evalCtx := evaluationContext()

	result, err := ofClient.StringValueDetails(context.TODO(), flagName, "blah", evalCtx)
	require.NoError(t, err, "Should not return error") // Use require to prevent panic when accessing result fields
	assert.Equal(t, flagName, result.FlagKey, "Flag key should match")
	assert.Contains(t, string(result.Reason), string(openfeature.TargetingMatchReason), "Reason should be TargetingMatchReason")
	assert.Equal(t, treatmentOff, result.Value, "Value should be 'off'")
	assert.Equal(t, treatmentOff, result.Variant, "Variant should be 'off'")
	assert.Empty(t, result.ErrorCode, "ErrorCode should be empty")
}

func TestObjectDetails(t *testing.T) {
	ofClient := create(t)
	flagName := flagObj
	evalCtx := evaluationContext()

	result, err := ofClient.ObjectValueDetails(context.TODO(), flagName, map[string]any{}, evalCtx)
	require.NoError(t, err, "Should not return error") // Use require to prevent panic when accessing result fields
	assert.Equal(t, flagName, result.FlagKey, "Flag key should match")
	assert.Contains(t, string(result.Reason), string(openfeature.TargetingMatchReason), "Reason should be TargetingMatchReason")
	assert.Equal(t, flagName, result.Variant, "Variant should be flag name")
	assert.Empty(t, result.ErrorCode, "ErrorCode should be empty")

	// Verify FlagSetResult structure
	flags, ok := result.Value.(FlagSetResult)
	require.True(t, ok, "Value should be FlagSetResult")
	require.Contains(t, flags, "obj_feature", "Should contain obj_feature flag")
	assert.Equal(t, "on", flags["obj_feature"].Treatment, "Should return correct treatment")
	assert.Equal(t, map[string]any{"key": "value"}, flags["obj_feature"].Config, "Should return correct config")
}

func TestFloatDetails(t *testing.T) {
	ofClient := create(t)
	flagName := flagInt
	evalCtx := evaluationContext()

	result, err := ofClient.FloatValueDetails(context.TODO(), flagName, 0, evalCtx)
	require.NoError(t, err, "Should not return error") // Use require to prevent panic when accessing result fields
	assert.Equal(t, flagName, result.FlagKey, "Flag key should match")
	assert.Contains(t, string(result.Reason), string(openfeature.TargetingMatchReason), "Reason should be TargetingMatchReason")
	assert.Equal(t, float64(32), result.Value, "Value should be 32")
	assert.Equal(t, "32", result.Variant, "Variant should be '32'")
	assert.Empty(t, result.ErrorCode, "ErrorCode should be empty")

	// Test with actual float value
	flagName = "float_feature"
	result, err = ofClient.FloatValueDetails(context.TODO(), flagName, 0, evalCtx)
	require.NoError(t, err, "Should not return error")
	assert.Equal(t, 32.5, result.Value, "Value should be 32.5")
	assert.Equal(t, "32.5", result.Variant, "Variant should be '32.5'")
	assert.Empty(t, result.ErrorCode, "ErrorCode should be empty")
}

// =============================================================================
// Parse Error Tests
// =============================================================================

func TestParseErrorHandling(t *testing.T) {
	ofClient := create(t)
	evalCtx := evaluationContext()

	tests := []struct {
		testBoolFunc   func() (bool, error)
		testBoolDeets  func() (openfeature.BooleanEvaluationDetails, error)
		testIntFunc    func() (int64, error)
		testIntDeets   func() (openfeature.IntEvaluationDetails, error)
		testFloatFunc  func() (float64, error)
		testFloatDeets func() (openfeature.FloatEvaluationDetails, error)
		name           string
		intDefault     int64
		floatDefault   float64
		boolDefault    bool
	}{
		{
			name:         "Boolean",
			testBoolFunc: func() (bool, error) { return ofClient.BooleanValue(context.TODO(), flagUnparseable, false, evalCtx) },
			testBoolDeets: func() (openfeature.BooleanEvaluationDetails, error) {
				return ofClient.BooleanValueDetails(context.TODO(), flagUnparseable, false, evalCtx)
			},
			boolDefault: false,
		},
		{
			name:        "Integer",
			testIntFunc: func() (int64, error) { return ofClient.IntValue(context.TODO(), flagUnparseable, 10, evalCtx) },
			testIntDeets: func() (openfeature.IntEvaluationDetails, error) {
				return ofClient.IntValueDetails(context.TODO(), flagUnparseable, 10, evalCtx)
			},
			intDefault: 10,
		},
		{
			name:          "Float",
			testFloatFunc: func() (float64, error) { return ofClient.FloatValue(context.TODO(), flagUnparseable, 10, evalCtx) },
			testFloatDeets: func() (openfeature.FloatEvaluationDetails, error) {
				return ofClient.FloatValueDetails(context.TODO(), flagUnparseable, 10, evalCtx)
			},
			floatDefault: 10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Value functions (Boolean, Int, Float)
			if tt.testBoolFunc != nil {
				result, err := tt.testBoolFunc()
				require.Error(t, err, "Should return parse error")
				assert.Contains(t, err.Error(), string(openfeature.ParseErrorCode), "Error should be ParseErrorCode")
				assert.Equal(t, tt.boolDefault, result, "Should return default value")

				// Test Details function
				details, err := tt.testBoolDeets()
				require.Error(t, err, "Should return parse error")
				assert.Contains(t, err.Error(), string(openfeature.ParseErrorCode), "Error should be ParseErrorCode")
				assert.Equal(t, tt.boolDefault, details.Value, "Value should be default")
				assert.Equal(t, openfeature.ParseErrorCode, details.ErrorCode, "ErrorCode should be ParseErrorCode")
				assert.Equal(t, openfeature.ErrorReason, details.Reason, "Reason should be ErrorReason")
				assert.Equal(t, treatmentUnparseable, details.Variant, "Variant should be the treatment string")
			}

			if tt.testIntFunc != nil {
				result, err := tt.testIntFunc()
				require.Error(t, err, "Should return parse error")
				assert.Contains(t, err.Error(), string(openfeature.ParseErrorCode), "Error should be ParseErrorCode")
				assert.Equal(t, tt.intDefault, result, "Should return default value")

				// Test Details function
				details, err := tt.testIntDeets()
				require.Error(t, err, "Should return parse error")
				assert.Contains(t, err.Error(), string(openfeature.ParseErrorCode), "Error should be ParseErrorCode")
				assert.Equal(t, tt.intDefault, details.Value, "Value should be default")
				assert.Equal(t, openfeature.ParseErrorCode, details.ErrorCode, "ErrorCode should be ParseErrorCode")
				assert.Equal(t, openfeature.ErrorReason, details.Reason, "Reason should be ErrorReason")
				assert.Equal(t, treatmentUnparseable, details.Variant, "Variant should be the treatment string")
			}

			if tt.testFloatFunc != nil {
				result, err := tt.testFloatFunc()
				require.Error(t, err, "Should return parse error")
				assert.Contains(t, err.Error(), string(openfeature.ParseErrorCode), "Error should be ParseErrorCode")
				assert.Equal(t, tt.floatDefault, result, "Should return default value")

				// Test Details function
				details, err := tt.testFloatDeets()
				require.Error(t, err, "Should return parse error")
				assert.Contains(t, err.Error(), string(openfeature.ParseErrorCode), "Error should be ParseErrorCode")
				assert.Equal(t, tt.floatDefault, details.Value, "Value should be default")
				assert.Equal(t, openfeature.ParseErrorCode, details.ErrorCode, "ErrorCode should be ParseErrorCode")
				assert.Equal(t, openfeature.ErrorReason, details.Reason, "Reason should be ErrorReason")
				assert.Equal(t, treatmentUnparseable, details.Variant, "Variant should be the treatment string")
			}
		})
	}
}

// =============================================================================
// Attributes and Configuration Tests
// =============================================================================

// TestAttributesPassedToSplit verifies that attributes from the evaluation context
// are passed to the Split SDK for targeting rules (Bug #2 fix).
func TestAttributesPassedToSplit(t *testing.T) {
	ofClient := create(t)

	evalCtx := openfeature.NewEvaluationContext("key", map[string]any{
		"email":        "user@example.com",
		"age":          int64(30),
		"beta_user":    true,
		"account_type": "premium",
		"roles":        []string{"admin", "user"},
	})

	// Test boolean evaluation with attributes
	flagName := flagMyFeature
	result, err := ofClient.BooleanValue(context.TODO(), flagName, false, evalCtx)
	require.NoError(t, err, "Attributes should not cause error in BooleanValue")
	assert.True(t, result, "Should return true for my_feature")

	// Test string evaluation with attributes
	flagName2 := "some_other_feature"
	strResult, err := ofClient.StringValue(context.TODO(), flagName2, "default", evalCtx)
	require.NoError(t, err, "Attributes should not cause error in StringValue")
	assert.Equal(t, treatmentOff, strResult, "Should return 'off' treatment")

	// Test that attributes don't interfere with existing functionality
	evalCtxNoAttrs := openfeature.NewEvaluationContext("key", nil)
	result2, err := ofClient.BooleanValue(context.TODO(), flagName, false, evalCtxNoAttrs)
	require.NoError(t, err, "Evaluation without attributes should succeed")
	assert.True(t, result2, "Should return true even without attributes")
}

// TestDynamicConfiguration verifies that ObjectEvaluation correctly retrieves
// Dynamic Configuration from the config field.
func TestDynamicConfiguration(t *testing.T) {
	ofClient := create(t)
	flagName := flagMyFeature
	evalCtx := openfeature.NewEvaluationContext("key", nil)

	result, err := ofClient.ObjectValue(context.TODO(), flagName, FlagSetResult{}, evalCtx)
	require.NoError(t, err, "Dynamic Configuration evaluation should succeed")

	flags, ok := result.(FlagSetResult)
	require.True(t, ok, "Result should be FlagSetResult")
	require.Contains(t, flags, "my_feature", "Should contain my_feature flag")

	flagResult := flags["my_feature"]
	assert.Equal(t, "on", flagResult.Treatment, "Should return correct treatment")
	assert.Equal(t, map[string]any{"desc": "this applies only to ON treatment"}, flagResult.Config, "Should return parsed config")
}

// TestMalformedJSONInDynamicConfiguration verifies that malformed JSON in Dynamic Configuration
// is handled gracefully - config is set to nil and a warning is logged.
func TestMalformedJSONInDynamicConfiguration(t *testing.T) {
	ofClient := create(t)
	evalCtx := openfeature.NewEvaluationContext("key", nil)

	t.Run("StringEvaluation", func(t *testing.T) {
		details, err := ofClient.StringValueDetails(context.TODO(), flagMalformedJSON, "default", evalCtx)
		require.NoError(t, err, "Should not return error for valid flag")
		assert.Equal(t, treatmentOn, details.Value, "Should return treatment")
		assert.Empty(t, details.FlagMetadata, "FlagMetadata should be empty for malformed JSON")
	})

	t.Run("BooleanEvaluation", func(t *testing.T) {
		details, err := ofClient.BooleanValueDetails(context.TODO(), flagMalformedJSON, false, evalCtx)
		require.NoError(t, err, "Should not return error for valid flag")
		assert.True(t, details.Value, "Should return true for 'on' treatment")
		assert.Empty(t, details.FlagMetadata, "FlagMetadata should be empty for malformed JSON")
	})

	t.Run("ObjectEvaluation", func(t *testing.T) {
		result, err := ofClient.ObjectValue(context.TODO(), flagMalformedJSON, FlagSetResult{}, evalCtx)
		require.NoError(t, err, "Should not return error for valid flag")

		flags, ok := result.(FlagSetResult)
		require.True(t, ok, "Result should be FlagSetResult")
		flagResult, ok := flags[flagMalformedJSON]
		require.True(t, ok, "Result should contain flag entry")
		assert.Equal(t, treatmentOn, flagResult.Treatment, "Should return treatment")
		assert.Nil(t, flagResult.Config, "Config should be nil for malformed JSON")
	})
}

// =============================================================================
// Missing Key and Not Found Tests
// =============================================================================

// TestEvaluationMissingTargetingKey tests all evaluation types with missing targeting key.
func TestEvaluationMissingTargetingKey(t *testing.T) {
	ofClient := create(t)

	tests := []struct {
		testStrFunc   func() (string, error)
		testFloatFunc func() (float64, error)
		testIntFunc   func() (int64, error)
		testObjFunc   func() (any, error)
		objDefault    any
		name          string
		strDefault    string
		floatDefault  float64
		intDefault    int64
	}{
		{
			name: "String",
			testStrFunc: func() (string, error) {
				return ofClient.StringValue(context.TODO(), "str_feature", "default", openfeature.NewEvaluationContext("", nil))
			},
			strDefault: "default",
		},
		{
			name: "Float",
			testFloatFunc: func() (float64, error) {
				return ofClient.FloatValue(context.TODO(), "float_feature", 3.14, openfeature.NewEvaluationContext("", nil))
			},
			floatDefault: 3.14,
		},
		{
			name: "Integer",
			testIntFunc: func() (int64, error) {
				return ofClient.IntValue(context.TODO(), flagInt, 42, openfeature.NewEvaluationContext("", nil))
			},
			intDefault: 42,
		},
		{
			name: "Object",
			testObjFunc: func() (any, error) {
				return ofClient.ObjectValue(context.TODO(), flagObj, FlagSetResult{}, openfeature.NewEvaluationContext("", nil))
			},
			objDefault: FlagSetResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.testStrFunc != nil {
				result, err := tt.testStrFunc()
				assert.Error(t, err, "Should return error when targeting key is missing")
				assert.Contains(t, err.Error(), string(openfeature.TargetingKeyMissingCode), "Error should be TargetingKeyMissingCode")
				assert.Equal(t, tt.strDefault, result, "Should return default value")
			}

			if tt.testFloatFunc != nil {
				result, err := tt.testFloatFunc()
				assert.Error(t, err, "Should return error when targeting key is missing")
				assert.Contains(t, err.Error(), string(openfeature.TargetingKeyMissingCode), "Error should be TargetingKeyMissingCode")
				assert.Equal(t, tt.floatDefault, result, "Should return default value")
			}

			if tt.testIntFunc != nil {
				result, err := tt.testIntFunc()
				assert.Error(t, err, "Should return error when targeting key is missing")
				assert.Contains(t, err.Error(), string(openfeature.TargetingKeyMissingCode), "Error should be TargetingKeyMissingCode")
				assert.Equal(t, tt.intDefault, result, "Should return default value")
			}

			if tt.testObjFunc != nil {
				result, err := tt.testObjFunc()
				assert.Error(t, err, "Should return error when targeting key is missing")
				assert.Contains(t, err.Error(), string(openfeature.TargetingKeyMissingCode), "Error should be TargetingKeyMissingCode")
				assert.Equal(t, tt.objDefault, result, "Should return default value")
			}
		})
	}
}

// TestEvaluationNotFound tests all evaluation types with non-existent flags.
func TestEvaluationNotFound(t *testing.T) {
	ofClient := create(t)
	evalCtx := evaluationContext()

	tests := []struct {
		testStrFunc   func() (string, error)
		testFloatFunc func() (float64, error)
		testIntFunc   func() (int64, error)
		testObjFunc   func() (any, error)
		objDefault    any
		name          string
		strDefault    string
		floatDefault  float64
		intDefault    int64
	}{
		{
			name: "String",
			testStrFunc: func() (string, error) {
				return ofClient.StringValue(context.TODO(), "nonexistent-string-feature", "default", evalCtx)
			},
			strDefault: "default",
		},
		{
			name: "Float",
			testFloatFunc: func() (float64, error) {
				return ofClient.FloatValue(context.TODO(), "nonexistent-float-feature", 3.14, evalCtx)
			},
			floatDefault: 3.14,
		},
		{
			name: "Integer",
			testIntFunc: func() (int64, error) {
				return ofClient.IntValue(context.TODO(), "nonexistent-int-feature", 42, evalCtx)
			},
			intDefault: 42,
		},
		{
			name: "Object",
			testObjFunc: func() (any, error) {
				return ofClient.ObjectValue(context.TODO(), "nonexistent-obj-feature", FlagSetResult{}, evalCtx)
			},
			objDefault: FlagSetResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.testStrFunc != nil {
				result, err := tt.testStrFunc()
				assert.Error(t, err, "Should return error for non-existent flag")
				assert.Contains(t, err.Error(), string(openfeature.FlagNotFoundCode), "Error should be FlagNotFoundCode")
				assert.Equal(t, tt.strDefault, result, "Should return default value")
			}

			if tt.testFloatFunc != nil {
				result, err := tt.testFloatFunc()
				assert.Error(t, err, "Should return error for non-existent flag")
				assert.Contains(t, err.Error(), string(openfeature.FlagNotFoundCode), "Error should be FlagNotFoundCode")
				assert.Equal(t, tt.floatDefault, result, "Should return default value")
			}

			if tt.testIntFunc != nil {
				result, err := tt.testIntFunc()
				assert.Error(t, err, "Should return error for non-existent flag")
				assert.Contains(t, err.Error(), string(openfeature.FlagNotFoundCode), "Error should be FlagNotFoundCode")
				assert.Equal(t, tt.intDefault, result, "Should return default value")
			}

			if tt.testObjFunc != nil {
				result, err := tt.testObjFunc()
				assert.Error(t, err, "Should return error for non-existent flag")
				assert.Contains(t, err.Error(), string(openfeature.FlagNotFoundCode), "Error should be FlagNotFoundCode")
				assert.Equal(t, tt.objDefault, result, "Should return default value")
			}
		})
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestIntegerEdgeCases tests integer evaluation with boundary values and edge cases.
func TestIntegerEdgeCases(t *testing.T) {
	ofClient := create(t)
	evalCtx := evaluationContext()

	tests := []struct {
		name         string
		description  string
		defaultValue int64
	}{
		{name: "Zero", defaultValue: 0, description: "Test with zero value"},
		{name: "Negative", defaultValue: -42, description: "Test with negative integer"},
		{name: "MaxInt64", defaultValue: 9223372036854775807, description: "Test with max int64 value"},
		{name: "MinInt64", defaultValue: -9223372036854775808, description: "Test with min int64 value"},
		{name: "SmallNegative", defaultValue: -1, description: "Test with -1 value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ofClient.IntValue(context.TODO(), "nonexistent-int-edge-case", tt.defaultValue, evalCtx)
			assert.Error(t, err, "Should return error for non-existent flag")
			assert.Contains(t, err.Error(), string(openfeature.FlagNotFoundCode), "Error should be FlagNotFoundCode")
			assert.Equal(t, tt.defaultValue, result, "Should return default value: %s", tt.description)
		})
	}
}

// TestFloatEdgeCases tests float evaluation with boundary values and edge cases.
func TestFloatEdgeCases(t *testing.T) {
	ofClient := create(t)
	evalCtx := evaluationContext()

	tests := []struct {
		name         string
		description  string
		defaultValue float64
	}{
		{name: "Zero", defaultValue: 0.0, description: "Test with zero value"},
		{name: "Negative", defaultValue: -3.14, description: "Test with negative float"},
		{name: "VerySmall", defaultValue: 1e-10, description: "Test with very small number (scientific notation)"},
		{name: "VeryLarge", defaultValue: 1e10, description: "Test with very large number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ofClient.FloatValue(context.TODO(), "nonexistent-float-edge-case", tt.defaultValue, evalCtx)
			assert.Error(t, err, "Should return error for non-existent flag")
			assert.Contains(t, err.Error(), string(openfeature.FlagNotFoundCode), "Error should be FlagNotFoundCode")
			assert.Equal(t, tt.defaultValue, result, "Should return default value: %s", tt.description)
		})
	}
}

// TestStringEdgeCases tests string evaluation with edge case values.
func TestStringEdgeCases(t *testing.T) {
	ofClient := create(t)
	evalCtx := evaluationContext()

	tests := []struct {
		name         string
		flagName     string
		defaultValue string
		description  string
	}{
		{name: "EmptyString", flagName: "nonexistent-flag", defaultValue: "", description: "Test with empty string as default value"},
		{name: "VeryLongString", flagName: "nonexistent-flag", defaultValue: string(make([]byte, 1000)), description: "Test with very long default value (1000+ chars)"},
		{name: "UnicodeChars", flagName: "nonexistent-flag", defaultValue: "hello-世界-🌍", description: "Test with unicode characters"},
		{name: "SpecialChars", flagName: "nonexistent-flag", defaultValue: "!@#$%^&*()_+-=[]{}|;:',.<>?/~`", description: "Test with special characters"},
		{name: "Whitespace", flagName: "nonexistent-flag", defaultValue: "  \t\n\r  ", description: "Test with whitespace characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ofClient.StringValue(context.TODO(), tt.flagName, tt.defaultValue, evalCtx)
			assert.Error(t, err, "Should return error for non-existent flag")
			assert.Contains(t, err.Error(), string(openfeature.FlagNotFoundCode), "Error should be FlagNotFoundCode")
			assert.Equal(t, tt.defaultValue, result, "Should return default value: %s", tt.description)
		})
	}
}

// TestObjectEdgeCases tests object evaluation with edge case structures.
func TestObjectEdgeCases(t *testing.T) {
	ofClient := create(t)
	evalCtx := evaluationContext()

	tests := []struct {
		name         string
		defaultValue map[string]any
		description  string
	}{
		{name: "EmptyObject", defaultValue: map[string]any{}, description: "Test with empty object"},
		{
			name: "NestedObject",
			defaultValue: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{"level3": "deep"},
				},
			},
			description: "Test with deeply nested object",
		},
		{
			name: "ObjectWithArray",
			defaultValue: map[string]any{
				"items":  []any{"a", "b", "c"},
				"counts": []int{1, 2, 3},
			},
			description: "Test with arrays in object",
		},
		{
			name:         "ObjectWithNull",
			defaultValue: map[string]any{"key": "value", "nullField": nil},
			description:  "Test with null values in object",
		},
		{
			name: "MixedTypes",
			defaultValue: map[string]any{
				"string": "text", "number": 42, "float": 3.14, "bool": true,
				"array": []any{1, "two", 3.0}, "nested": map[string]any{"inner": "value"},
			},
			description: "Test with mixed types in object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ofClient.ObjectValue(context.TODO(), "nonexistent-obj-edge-case", tt.defaultValue, evalCtx)
			assert.Error(t, err, "Should return error for non-existent flag")
			assert.Contains(t, err.Error(), string(openfeature.FlagNotFoundCode), "Error should be FlagNotFoundCode")
			assert.Equal(t, tt.defaultValue, result, "Should return default value: %s", tt.description)
		})
	}
}

// TestTargetingKeyEdgeCases tests various edge cases for targeting keys.
func TestTargetingKeyEdgeCases(t *testing.T) {
	ofClient := create(t)

	tests := []struct {
		name         string
		targetingKey string
		flagName     string
		description  string
	}{
		{name: "EmptyTargetingKey", targetingKey: "", flagName: flagSomeOther, description: "Test with empty targeting key"},
		{name: "VeryLongTargetingKey", targetingKey: string(make([]byte, 1000)), flagName: flagSomeOther, description: "Test with very long targeting key (1000+ chars)"},
		{name: "UnicodeTargetingKey", targetingKey: "user-世界-🌍", flagName: flagSomeOther, description: "Test with unicode in targeting key"},
		{name: "SpecialCharsTargetingKey", targetingKey: "user@example.com", flagName: flagSomeOther, description: "Test with email-like targeting key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evalCtx := openfeature.NewEvaluationContext(tt.targetingKey, nil)

			result, err := ofClient.BooleanValue(context.TODO(), tt.flagName, true, evalCtx)

			if tt.targetingKey == "" {
				assert.Error(t, err, "Should return error for empty targeting key")
				assert.Contains(t, err.Error(), string(openfeature.TargetingKeyMissingCode), "Error should be TargetingKeyMissingCode")
			} else {
				_ = result
				_ = err
			}
		})
	}
}

// =============================================================================
// Context Cancellation Tests
// =============================================================================

// TestContextCancellation verifies that canceled contexts are respected in all evaluation methods.
func TestContextCancellation(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "test-user",
	}

	// Test Boolean evaluation with canceled context
	boolResult := provider.BooleanEvaluation(ctx, flagSomeOther, true, flatCtx)
	assert.Equal(t, true, boolResult.Value, "Should return default value when context is canceled")
	assert.Equal(t, openfeature.ErrorReason, boolResult.Reason, "Should have error reason")
	assert.NotNil(t, boolResult.ResolutionError, "Should have resolution error")

	// Test String evaluation with canceled context
	strResult := provider.StringEvaluation(ctx, flagSomeOther, "default", flatCtx)
	assert.Equal(t, "default", strResult.Value, "Should return default value when context is canceled")
	assert.Equal(t, openfeature.ErrorReason, strResult.Reason, "Should have error reason")
	assert.NotNil(t, strResult.ResolutionError, "Should have resolution error")

	// Test Int evaluation with canceled context
	intResult := provider.IntEvaluation(ctx, flagInt, 999, flatCtx)
	assert.Equal(t, int64(999), intResult.Value, "Should return default value when context is canceled")
	assert.Equal(t, openfeature.ErrorReason, intResult.Reason, "Should have error reason")
	assert.NotNil(t, intResult.ResolutionError, "Should have resolution error")

	// Test Float evaluation with canceled context
	floatResult := provider.FloatEvaluation(ctx, "some_flag", 123.45, flatCtx)
	assert.Equal(t, 123.45, floatResult.Value, "Should return default value when context is canceled")
	assert.Equal(t, openfeature.ErrorReason, floatResult.Reason, "Should have error reason")
	assert.NotNil(t, floatResult.ResolutionError, "Should have resolution error")

	// Test Object evaluation with canceled context
	defaultObj := FlagSetResult{}
	objResult := provider.ObjectEvaluation(ctx, "some_flag", defaultObj, flatCtx)
	assert.Equal(t, defaultObj, objResult.Value, "Should return default value when context is canceled")
	assert.Equal(t, openfeature.ErrorReason, objResult.Reason, "Should have error reason")
	assert.NotNil(t, objResult.ResolutionError, "Should have resolution error")
}

// =============================================================================
// PROVIDER_NOT_READY Evaluation Tests
// =============================================================================

// TestEvaluationWhenProviderNotReady tests all evaluation types when provider is not initialized.
func TestEvaluationWhenProviderNotReady(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	// Initialize then shut down to reliably put provider in NotReady state.
	// We can't rely on "not calling Init" because in localhost mode, the Split SDK
	// factory auto-initializes from the YAML file during New() (factory.IsReady() == true).
	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)
	err = provider.ShutdownWithContext(context.Background())
	require.NoError(t, err)
	require.Equal(t, openfeature.NotReadyState, provider.Status(), "Provider should be NotReady after shutdown")

	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "test-user",
	}

	t.Run("Boolean", func(t *testing.T) {
		result := provider.BooleanEvaluation(context.TODO(), flagSomeOther, true, flatCtx)
		assert.Equal(t, true, result.Value, "Should return default value")
		assert.Equal(t, openfeature.ErrorReason, result.Reason)
		assert.Contains(t, result.ResolutionError.Error(), string(openfeature.ProviderNotReadyCode))
	})

	t.Run("String", func(t *testing.T) {
		result := provider.StringEvaluation(context.TODO(), flagSomeOther, "default", flatCtx)
		assert.Equal(t, "default", result.Value, "Should return default value")
		assert.Equal(t, openfeature.ErrorReason, result.Reason)
		assert.Contains(t, result.ResolutionError.Error(), string(openfeature.ProviderNotReadyCode))
	})

	t.Run("Int", func(t *testing.T) {
		result := provider.IntEvaluation(context.TODO(), flagInt, 42, flatCtx)
		assert.Equal(t, int64(42), result.Value, "Should return default value")
		assert.Equal(t, openfeature.ErrorReason, result.Reason)
		assert.Contains(t, result.ResolutionError.Error(), string(openfeature.ProviderNotReadyCode))
	})

	t.Run("Float", func(t *testing.T) {
		result := provider.FloatEvaluation(context.TODO(), flagInt, 3.14, flatCtx)
		assert.Equal(t, 3.14, result.Value, "Should return default value")
		assert.Equal(t, openfeature.ErrorReason, result.Reason)
		assert.Contains(t, result.ResolutionError.Error(), string(openfeature.ProviderNotReadyCode))
	})

	t.Run("Object", func(t *testing.T) {
		defaultObj := FlagSetResult{}
		result := provider.ObjectEvaluation(context.TODO(), flagObj, defaultObj, flatCtx)
		assert.Equal(t, defaultObj, result.Value, "Should return default value")
		assert.Equal(t, openfeature.ErrorReason, result.Reason)
		assert.Contains(t, result.ResolutionError.Error(), string(openfeature.ProviderNotReadyCode))
	})
}

// =============================================================================
// INVALID_CONTEXT Evaluation Tests
// =============================================================================

// TestEvaluationWithInvalidContext tests evaluation with a non-string targeting key.
func TestEvaluationWithInvalidContext(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	// Targeting key exists but is NOT a string (integer instead)
	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: 12345,
	}

	t.Run("Boolean", func(t *testing.T) {
		result := provider.BooleanEvaluation(context.TODO(), flagSomeOther, true, flatCtx)
		assert.Equal(t, true, result.Value, "Should return default value")
		assert.Equal(t, openfeature.ErrorReason, result.Reason)
		assert.Contains(t, result.ResolutionError.Error(), string(openfeature.InvalidContextCode))
	})

	t.Run("String", func(t *testing.T) {
		result := provider.StringEvaluation(context.TODO(), flagSomeOther, "default", flatCtx)
		assert.Equal(t, "default", result.Value, "Should return default value")
		assert.Equal(t, openfeature.ErrorReason, result.Reason)
		assert.Contains(t, result.ResolutionError.Error(), string(openfeature.InvalidContextCode))
	})

	t.Run("Int", func(t *testing.T) {
		result := provider.IntEvaluation(context.TODO(), flagInt, 42, flatCtx)
		assert.Equal(t, int64(42), result.Value, "Should return default value")
		assert.Equal(t, openfeature.ErrorReason, result.Reason)
		assert.Contains(t, result.ResolutionError.Error(), string(openfeature.InvalidContextCode))
	})

	t.Run("Float", func(t *testing.T) {
		result := provider.FloatEvaluation(context.TODO(), flagInt, 3.14, flatCtx)
		assert.Equal(t, 3.14, result.Value, "Should return default value")
		assert.Equal(t, openfeature.ErrorReason, result.Reason)
		assert.Contains(t, result.ResolutionError.Error(), string(openfeature.InvalidContextCode))
	})

	t.Run("Object", func(t *testing.T) {
		defaultObj := FlagSetResult{}
		result := provider.ObjectEvaluation(context.TODO(), flagObj, defaultObj, flatCtx)
		assert.Equal(t, defaultObj, result.Value, "Should return default value")
		assert.Equal(t, openfeature.ErrorReason, result.Reason)
		assert.Contains(t, result.ResolutionError.Error(), string(openfeature.InvalidContextCode))
	})
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestIntegrationWithOpenFeatureSDK tests integration with the OpenFeature SDK.
func TestIntegrationWithOpenFeatureSDK(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "test-user",
	}

	// Test boolean evaluation
	boolResult := provider.BooleanEvaluation(context.TODO(), flagSomeOther, false, flatCtx)
	assert.Equal(t, openfeature.TargetingMatchReason, boolResult.Reason, "Boolean evaluation should succeed")
	assert.False(t, boolResult.Value, "Boolean value should be false for flagSomeOther")

	// Test string evaluation
	strResult := provider.StringEvaluation(context.TODO(), flagSomeOther, "default", flatCtx)
	assert.Equal(t, openfeature.TargetingMatchReason, strResult.Reason, "String evaluation should succeed")
	assert.Equal(t, treatmentOff, strResult.Value, "String value should be 'off'")

	// Test integer evaluation
	intResult := provider.IntEvaluation(context.TODO(), flagInt, 0, flatCtx)
	assert.Equal(t, openfeature.TargetingMatchReason, intResult.Reason, "Int evaluation should succeed")
	assert.Equal(t, int64(32), intResult.Value, "Int value should be 32")
}
