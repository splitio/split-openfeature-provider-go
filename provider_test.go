package split_openfeature_provider_go

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/client"
	"github.com/splitio/go-client/v6/splitio/conf"
)

func create(t *testing.T) (*openfeature.Client, *SplitProvider) {
	t.Helper()
	cfg := conf.Default()
	cfg.SplitFile = "./split.yaml"
	cfg.BlockUntilReady = 10
	factory, err := client.NewSplitFactory("localhost", cfg)
	if err != nil {
		t.Fatal("error creating split factory:", err)
	}
	splitClient := factory.Client()
	if err := splitClient.BlockUntilReady(10); err != nil {
		t.Fatal("split sdk timeout:", err)
	}
	provider, err := NewProvider(splitClient)
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
	if err := openfeature.SetProviderAndWait(provider); err != nil {
		t.Fatal("set provider:", err)
	}
	ofClient := openfeature.NewClient("test_client")
	return ofClient, provider
}

func evaluationContext() openfeature.EvaluationContext {
	return openfeature.NewEvaluationContext("key", map[string]any{})
}

// func TestNewProviderWithAPIKey(t *testing.T) {
// 	provider, err := NewProviderWithAPIKey("localhost")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if provider == nil {
// 		t.Fatal("provider is nil")
// 	}
// }

func TestUseDefault(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "random-non-existent-feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.BooleanValue(ctx, flagName, false, evalCtx)
	if err == nil {
		t.Error("should have returned flag not found error")
	} else if !strings.Contains(err.Error(), string(openfeature.FlagNotFoundCode)) {
		t.Errorf("unexpected error: %s", err.Error())
	} else if result {
		t.Error("result should be default false")
	}
	result, err = ofClient.BooleanValue(ctx, flagName, true, evalCtx)
	if err == nil {
		t.Error("should have returned flag not found error")
	} else if !strings.Contains(err.Error(), string(openfeature.FlagNotFoundCode)) {
		t.Errorf("unexpected error: %s", err.Error())
	} else if !result {
		t.Error("result should be default true")
	}
}

func TestMissingTargetingKey(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "random-non-existent-feature"
	ctx := context.Background()

	result, err := ofClient.BooleanValue(ctx, flagName, false, openfeature.EvaluationContext{})
	if err == nil {
		t.Error("should have returned targeting key missing error")
	} else if !strings.Contains(err.Error(), string(openfeature.TargetingKeyMissingCode)) {
		t.Errorf("unexpected error: %s", err.Error())
	} else if result {
		t.Error("result should be default false")
	}
}

func TestGetControlVariantNonExistentSplit(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "random-non-existent-feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.BooleanValueDetails(ctx, flagName, false, evalCtx)
	if err == nil {
		t.Error("should have returned flag not found error")
	} else if !strings.Contains(err.Error(), string(openfeature.FlagNotFoundCode)) {
		t.Errorf("unexpected error: %s", err.Error())
	} else if result.Value {
		t.Error("result value should be default false")
	} else if result.Variant != "control" {
		t.Error("variant should be control")
	}
}

func TestGetBooleanSplit(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "some_other_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.BooleanValue(ctx, flagName, true, evalCtx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if result {
		t.Error("result should be false as in split.yaml")
	}
}

func TestGetBooleanWithKeySplit(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "my_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.BooleanValue(ctx, flagName, false, evalCtx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if !result {
		t.Error("result should be true as in split.yaml")
	}
	evalCtx = openfeature.NewEvaluationContext("randomKey", map[string]any{})
	result, err = ofClient.BooleanValue(ctx, flagName, true, evalCtx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if result {
		t.Error("result should be false as in split.yaml for randomKey")
	}
}

func TestGetStringSplit(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "some_other_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.StringValue(ctx, flagName, "on", evalCtx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if result != "off" {
		t.Errorf("result want off, got %s", result)
	}
}

func TestGetIntegerSplit(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "int_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.IntValue(ctx, flagName, 0, evalCtx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if result != 32 {
		t.Errorf("result want 32, got %d", result)
	}
}

func TestGetObjectSplit(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "obj_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.ObjectValue(ctx, flagName, nil, evalCtx)
	expected := map[string]any{"key": "value"}
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("result want %v, got %v", expected, result)
	}
}

func TestGetFloatSplit(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "int_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.FloatValue(ctx, flagName, 0, evalCtx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if result != 32 {
		t.Errorf("result want 32, got %f", result)
	}
}

func TestMetadataName(t *testing.T) {
	ofClient, provider := create(t)

	if ofClient.Metadata().Domain() != "test_client" {
		t.Error("client domain should be test_client")
	}
	if provider.Metadata().Name != "Split" {
		t.Errorf("provider name want Split, got %s", provider.Metadata().Name)
	}
}

func TestBooleanDetails(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "some_other_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.BooleanValueDetails(ctx, flagName, true, evalCtx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if result.FlagKey != flagName {
		t.Errorf("flag key want %s, got %s", flagName, result.FlagKey)
	}
	if !strings.Contains(string(result.Reason), string(openfeature.TargetingMatchReason)) {
		t.Errorf("reason want targeting match, got %s", result.Reason)
	}
	if result.Value {
		t.Error("value should be false as in split.yaml")
	}
	if result.Variant != "off" {
		t.Errorf("variant want off, got %s", result.Variant)
	}
	if result.ErrorCode != "" {
		t.Errorf("unexpected error code: %s", result.ErrorCode)
	}
}

func TestIntegerDetails(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "int_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.IntValueDetails(ctx, flagName, 0, evalCtx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if result.FlagKey != flagName {
		t.Errorf("flag key want %s, got %s", flagName, result.FlagKey)
	}
	if !strings.Contains(string(result.Reason), string(openfeature.TargetingMatchReason)) {
		t.Errorf("reason want targeting match, got %s", result.Reason)
	}
	if result.Value != 32 {
		t.Errorf("value want 32, got %d", result.Value)
	}
	if result.Variant != "32" {
		t.Errorf("variant want 32, got %s", result.Variant)
	}
	if result.ErrorCode != "" {
		t.Errorf("unexpected error code: %s", result.ErrorCode)
	}
}

func TestStringDetails(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "some_other_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.StringValueDetails(ctx, flagName, "blah", evalCtx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if result.FlagKey != flagName {
		t.Errorf("flag key want %s, got %s", flagName, result.FlagKey)
	}
	if result.Value != "off" {
		t.Errorf("value want off, got %s", result.Value)
	}
	if result.Variant != "off" {
		t.Errorf("variant want off, got %s", result.Variant)
	}
}

func TestObjectDetails(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "obj_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.ObjectValueDetails(ctx, flagName, map[string]any{}, evalCtx)
	expected := map[string]any{"key": "value"}
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if result.FlagKey != flagName {
		t.Errorf("flag key want %s, got %s", flagName, result.FlagKey)
	}
	if !reflect.DeepEqual(result.Value, expected) {
		t.Errorf("value want %v, got %v", expected, result.Value)
	}
	if result.Variant != "{\"key\": \"value\"}" {
		t.Errorf("variant want JSON, got %s", result.Variant)
	}
}

func TestFloatDetails(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "int_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.FloatValueDetails(ctx, flagName, 0, evalCtx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if result.Value != 32 {
		t.Errorf("value want 32, got %f", result.Value)
	}
	if result.Variant != "32" {
		t.Errorf("variant want 32, got %s", result.Variant)
	}
	flagName = "float_feature"
	result, err = ofClient.FloatValueDetails(ctx, flagName, 0, evalCtx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if result.Value != 32.5 {
		t.Errorf("value want 32.5, got %f", result.Value)
	}
	if result.Variant != "32.5" {
		t.Errorf("variant want 32.5, got %s", result.Variant)
	}
}

func TestBooleanFail(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "obj_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.BooleanValue(ctx, flagName, false, evalCtx)
	if err == nil {
		t.Error("expected parse error")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("expected parse error, got %s", err.Error())
	}
	if result {
		t.Error("result should be default false")
	}
	resultDetails, err := ofClient.BooleanValueDetails(ctx, flagName, false, evalCtx)
	if err == nil {
		t.Error("expected parse error")
	} else if resultDetails.ErrorCode != openfeature.ParseErrorCode {
		t.Errorf("error code want %s, got %s", openfeature.ParseErrorCode, resultDetails.ErrorCode)
	} else if resultDetails.Reason != openfeature.ErrorReason {
		t.Errorf("reason want %s, got %s", openfeature.ErrorReason, resultDetails.Reason)
	}
}

func TestIntegerFail(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "obj_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.IntValue(ctx, flagName, 10, evalCtx)
	if err == nil {
		t.Error("expected parse error")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("expected parse error, got %s", err.Error())
	}
	if result != 10 {
		t.Errorf("result should be default 10, got %d", result)
	}
	resultDetails, err := ofClient.IntValueDetails(ctx, flagName, 10, evalCtx)
	if err == nil {
		t.Error("expected parse error")
	} else if resultDetails.ErrorCode != openfeature.ParseErrorCode {
		t.Errorf("error code want %s, got %s", openfeature.ParseErrorCode, resultDetails.ErrorCode)
	}
}

func TestFloatFail(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "obj_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()

	result, err := ofClient.FloatValue(ctx, flagName, 10, evalCtx)
	if err == nil {
		t.Error("expected parse error")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("expected parse error, got %s", err.Error())
	}
	if result != 10 {
		t.Errorf("result should be default 10, got %f", result)
	}
	resultDetails, err := ofClient.FloatValueDetails(ctx, flagName, 10, evalCtx)
	if err == nil {
		t.Error("expected parse error")
	} else if resultDetails.ErrorCode != openfeature.ParseErrorCode {
		t.Errorf("error code want %s, got %s", openfeature.ParseErrorCode, resultDetails.ErrorCode)
	}
}

func TestObjectFail(t *testing.T) {
	ofClient, _ := create(t)
	flagName := "int_feature"
	evalCtx := evaluationContext()
	ctx := context.Background()
	defaultTreatment := map[string]any{"key": "value"}

	result, err := ofClient.ObjectValue(ctx, flagName, defaultTreatment, evalCtx)
	if err == nil {
		t.Error("expected parse error")
	} else if !strings.Contains(err.Error(), string(openfeature.ParseErrorCode)) {
		t.Errorf("expected parse error, got %s", err.Error())
	}
	if !reflect.DeepEqual(result, defaultTreatment) {
		t.Error("result should be default")
	}
	resultDetails, err := ofClient.ObjectValueDetails(ctx, flagName, defaultTreatment, evalCtx)
	if err == nil {
		t.Error("expected parse error")
	} else if resultDetails.ErrorCode != openfeature.ParseErrorCode {
		t.Errorf("error code want %s, got %s", openfeature.ParseErrorCode, resultDetails.ErrorCode)
	} else if resultDetails.Variant != "32" {
		t.Errorf("variant want 32, got %s", resultDetails.Variant)
	}
}
