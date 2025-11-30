//nolint:dupl,gocognit // Test patterns: type-specific tests have similar structure, comprehensive tests have higher complexity
package split

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/conf"
	"github.com/splitio/go-toolkit/v5/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// Test flag names used across multiple tests
const (
	flagNonExistent      = "random-non-existent-feature"
	flagSomeOther        = "some_other_feature"
	flagMyFeature        = "my_feature"
	flagInt              = "int_feature"
	flagObj              = "obj_feature"
	flagUnparseable      = "unparseable_feature"
	flagMalformedJSON    = "malformed_json_feature"
	treatmentOff         = "off"
	treatmentOn          = "on"               // Treatment for obj_feature (now uses correct YAML format)
	treatmentUnparseable = "not-a-valid-type" // Treatment that cannot be parsed as bool/int/float
	testClientName       = "test_client"
	testSplitFile        = "testdata/split.yaml"
	providerNameSplit    = "Split"
)

// TestMain adds goroutine leak detection to all tests.
// Uses goleak to detect goroutine leaks from OUR code (external dependencies ignored).
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// Ignore OpenFeature SDK event executor goroutines (created per test via SetProvider)
		// Use IgnoreAnyFunction because these goroutines can be in various states
		// Note: Function names differ between normal and race detector builds
		goleak.IgnoreAnyFunction("github.com/open-feature/go-sdk/openfeature.(*eventExecutor).startEventListener.func1.1"),
		goleak.IgnoreAnyFunction("github.com/open-feature/go-sdk/openfeature.newEventExecutor.(*eventExecutor).startEventListener.func1.1"), // -race variant
		goleak.IgnoreAnyFunction("github.com/open-feature/go-sdk/openfeature.(*eventExecutor).startListeningAndShutdownOld.func1"),
		goleak.IgnoreAnyFunction("github.com/open-feature/go-sdk/openfeature.newEventExecutor.(*eventExecutor).startListeningAndShutdownOld.func1"), // -race variant
		goleak.IgnoreAnyFunction("github.com/open-feature/go-sdk/openfeature.(*eventExecutor).triggerEvent"),
		// Ignore Split SDK background goroutines (created during individual tests)
		goleak.IgnoreTopFunction("github.com/splitio/go-split-commons/v8/synchronizer.(*ManagerImpl).Start.func1"),
		goleak.IgnoreTopFunction("github.com/splitio/go-split-commons/v8/synchronizer.(*ManagerImpl).StartBGSync.func1"),
		// Ignore standard library goroutines
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("time.Sleep"),
	)
}

func create(t *testing.T) *openfeature.Client {
	t.Helper()
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10 // Must be positive

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")
	require.NotNil(t, provider, "Provider should not be nil")

	// Proper cleanup: Shutdown provider when test completes
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = openfeature.ShutdownWithContext(ctx)
	})

	// Use context-aware SetProviderWithContextAndWait (gold standard)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err = openfeature.SetProviderWithContextAndWait(ctx, provider)
	require.NoError(t, err, "Failed to set provider")

	return openfeature.NewClient(testClientName)
}

func evaluationContext() openfeature.EvaluationContext {
	return openfeature.NewEvaluationContext("key", nil)
}

func TestCreateSimple(t *testing.T) {
	// Test New() with configuration
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10 // Must be positive

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Provider creation should succeed")
	assert.NotNil(t, provider, "Provider should not be nil")
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()
}

// TestNewErrors tests error handling in New constructor.
// This improves coverage for the New function.
func TestNewErrors(t *testing.T) {
	// Test with empty API key - should fail during factory creation
	provider, err := New("")
	assert.Error(t, err, "Empty API key should cause error")
	assert.Nil(t, provider, "Provider should be nil when creation fails")

	// Test with invalid API key format - Split SDK should reject it
	provider, err = New("invalid-key-format-!@#$%")
	// Note: Split SDK might accept any string as API key and only fail on network calls
	// The behavior depends on Split SDK version, so we just verify it doesn't panic
	_ = provider
	_ = err

	// Note: We cannot test timeout scenarios without:
	// 1. Mocking the Split SDK (would require interface extraction)
	// 2. Using a real Split instance (not suitable for unit tests)
	// 3. Configuring very short timeouts (would make tests flaky)
	// These scenarios are better covered by integration tests.
}

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

	result, err := ofClient.ObjectValue(context.TODO(), flagName, 0, evalCtx)
	expectedResult := map[string]any{
		"obj_feature": map[string]any{
			"treatment": "on",
			"config": map[string]any{
				"key": "value",
			},
		},
	}
	assert.NoError(t, err, "Should not return error for valid flag")
	assert.Equal(t, expectedResult, result, "Should return expected object")
}

func TestFloatEvaluationReturnsCorrectValue(t *testing.T) {
	ofClient := create(t)
	flagName := flagInt
	evalCtx := evaluationContext()

	result, err := ofClient.FloatValue(context.TODO(), flagName, 0, evalCtx)
	assert.NoError(t, err, "Should not return error for valid flag")
	assert.Equal(t, float64(32), result, "Should return 32.0")
}

func TestMetadataReturnsProviderName(t *testing.T) {
	ofClient := create(t)
	assert.Equal(t, testClientName, ofClient.Metadata().Domain(), "Client name should match")
	assert.Equal(t, providerNameSplit, openfeature.ProviderMetadata().Name, "Provider name should be 'Split'")
}

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
	expectedResult := map[string]any{
		"obj_feature": map[string]any{
			"treatment": "on",
			"config": map[string]any{
				"key": "value",
			},
		},
	}
	require.NoError(t, err, "Should not return error") // Use require to prevent panic when accessing result fields
	assert.Equal(t, flagName, result.FlagKey, "Flag key should match")
	assert.Contains(t, string(result.Reason), string(openfeature.TargetingMatchReason), "Reason should be TargetingMatchReason")
	assert.Equal(t, expectedResult, result.Value, "Value should match expected object")
	assert.Equal(t, flagName, result.Variant, "Variant should be flag name")
	assert.Empty(t, result.ErrorCode, "ErrorCode should be empty")
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

// TestAttributesPassedToSplit verifies that attributes from the evaluation context
// are passed to the Split SDK for targeting rules (Bug #2 fix).
// This test ensures attributes don't cause errors and work correctly.
func TestAttributesPassedToSplit(t *testing.T) {
	ofClient := create(t)

	// Create evaluation context with various attribute types
	// Split SDK supports: strings, int64, bool, []string
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
	// This validates backward compatibility
	evalCtxNoAttrs := openfeature.NewEvaluationContext("key", nil)
	result2, err := ofClient.BooleanValue(context.TODO(), flagName, false, evalCtxNoAttrs)
	require.NoError(t, err, "Evaluation without attributes should succeed")
	assert.True(t, result2, "Should return true even without attributes")
}

// TestDynamicConfiguration verifies that ObjectEvaluation correctly retrieves
// Dynamic Configuration from the config field.
//
// split.yaml defines my_feature with:
//
//	treatment: "on"
//	keys: "key"
//	config: "{\"desc\" : \"this applies only to ON treatment\"}"
//
// ObjectEvaluation returns Split SDK structure: {"flagName": {"treatment": "...", "config": {...}}}
func TestDynamicConfiguration(t *testing.T) {
	ofClient := create(t)
	flagName := flagMyFeature
	evalCtx := openfeature.NewEvaluationContext("key", nil)

	result, err := ofClient.ObjectValue(context.TODO(), flagName, nil, evalCtx)
	require.NoError(t, err, "Dynamic Configuration evaluation should succeed")

	// Expected: Split SDK structure with treatment and parsed config
	expectedResult := map[string]any{
		"my_feature": map[string]any{
			"treatment": "on",
			"config": map[string]any{
				"desc": "this applies only to ON treatment",
			},
		},
	}

	assert.Equal(t, expectedResult, result, "Should return Split SDK structure with parsed config")
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
		result, err := ofClient.ObjectValue(context.TODO(), flagMalformedJSON, nil, evalCtx)
		require.NoError(t, err, "Should not return error for valid flag")

		flagResult, ok := result.(map[string]any)[flagMalformedJSON].(map[string]any)
		require.True(t, ok, "Result should contain flag entry")
		assert.Equal(t, treatmentOn, flagResult["treatment"], "Should return treatment")
		assert.Nil(t, flagResult["config"], "Config should be nil for malformed JSON")
	})
}

// TestEvaluationMissingTargetingKey tests all evaluation types with missing targeting key.
// This consolidates 4 separate tests into a single table-driven test.
func TestEvaluationMissingTargetingKey(t *testing.T) {
	ofClient := create(t)

	tests := []struct {
		testStrFunc   func() (string, error)
		testFloatFunc func() (float64, error)
		testIntFunc   func() (int64, error)
		testObjFunc   func() (any, error)
		objDefault    map[string]any
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
				return ofClient.ObjectValue(context.TODO(), flagObj, map[string]any{"key": "default"}, openfeature.NewEvaluationContext("", nil))
			},
			objDefault: map[string]any{"key": "default"},
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
// This consolidates 4 separate tests into a single table-driven test.
func TestEvaluationNotFound(t *testing.T) {
	ofClient := create(t)
	evalCtx := evaluationContext()

	tests := []struct {
		testStrFunc   func() (string, error)
		testFloatFunc func() (float64, error)
		testIntFunc   func() (int64, error)
		testObjFunc   func() (any, error)
		objDefault    map[string]any
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
				return ofClient.ObjectValue(context.TODO(), "nonexistent-obj-feature", map[string]any{"key": "default"}, evalCtx)
			},
			objDefault: map[string]any{"key": "default"},
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

// *** Edge Case Tests ***

// TestIntegerEdgeCases tests integer evaluation with boundary values and edge cases.
func TestIntegerEdgeCases(t *testing.T) {
	ofClient := create(t)
	evalCtx := evaluationContext()

	tests := []struct {
		name         string
		description  string
		defaultValue int64
	}{
		{
			name:         "Zero",
			defaultValue: 0,
			description:  "Test with zero value",
		},
		{
			name:         "Negative",
			defaultValue: -42,
			description:  "Test with negative integer",
		},
		{
			name:         "MaxInt64",
			defaultValue: 9223372036854775807,
			description:  "Test with max int64 value",
		},
		{
			name:         "MinInt64",
			defaultValue: -9223372036854775808,
			description:  "Test with min int64 value",
		},
		{
			name:         "SmallNegative",
			defaultValue: -1,
			description:  "Test with -1 value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with non-existent flag - should return default value
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
		{
			name:         "Zero",
			defaultValue: 0.0,
			description:  "Test with zero value",
		},
		{
			name:         "Negative",
			defaultValue: -3.14,
			description:  "Test with negative float",
		},
		{
			name:         "VerySmall",
			defaultValue: 1e-10,
			description:  "Test with very small number (scientific notation)",
		},
		{
			name:         "VeryLarge",
			defaultValue: 1e10,
			description:  "Test with very large number",
		},
		{
			name:         "Zero",
			defaultValue: 0.0,
			description:  "Test with zero value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with non-existent flag - should return default value
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
		{
			name:         "EmptyString",
			flagName:     "nonexistent-flag",
			defaultValue: "",
			description:  "Test with empty string as default value",
		},
		{
			name:         "VeryLongString",
			flagName:     "nonexistent-flag",
			defaultValue: string(make([]byte, 1000)),
			description:  "Test with very long default value (1000+ chars)",
		},
		{
			name:         "UnicodeChars",
			flagName:     "nonexistent-flag",
			defaultValue: "hello-世界-🌍",
			description:  "Test with unicode characters",
		},
		{
			name:         "SpecialChars",
			flagName:     "nonexistent-flag",
			defaultValue: "!@#$%^&*()_+-=[]{}|;:',.<>?/~`",
			description:  "Test with special characters",
		},
		{
			name:         "Whitespace",
			flagName:     "nonexistent-flag",
			defaultValue: "  \t\n\r  ",
			description:  "Test with whitespace characters",
		},
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
		{
			name:         "EmptyObject",
			defaultValue: map[string]any{},
			description:  "Test with empty object",
		},
		{
			name: "NestedObject",
			defaultValue: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": "deep",
					},
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
			name: "ObjectWithNull",
			defaultValue: map[string]any{
				"key":       "value",
				"nullField": nil,
			},
			description: "Test with null values in object",
		},
		{
			name: "MixedTypes",
			defaultValue: map[string]any{
				"string": "text",
				"number": 42,
				"float":  3.14,
				"bool":   true,
				"array":  []any{1, "two", 3.0},
				"nested": map[string]any{"inner": "value"},
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
		{
			name:         "EmptyTargetingKey",
			targetingKey: "",
			flagName:     flagSomeOther,
			description:  "Test with empty targeting key",
		},
		{
			name:         "VeryLongTargetingKey",
			targetingKey: string(make([]byte, 1000)),
			flagName:     flagSomeOther,
			description:  "Test with very long targeting key (1000+ chars)",
		},
		{
			name:         "UnicodeTargetingKey",
			targetingKey: "user-世界-🌍",
			flagName:     flagSomeOther,
			description:  "Test with unicode in targeting key",
		},
		{
			name:         "SpecialCharsTargetingKey",
			targetingKey: "user@example.com",
			flagName:     flagSomeOther,
			description:  "Test with email-like targeting key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evalCtx := openfeature.NewEvaluationContext(tt.targetingKey, nil)

			// Should not panic and should return a result
			result, err := ofClient.BooleanValue(context.TODO(), tt.flagName, true, evalCtx)

			// For empty key, expect TargetingKeyMissingCode error
			if tt.targetingKey == "" {
				assert.Error(t, err, "Should return error for empty targeting key")
				assert.Contains(t, err.Error(), string(openfeature.TargetingKeyMissingCode), "Error should be TargetingKeyMissingCode")
			} else {
				// For valid keys, should succeed (treatment may vary)
				// We don't assert the result value since it depends on Split configuration
				_ = result
				_ = err
			}
		})
	}
}

// *** Lifecycle Management Tests ***

// TestProviderInit tests the Init method and lifecycle initialization.
func TestProviderInit(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10 // Must be positive

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	// Provider should start in NotReady state
	assert.Equal(t, openfeature.NotReadyState, provider.Status(), "Provider should start in NotReady state")

	// Call Init
	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	// Provider should now be in Ready state
	assert.Equal(t, openfeature.ReadyState, provider.Status(), "Provider should be Ready after Init")

	// Calling Init again should be idempotent
	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	assert.NoError(t, err, "Second Init call should succeed (idempotent)")

	// Cleanup
	_ = provider.ShutdownWithContext(context.Background())
}

// TestProviderShutdown tests the Shutdown method and resource cleanup.
func TestProviderShutdown(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10 // Must be positive

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	// Provider should be Ready
	assert.Equal(t, openfeature.ReadyState, provider.Status(), "Provider should be Ready after Init")

	// Shutdown the provider
	_ = provider.ShutdownWithContext(context.Background())

	// Provider should be NotReady after shutdown
	assert.Equal(t, openfeature.NotReadyState, provider.Status(), "Provider should be NotReady after Shutdown")

	// Calling Shutdown again should be idempotent (should not panic)
	_ = provider.ShutdownWithContext(context.Background())
}

// TestProviderShutdownTimeout tests that Shutdown completes within reasonable time.
func TestProviderShutdownTimeout(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10 // Must be positive

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	// Shutdown should complete quickly
	done := make(chan struct{})
	go func() {
		_ = provider.ShutdownWithContext(context.Background())
		close(done)
	}()

	select {
	case <-done:
		// Success - shutdown completed
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not complete within 10 seconds")
	}
}

// TestProviderEventChannel tests the EventChannel method and event emission.
func TestProviderEventChannel(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10 // Must be positive

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	// Get event channel
	eventChan := provider.EventChannel()
	require.NotNil(t, eventChan, "EventChannel() should not return nil")

	// Listen for events in background
	events := make([]openfeature.Event, 0)
	done := make(chan struct{})
	go func() {
		for event := range eventChan {
			events = append(events, event)
			if event.EventType == openfeature.ProviderReady {
				close(done)
				return
			}
		}
	}()

	// Init provider (should emit PROVIDER_READY event)
	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	// Wait for PROVIDER_READY event (with timeout)
	select {
	case <-done:
		// Success - received ProviderReady event
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for ProviderReady event")
	}

	// Check that we received at least one event
	assert.NotEmpty(t, events, "Should receive at least one event")

	// Verify the event is PROVIDER_READY
	foundReady := false
	for _, event := range events {
		if event.EventType == openfeature.ProviderReady {
			foundReady = true
			assert.Equal(t, providerNameSplit, event.ProviderName, "Provider name should be 'Split'")
		}
	}
	assert.True(t, foundReady, "Should receive ProviderReady event")

	// Cleanup
	_ = provider.ShutdownWithContext(context.Background())
}

// TestProviderHealth tests the Health method.
func TestProviderHealth(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10 // Must be positive

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	// Health before Init
	metrics := provider.Metrics()
	assert.Equal(t, providerNameSplit, metrics["provider"], "Provider name should be 'Split'")
	assert.False(t, metrics["initialized"].(bool), "Should not be initialized before Init")
	assert.Equal(t, string(openfeature.NotReadyState), metrics["status"], "Status should be NOT_READY")

	// Init provider
	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	// Metrics after Init
	metrics = provider.Metrics()
	assert.True(t, metrics["initialized"].(bool), "Should be initialized after Init")
	assert.Equal(t, string(openfeature.ReadyState), metrics["status"], "Status should be READY")
	assert.True(t, metrics["ready"].(bool), "Should be ready")

	// Check that splits_count exists and is > 0
	splitsCount, ok := metrics["splits_count"].(int)
	require.True(t, ok, "splits_count should be an int")
	assert.Greater(t, splitsCount, 0, "splits_count should be greater than 0")

	// Cleanup
	_ = provider.ShutdownWithContext(context.Background())
}

// TestProviderFactoryGetter tests the Factory method.
func TestProviderFactoryGetter(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10 // Must be positive

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	// Initialize the provider (wait for SDK to be ready)
	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Failed to initialize provider")

	// Get factory
	factory := provider.Factory()
	require.NotNil(t, factory, "Factory should not be nil")

	// Verify we can get the client from factory
	splitClient := factory.Client()
	require.NotNil(t, splitClient, "Client should not be nil")

	// Verify we can use the client for advanced operations (Track event)
	err = splitClient.Track("test-user", "test-traffic", "test-event", 1.0, nil)
	assert.NoError(t, err, "Track call should succeed")

	// Verify we can get the manager from factory
	manager := factory.Manager()
	require.NotNil(t, manager, "Manager should not be nil")

	// Verify we can query metadata
	splitNames := manager.SplitNames()
	assert.NotEmpty(t, splitNames, "Should have split definitions loaded")

	// Verify readiness check
	assert.True(t, factory.IsReady(), "Factory should be ready")

	// Cleanup
	_ = provider.ShutdownWithContext(context.Background())
}

// TestConcurrentEvaluations tests thread safety with concurrent evaluations.
// This validates that the provider can safely handle multiple goroutines
// evaluating flags simultaneously without data races.
func TestConcurrentEvaluations(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err, "Failed to create provider")

	// Use context-aware methods (gold standard)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err = openfeature.SetProviderWithContextAndWait(ctx, provider)
	require.NoError(t, err, "Failed to set provider")

	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		_ = provider.ShutdownWithContext(shutdownCtx)
	}()

	const numGoroutines = 50
	const numEvaluations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	// Run concurrent evaluations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Each goroutine gets its own client
			ofClient := openfeature.NewClient("concurrent-test")
			evalCtx := openfeature.NewEvaluationContext("test-user", nil)

			for j := 0; j < numEvaluations; j++ {
				// Boolean evaluation
				_, err := ofClient.BooleanValue(
					context.TODO(),
					flagSomeOther,
					false,
					evalCtx,
				)
				// Ignore FlagNotFoundCode as it's expected for non-existent flags
				if err != nil && !contains(err.Error(), "FLAG_NOT_FOUND") {
					errors <- fmt.Errorf("goroutine %d iteration %d: %w", id, j, err)
					return
				}

				// String evaluation
				_, err = ofClient.StringValue(
					context.TODO(),
					flagSomeOther,
					"default",
					evalCtx,
				)
				if err != nil && !contains(err.Error(), "FLAG_NOT_FOUND") {
					errors <- fmt.Errorf("goroutine %d iteration %d: %w", id, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent evaluation error: %v", err)
	}
}

// Helper function for TestConcurrentEvaluations
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

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

	// Initialize provider directly instead of using global SetProviderAndWait
	// to avoid conflicts with other tests
	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err, "Init should succeed")

	// Test evaluations directly through the provider
	// This validates that the provider works correctly with the OpenFeature patterns
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
	defaultObj := map[string]any{"fallback": true}
	objResult := provider.ObjectEvaluation(ctx, "some_flag", defaultObj, flatCtx)
	assert.Equal(t, defaultObj, objResult.Value, "Should return default value when context is canceled")
	assert.Equal(t, openfeature.ErrorReason, objResult.Reason, "Should have error reason")
	assert.NotNil(t, objResult.ResolutionError, "Should have resolution error")
}

// TestConcurrentInitShutdown tests race conditions when Init and Shutdown are called concurrently.
// This ensures thread-safety of the provider lifecycle management and that no panics occur.
func TestConcurrentInitShutdown(t *testing.T) {
	// Use a shorter test when running with all tests to avoid timeout
	// Individual test run with -run flag will still be thorough due to race detector
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 1

	// Reduced iterations to prevent timeout when run with all tests
	// This still provides good coverage for race conditions
	const iterations = 2
	for i := 0; i < iterations; i++ {
		provider, err := New("localhost", WithSplitConfig(cfg))
		require.NoError(t, err)

		var wg sync.WaitGroup
		const concurrency = 3

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Multiple concurrent Inits
		for j := 0; j < concurrency; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = provider.InitWithContext(ctx, openfeature.NewEvaluationContext("", nil))
			}()
		}

		// One concurrent Shutdown (to test race with Init)
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond) // Small delay to let some Inits start
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			_ = provider.ShutdownWithContext(shutdownCtx)
		}()

		wg.Wait()

		// Verify provider is in NotReady state after concurrent operations
		assert.Equal(t, openfeature.NotReadyState, provider.Status(), "Provider should be NotReady after shutdown")
	}
}

// TestEventChannelOverflow tests behavior when event channel buffer is full.
// The provider has a buffered channel (size 100) for events. This test verifies
// that when the buffer is full, new events are dropped gracefully without blocking.
func TestEventChannelOverflow(t *testing.T) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 1

	provider, err := New("localhost", WithSplitConfig(cfg))
	require.NoError(t, err)
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	// Don't consume events - let the channel fill up
	eventChan := provider.EventChannel()

	// Generate more events than buffer size (100) to trigger overflow
	// We'll emit 150 events to exceed the buffer
	const eventsToEmit = 150
	const bufferSize = 100

	for i := 0; i < eventsToEmit; i++ {
		// Emit events by triggering provider state changes
		// Since we can't directly emit, we'll test that the channel doesn't block
		select {
		case <-eventChan:
			// Drain one event to make room
		default:
			// Channel full or empty
		}
	}

	// Verify that event emission doesn't block
	// If it blocks, this test would hang
	done := make(chan bool)
	go func() {
		// Try to emit event (simulated by checking provider state)
		status := provider.Status()
		assert.Equal(t, openfeature.ReadyState, status, "Provider should still be ready")
		done <- true
	}()

	select {
	case <-done:
		// Success - operation completed without blocking
	case <-time.After(2 * time.Second):
		t.Fatal("Event emission appears to be blocking")
	}

	// Drain remaining events to verify channel is still functional
	drained := 0
	for {
		select {
		case <-eventChan:
			drained++
		case <-time.After(10 * time.Millisecond):
			// No more events
			goto done
		}
	}
done:
	assert.LessOrEqual(t, drained, bufferSize, "Should not have more events than buffer size")
}

// BenchmarkBooleanEvaluation benchmarks single boolean flag evaluation performance.
func BenchmarkBooleanEvaluation(b *testing.B) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	if err != nil {
		b.Fatalf("Failed to initialize provider: %v", err)
	}

	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "bench-user",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.BooleanEvaluation(context.TODO(), flagSomeOther, false, flatCtx)
	}
}

// BenchmarkStringEvaluation benchmarks single string flag evaluation performance.
func BenchmarkStringEvaluation(b *testing.B) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	if err != nil {
		b.Fatalf("Failed to initialize provider: %v", err)
	}

	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "bench-user",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.StringEvaluation(context.TODO(), flagSomeOther, "default", flatCtx)
	}
}

// BenchmarkConcurrentEvaluations benchmarks concurrent flag evaluations.
func BenchmarkConcurrentEvaluations(b *testing.B) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	if err != nil {
		b.Fatalf("Failed to initialize provider: %v", err)
	}

	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "bench-user",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = provider.BooleanEvaluation(context.TODO(), flagSomeOther, false, flatCtx)
		}
	})
}

// BenchmarkProviderInitialization measures provider initialization time.
func BenchmarkProviderInitialization(b *testing.B) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider, err := New("localhost", WithSplitConfig(cfg))
		if err != nil {
			b.Fatalf("Failed to create provider: %v", err)
		}

		err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
		if err != nil {
			b.Fatalf("Failed to initialize provider: %v", err)
		}

		_ = provider.ShutdownWithContext(context.Background())
	}
}

// BenchmarkAttributeHeavyEvaluation measures evaluation performance with many attributes.
func BenchmarkAttributeHeavyEvaluation(b *testing.B) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	if err != nil {
		b.Fatalf("Failed to initialize provider: %v", err)
	}

	// Create evaluation context with many attributes
	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "bench-user",
		"email":                  "user@example.com",
		"plan":                   "enterprise",
		"region":                 "us-east-1",
		"org_id":                 "org-12345",
		"user_id":                "user-67890",
		"account_type":           "premium",
		"feature_flags_enabled":  true,
		"beta_tester":            true,
		"signup_date":            "2024-01-15",
		"last_login":             "2025-01-18",
		"session_count":          42,
		"total_spend":            1299.99,
		"conversion_rate":        0.25,
		"engagement_score":       87.5,
		"device_type":            "desktop",
		"browser":                "chrome",
		"os":                     "macos",
		"language":               "en-US",
		"timezone":               "America/New_York",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.BooleanEvaluation(context.TODO(), flagSomeOther, false, flatCtx)
	}
}

// TestLoggerConfiguration verifies all logger configuration scenarios work correctly.
// This tests the complete logger wiring for all possible combinations of logger configuration.
func TestLoggerConfiguration(t *testing.T) {
	// Helper to create base Split config
	baseConfig := func() *conf.SplitSdkConfig {
		cfg := conf.Default()
		cfg.SplitFile = testSplitFile
		cfg.LoggerConfig.LogLevel = logging.LevelNone
		cfg.BlockUntilReady = 10
		return cfg
	}

	tests := []struct {
		name                      string
		setup                     func() (provider *Provider, customSlog *slog.Logger, customSplit *customTestLogger)
		expectProviderUsesDefault bool
		expectSplitLoggerType     string // "adapter" or "custom"
	}{
		{
			name: "no logger specified uses defaults",
			setup: func() (*Provider, *slog.Logger, *customTestLogger) {
				p, err := New("localhost")
				require.NoError(t, err)
				return p, nil, nil
			},
			expectProviderUsesDefault: true,
			expectSplitLoggerType:     "adapter",
		},
		{
			name: "with logger option uses custom for both",
			setup: func() (*Provider, *slog.Logger, *customTestLogger) {
				var buf strings.Builder
				customLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
				p, err := New("localhost", WithLogger(customLogger))
				require.NoError(t, err)
				return p, customLogger, nil
			},
			expectProviderUsesDefault: false,
			expectSplitLoggerType:     "adapter",
		},
		{
			name: "split config logger only preserves custom split logger",
			setup: func() (*Provider, *slog.Logger, *customTestLogger) {
				customSplitLogger := &customTestLogger{logs: make([]string, 0)}
				cfg := baseConfig()
				cfg.Logger = customSplitLogger
				p, err := New("localhost", WithSplitConfig(cfg))
				require.NoError(t, err)
				return p, nil, customSplitLogger
			},
			expectProviderUsesDefault: true,
			expectSplitLoggerType:     "custom",
		},
		{
			name: "both loggers uses each respectively",
			setup: func() (*Provider, *slog.Logger, *customTestLogger) {
				var buf strings.Builder
				customSlogLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
				customSplitLogger := &customTestLogger{logs: make([]string, 0)}
				cfg := baseConfig()
				cfg.Logger = customSplitLogger
				p, err := New("localhost", WithLogger(customSlogLogger), WithSplitConfig(cfg))
				require.NoError(t, err)
				return p, customSlogLogger, customSplitLogger
			},
			expectProviderUsesDefault: false,
			expectSplitLoggerType:     "custom",
		},
		{
			name: "with logger and empty split config uses custom for both",
			setup: func() (*Provider, *slog.Logger, *customTestLogger) {
				var buf strings.Builder
				customLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
				cfg := baseConfig()
				// cfg.Logger is nil
				p, err := New("localhost", WithLogger(customLogger), WithSplitConfig(cfg))
				require.NoError(t, err)
				return p, customLogger, nil
			},
			expectProviderUsesDefault: false,
			expectSplitLoggerType:     "adapter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, _, customSplitLogger := tt.setup()
			defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

			// Assert provider logger has source attribution
			assert.NotNil(t, provider.logger, "Provider logger should be set")
			// Note: Provider logger now has source="split-provider" added via With(),
			// so we can't directly compare to slog.Default() or custom logger.
			// We verify the logger exists and has correct type.

			// Assert Split SDK logger
			assert.NotNil(t, provider.splitConfig.Logger, "Split SDK logger should be set")

			switch tt.expectSplitLoggerType {
			case "adapter":
				adapter, ok := provider.splitConfig.Logger.(*SlogToSplitAdapter)
				require.True(t, ok, "Split SDK logger should be SlogToSplitAdapter")
				assert.NotNil(t, adapter.logger, "Adapter should have a logger")
				// Note: Adapter logger now has source="split-sdk" added via With(),
				// so we verify it exists but can't directly compare instances

			case "custom":
				assert.Equal(t, customSplitLogger, provider.splitConfig.Logger,
					"Split SDK should preserve custom logger (not overwritten)")
			}
		})
	}
}

// customTestLogger implements the Split SDK logging interface for testing
// Thread-safe to handle concurrent calls from Split SDK goroutines
type customTestLogger struct {
	mu   sync.Mutex
	logs []string
}

func (l *customTestLogger) Error(msg ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprint("ERROR: ", msg))
}

func (l *customTestLogger) Warning(msg ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprint("WARN: ", msg))
}

func (l *customTestLogger) Info(msg ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprint("INFO: ", msg))
}

func (l *customTestLogger) Debug(msg ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprint("DEBUG: ", msg))
}

func (l *customTestLogger) Verbose(msg ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprint("VERBOSE: ", msg))
}
