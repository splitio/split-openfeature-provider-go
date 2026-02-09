package split_openfeature_provider_go

import (
	"reflect"
	"strings"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/client"
	"github.com/splitio/go-client/v6/splitio/conf"
	"github.com/splitio/go-toolkit/logging"
)

func create(t *testing.T) *openfeature.Client {
	cfg := conf.Default()
	cfg.SplitFile = "./split.yaml"
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	factory, err := client.NewSplitFactory("localhost", cfg)
	if err != nil {
		// error
		t.Error("Error creating split factory")
	}
	splitClient := factory.Client()
	err = splitClient.BlockUntilReady(10)
	if err != nil {
		// error timeout
		t.Error("Split sdk timeout error")
	}
	provider, err := NewProvider(*splitClient)
	if err != nil {
		t.Error(err)
	}
	if provider == nil {
		t.Error("Error creating Split Provider")
	}
	openfeature.SetProvider(provider)
	return openfeature.NewClient("test_client")
}

func evaluationContext() openfeature.EvaluationContext {
	return openfeature.NewEvaluationContext("key", nil)
}

func TestCreateSimple(t *testing.T) {
	provider, err := NewProviderSimple("localhost")
	if err != nil {
		t.Error(err)
	}
	if provider == nil {
		t.Error("Error creating Split Provider")
	}
}

func TestUseDefault(t *testing.T) {
	ofClient := create(t)
	flagName := "random-non-existent-feature"
	evalCtx := evaluationContext()

	result, err := ofClient.BooleanValue(t.Context(), flagName, false, evalCtx)
	if err == nil {
		t.Error("Should have returned flag not found error")
	} else if !strings.Contains(err.Error(), string(openfeature.FlagNotFoundCode)) {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result == true {
		t.Error("Result was true, but should have been default value of false")
	}
	result, err = ofClient.BooleanValue(t.Context(), flagName, true, evalCtx)
	if err == nil {
		t.Error("Should have returned flag not found error")
	} else if !strings.Contains(err.Error(), string(openfeature.FlagNotFoundCode)) {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result == false {
		t.Error("Result was false, but should have been default value of true")
	}
}

func TestMissingTargetingKey(t *testing.T) {
	ofClient := create(t)
	flagName := "random-non-existent-feature"

	result, err := ofClient.BooleanValue(t.Context(), flagName, false, openfeature.EvaluationContext{})
	if err == nil {
		t.Error("Should have returned targeting key missing error")
	} else if !strings.Contains(err.Error(), string(openfeature.TargetingKeyMissingCode)) {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result == true {
		t.Error("Result was true, but should have been default value of false")
	}
}

func TestGetControlVariantNonExistentSplit(t *testing.T) {
	ofClient := create(t)
	flagName := "random-non-existent-feature"
	evalCtx := evaluationContext()

	result, err := ofClient.BooleanValueDetails(t.Context(), flagName, false, evalCtx)
	if err == nil {
		t.Error("Should have returned flag not found error")
	} else if !strings.Contains(err.Error(), string(openfeature.FlagNotFoundCode)) {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result.Value == true {
		t.Error("Result was true, but should have been default value of false")
	} else if result.Variant != "control" {
		t.Error("Variant should be control due to Split Go SDK functionality")
	}
}

func TestGetBooleanSplit(t *testing.T) {
	ofClient := create(t)
	flagName := "some_other_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.BooleanValue(t.Context(), flagName, true, evalCtx)
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result == true {
		t.Error("Result was true, but should have been false as set in split.yaml")
	}
}

func TestGetBooleanWithKeySplit(t *testing.T) {
	ofClient := create(t)
	flagName := "my_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.BooleanValue(t.Context(), flagName, false, evalCtx)
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result == false {
		t.Error("Result was false, but should have been true as set in split.yaml")
	}

	evalCtx = openfeature.NewEvaluationContext("randomKey", nil)
	result, err = ofClient.BooleanValue(t.Context(), flagName, true, evalCtx)
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result == true {
		t.Error("Result was true, but should have been false as set in split.yaml")
	}
}

func TestGetStringSplit(t *testing.T) {
	ofClient := create(t)
	flagName := "some_other_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.StringValue(t.Context(), flagName, "on", evalCtx)
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result != "off" {
		t.Errorf("Result was %s, not off as set in split.yaml", result)
	}
}

func TestGetIntegerSplit(t *testing.T) {
	ofClient := create(t)
	flagName := "int_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.IntValue(t.Context(), flagName, 0, evalCtx)
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result != 32 {
		t.Errorf("Result was %d, not 32 as set in split.yaml", result)
	}
}

func TestGetObjectSplit(t *testing.T) {
	ofClient := create(t)
	flagName := "obj_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.ObjectValue(t.Context(), flagName, 0, evalCtx)
	expectedResult := map[string]interface{}{
		"desc": "this applies only to my_variant treatment",
	}
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if !reflect.DeepEqual(result, expectedResult) {
		t.Error("Result was not map from key to value as set in split.yaml")
	}
}

func TestGetFloatSplit(t *testing.T) {
	ofClient := create(t)
	flagName := "int_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.FloatValue(t.Context(), flagName, 0, evalCtx)
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result != float64(32) {
		t.Errorf("Result was %f, not 32 as set in split.yaml", result)
	}
}

func TestMetadataName(t *testing.T) {
	ofClient := create(t)
	if ofClient.Metadata().Name() != "test_client" {
		t.Error("Client name was not set properly")
	}
	if openfeature.ProviderMetadata().Name != "Split" {
		t.Errorf("Provider metadata name was %s, not Split", openfeature.ProviderMetadata().Name)
	}
}

func TestBooleanDetails(t *testing.T) {
	ofClient := create(t)
	flagName := "some_other_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.BooleanValueDetails(t.Context(), flagName, true, evalCtx)
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result.FlagKey != flagName {
		t.Errorf("Flag name is %s, not %s", result.FlagKey, flagName)
	} else if !strings.Contains(string(result.Reason), string(openfeature.TargetingMatchReason)) {
		t.Errorf("reason is %s, not targeting match", result.Reason)
	} else if result.Value == true {
		t.Error("Result was true, but should have been false as in split.yaml")
	} else if result.Variant != "off" {
		t.Errorf("Variant should be off as in split.yaml, but was %s", result.Variant)
	} else if result.ErrorCode != "" {
		t.Errorf("Unexpected error in result %s", result.ErrorCode)
	}
}

func TestIntegerDetails(t *testing.T) {
	ofClient := create(t)
	flagName := "int_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.IntValueDetails(t.Context(), flagName, 0, evalCtx)
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result.FlagKey != flagName {
		t.Errorf("Flag name is %s, not %s", result.FlagKey, flagName)
	} else if !strings.Contains(string(result.Reason), string(openfeature.TargetingMatchReason)) {
		t.Errorf("reason is %s, not targeting match", result.Reason)
	} else if result.Value != int64(32) {
		t.Errorf("Result was %d, but should have been 32 as in split.yaml", result.Value)
	} else if result.Variant != "32" {
		t.Errorf("Variant should be 32 as in split.yaml, but was %s", result.Variant)
	} else if result.ErrorCode != "" {
		t.Errorf("Unexpected error in result %s", result.ErrorCode)
	}
}

func TestStringDetails(t *testing.T) {
	ofClient := create(t)
	flagName := "some_other_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.StringValueDetails(t.Context(), flagName, "blah", evalCtx)
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result.FlagKey != flagName {
		t.Errorf("Flag name is %s, not %s", result.FlagKey, flagName)
	} else if !strings.Contains(string(result.Reason), string(openfeature.TargetingMatchReason)) {
		t.Errorf("reason is %s, not targeting match", result.Reason)
	} else if result.Value != "off" {
		t.Errorf("Result was %s, but should have been off as in split.yaml", result.Value)
	} else if result.Variant != "off" {
		t.Errorf("Variant should be off as in split.yaml, but was %s", result.Variant)
	} else if result.ErrorCode != "" {
		t.Errorf("Unexpected error in result %s", result.ErrorCode)
	}
}

func TestObjectDetails(t *testing.T) {
	ofClient := create(t)
	flagName := "obj_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.ObjectValueDetails(t.Context(), flagName, map[string]interface{}{}, evalCtx)
	expectedResult := map[string]interface{}{
		"desc": "this applies only to my_variant treatment",
	}
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result.FlagKey != flagName {
		t.Errorf("Flag name is %s, not %s", result.FlagKey, flagName)
	} else if !strings.Contains(string(result.Reason), string(openfeature.TargetingMatchReason)) {
		t.Errorf("reason is %s, not targeting match", result.Reason)
	} else if !reflect.DeepEqual(result.Value, expectedResult) {
		t.Error("Result was not map of key->value as in split.yaml")
	} else if result.Variant != "my_variant" {
		t.Errorf("Variant should be on as in split.yaml, but was %s", result.Variant)
	} else if result.ErrorCode != "" {
		t.Errorf("Unexpected error in result %s", result.ErrorCode)
	}
}

func TestFloatDetails(t *testing.T) {
	ofClient := create(t)
	flagName := "int_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.FloatValueDetails(t.Context(), flagName, 0, evalCtx)
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result.FlagKey != flagName {
		t.Errorf("Flag name is %s, not %s", result.FlagKey, flagName)
	} else if !strings.Contains(string(result.Reason), string(openfeature.TargetingMatchReason)) {
		t.Errorf("reason is %s, not targeting match", result.Reason)
	} else if result.Value != float64(32) {
		t.Errorf("Result was %f, but should have been 32 as in split.yaml", result.Value)
	} else if result.Variant != "32" {
		t.Errorf("Variant should be 32 as in split.yaml, but was %s", result.Variant)
	} else if result.ErrorCode != "" {
		t.Errorf("Unexpected error in result %s", result.ErrorCode)
	}

	flagName = "float_feature"
	result, err = ofClient.FloatValueDetails(t.Context(), flagName, 0, evalCtx)
	if err != nil {
		t.Errorf("Unexpected error occurred %s", err.Error())
	} else if result.Value != 32.5 {
		t.Errorf("Result was %f, but should have been 32.5 as in split.yaml", result.Value)
	} else if result.Variant != "32.5" {
		t.Errorf("Variant should be 32 as in split.yaml, but was %s", result.Variant)
	} else if result.ErrorCode != "" {
		t.Errorf("Unexpected error in result %s", result.ErrorCode)
	}
}

func TestBooleanFail(t *testing.T) {
	// attempt to fetch an object treatment as a boolean. Should result in the default
	ofClient := create(t)
	flagName := "obj_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.BooleanValue(t.Context(), flagName, false, evalCtx)
	if err == nil {
		t.Error("Expected error to occur")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("Expected parse error, got %s", err.Error())
	} else if result != false {
		t.Error("Result was true, but should have been default of false")
	}

	resultDetails, err := ofClient.BooleanValueDetails(t.Context(), flagName, false, evalCtx)
	if err == nil {
		t.Error("Expected error to occur")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("Expected parse error, got %s", err.Error())
	} else if resultDetails.Value != false {
		t.Error("Result was true, but should have been default of false")
	} else if resultDetails.ErrorCode != openfeature.ParseErrorCode {
		t.Errorf("Expected parse error code, got %s", resultDetails.ErrorCode)
	} else if resultDetails.Reason != openfeature.ErrorReason {
		t.Errorf("Expected error reason code, got %s", resultDetails.Reason)
	} else if resultDetails.Variant != "my_variant" {
		t.Errorf("Expected variant to be string of map, got %s", resultDetails.Variant)
	}
}

func TestIntegerFail(t *testing.T) {
	// attempt to fetch an object treatment as an integer. Should result in the default
	ofClient := create(t)
	flagName := "obj_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.IntValue(t.Context(), flagName, 10, evalCtx)
	if err == nil {
		t.Error("Expected error to occur")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("Expected parse error, got %s", err.Error())
	} else if result != int64(10) {
		t.Errorf("Result was %d, but should have been default of 10", result)
	}

	resultDetails, err := ofClient.IntValueDetails(t.Context(), flagName, 10, evalCtx)
	if err == nil {
		t.Error("Expected error to occur")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("Expected parse error, got %s", err.Error())
	} else if resultDetails.Value != int64(10) {
		t.Errorf("Result was %d, but should have been default of 10", resultDetails.Value)
	} else if resultDetails.ErrorCode != openfeature.ParseErrorCode {
		t.Errorf("Expected parse error code, got %s", resultDetails.ErrorCode)
	} else if resultDetails.Reason != openfeature.ErrorReason {
		t.Errorf("Expected error reason code, got %s", resultDetails.Reason)
	} else if resultDetails.Variant != "my_variant" {
		t.Errorf("Expected variant to be string of map, got %s", resultDetails.Variant)
	}
}

func TestFloatFail(t *testing.T) {
	// attempt to fetch an object treatment as a float. Should result in the default
	ofClient := create(t)
	flagName := "obj_feature"
	evalCtx := evaluationContext()

	result, err := ofClient.FloatValue(t.Context(), flagName, 10, evalCtx)
	if err == nil {
		t.Error("Expected error to occur")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("Expected parse error, got %s", err.Error())
	} else if result != float64(10) {
		t.Errorf("Result was %f, but should have been default of 10", result)
	}

	resultDetails, err := ofClient.FloatValueDetails(t.Context(), flagName, 10, evalCtx)
	if err == nil {
		t.Error("Expected error to occur")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("Expected parse error, got %s", err.Error())
	} else if resultDetails.Value != float64(10) {
		t.Errorf("Result was %f, but should have been default of 10", resultDetails.Value)
	} else if resultDetails.ErrorCode != openfeature.ParseErrorCode {
		t.Errorf("Expected parse error code, got %s", resultDetails.ErrorCode)
	} else if resultDetails.Reason != openfeature.ErrorReason {
		t.Errorf("Expected error reason code, got %s", resultDetails.Reason)
	} else if resultDetails.Variant != "my_variant" {
		t.Errorf("Expected variant to be string of map, got %s", resultDetails.Variant)
	}
}

func TestObjectFail(t *testing.T) {
	// attempt to fetch an int as an object. Should result in the default
	ofClient := create(t)
	flagName := "int_feature"
	evalCtx := evaluationContext()
	defaultTreatment := map[string]interface{}{
		"key": "value",
	}

	result, err := ofClient.ObjectValue(t.Context(), flagName, defaultTreatment, evalCtx)
	if err == nil {
		t.Error("Expected error to occur")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("Expected parse error, got %s", err.Error())
	} else if !reflect.DeepEqual(result, defaultTreatment) {
		t.Error("Result was not default treatment")
	}

	resultDetails, err := ofClient.ObjectValueDetails(t.Context(), flagName, defaultTreatment, evalCtx)
	if err == nil {
		t.Error("Expected error to occur")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("Expected parse error, got %s", err.Error())
	} else if !reflect.DeepEqual(resultDetails.Value, defaultTreatment) {
		t.Errorf("Result was %f, but should have been default of 10", resultDetails.Value)
	} else if resultDetails.ErrorCode != openfeature.ParseErrorCode {
		t.Errorf("Expected parse error code, got %s", resultDetails.ErrorCode)
	} else if resultDetails.Reason != openfeature.ErrorReason {
		t.Errorf("Expected error reason code, got %s", resultDetails.Reason)
	} else if resultDetails.Variant != "32" {
		t.Errorf("Expected variant to be string of integer, got %s", resultDetails.Variant)
	}
}
