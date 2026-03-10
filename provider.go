package split_openfeature_provider_go

import (
	"context"
	"encoding/json"
	"github.com/splitio/go-client/v6/splitio/conf"
	"strconv"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/client"
)

type SplitProvider struct {
	client *client.SplitClient
}

func NewProvider(splitClient *client.SplitClient) (*SplitProvider, error) {
	if splitClient == nil {
		return nil, ErrNilSplitClient
	}
	return &SplitProvider{
		client: splitClient,
	}, nil
}

func NewProviderSimple(apiKey string) (*SplitProvider, error) {
	cfg := conf.Default()
	factory, err := client.NewSplitFactory(apiKey, cfg)
	if err != nil {
		return nil, err
	}
	splitClient := factory.Client()
	err = splitClient.BlockUntilReady(10)
	if err != nil {
		return nil, err
	}
	return NewProvider(splitClient)
}

func (p *SplitProvider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{
		Name: "Split",
	}
}

func (p *SplitProvider) BooleanEvaluation(ctx context.Context, flag string, defaultValue bool, evalCtx openfeature.FlattenedContext) openfeature.BoolResolutionDetail {
	if noTargetingKey(evalCtx) {
		return openfeature.BoolResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailTargetingKeyMissing(""),
		}
	}
	evaluated, config := p.evaluateTreatmentWithConfig(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.BoolResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailNotFound(evaluated, config),
		}
	}
	var value bool
	if evaluated == "true" || evaluated == "on" {
		value = true
	} else if evaluated == "false" || evaluated == "off" {
		value = false
	} else {
		return openfeature.BoolResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailParseError(evaluated, config),
		}
	}
	return openfeature.BoolResolutionDetail{
		Value:                    value,
		ProviderResolutionDetail: resolutionDetailTargetingMatch(evaluated, config),
	}
}

func (p *SplitProvider) StringEvaluation(ctx context.Context, flag string, defaultValue string, evalCtx openfeature.FlattenedContext) openfeature.StringResolutionDetail {
	if noTargetingKey(evalCtx) {
		return openfeature.StringResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailTargetingKeyMissing(""),
		}
	}
	evaluated, config := p.evaluateTreatmentWithConfig(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.StringResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailNotFound(evaluated, config),
		}
	}
	return openfeature.StringResolutionDetail{
		Value:                    evaluated,
		ProviderResolutionDetail: resolutionDetailTargetingMatch(evaluated, config),
	}
}

func (p *SplitProvider) FloatEvaluation(ctx context.Context, flag string, defaultValue float64, evalCtx openfeature.FlattenedContext) openfeature.FloatResolutionDetail {
	if noTargetingKey(evalCtx) {
		return openfeature.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailTargetingKeyMissing(""),
		}
	}
	evaluated, config := p.evaluateTreatmentWithConfig(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailNotFound(evaluated, config),
		}
	}
	floatEvaluated, parseErr := strconv.ParseFloat(evaluated, 64)
	if parseErr != nil {
		return openfeature.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailParseError(evaluated, config),
		}
	}
	return openfeature.FloatResolutionDetail{
		Value:                    floatEvaluated,
		ProviderResolutionDetail: resolutionDetailTargetingMatch(evaluated, config),
	}
}

func (p *SplitProvider) IntEvaluation(ctx context.Context, flag string, defaultValue int64, evalCtx openfeature.FlattenedContext) openfeature.IntResolutionDetail {
	if noTargetingKey(evalCtx) {
		return openfeature.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailTargetingKeyMissing(""),
		}
	}
	evaluated, config := p.evaluateTreatmentWithConfig(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailNotFound(evaluated, config),
		}
	}
	intEvaluated, parseErr := strconv.ParseInt(evaluated, 10, 64)
	if parseErr != nil {
		return openfeature.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailParseError(evaluated, config),
		}
	}
	return openfeature.IntResolutionDetail{
		Value:                    intEvaluated,
		ProviderResolutionDetail: resolutionDetailTargetingMatch(evaluated, config),
	}
}

func (p *SplitProvider) ObjectEvaluation(ctx context.Context, flag string, defaultValue interface{}, evalCtx openfeature.FlattenedContext) openfeature.InterfaceResolutionDetail {
	if noTargetingKey(evalCtx) {
		return openfeature.InterfaceResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailTargetingKeyMissing(""),
		}
	}
	evaluated, config := p.evaluateTreatmentWithConfig(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.InterfaceResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailNotFound(evaluated, config),
		}
	}
	var data map[string]interface{}
	parseErr := json.Unmarshal([]byte(evaluated), &data)
	if parseErr != nil {
		return openfeature.InterfaceResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailParseError(evaluated, config),
		}
	}
	return openfeature.InterfaceResolutionDetail{
		Value:                    data,
		ProviderResolutionDetail: resolutionDetailTargetingMatch(evaluated, config),
	}
}

func (p *SplitProvider) Hooks() []openfeature.Hook {
	return []openfeature.Hook{}
}

// Track sends a tracking event to Split. It implements the openfeature.Tracker interface.
// Key is taken from evaluationContext.TargetingKey(); traffic type from evaluation context attribute "trafficType".
// If either is missing or empty, Track returns without sending (same as key requirement for evaluations).
func (p *SplitProvider) Track(ctx context.Context, trackingEventName string, evaluationContext openfeature.EvaluationContext, details openfeature.TrackingEventDetails) {
	key := evaluationContext.TargetingKey()
	if key == "" {
		return
	}
	trafficType := evaluationContext.Attribute("trafficType")
	if trafficType == nil {
		return
	}
	trafficTypeStr, ok := trafficType.(string)
	if !ok || trafficTypeStr == "" {
		return
	}
	value := details.Value()
	properties := stringMapFromAttributes(details.Attributes())
	_ = p.client.Track(key, trafficTypeStr, trackingEventName, value, properties)
}

// stringMapFromAttributes converts map[string]any to map[string]interface{} for the Split SDK.
// Returns nil if attrs is nil or empty so the Split client receives nil for optional properties.
func stringMapFromAttributes(attrs map[string]any) map[string]interface{} {
	if attrs == nil || len(attrs) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	return out
}

// *** Helpers ***

const flagMetadataConfigKey = "config"

func (p *SplitProvider) evaluateTreatmentWithConfig(flag string, evalContext openfeature.FlattenedContext) (treatment string, config string) {
	result := p.client.TreatmentWithConfig(evalContext[openfeature.TargetingKey], flag, nil)
	treatment = result.Treatment
	if result.Config != nil {
		config = *result.Config
	}
	return treatment, config
}

func flagMetadataWithConfig(config string) openfeature.FlagMetadata {
	return openfeature.FlagMetadata{flagMetadataConfigKey: config}
}

func noTargetingKey(evalContext openfeature.FlattenedContext) bool {
	_, ok := evalContext[openfeature.TargetingKey]
	return !ok
}

func noTreatment(treatment string) bool {
	return treatment == "" || treatment == "control"
}

func resolutionDetailNotFound(variant string, config string) openfeature.ProviderResolutionDetail {
	return providerResolutionDetailError(
		openfeature.NewFlagNotFoundResolutionError(
			"Flag not found."),
		openfeature.DefaultReason,
		variant,
		config)
}

func resolutionDetailParseError(variant string, config string) openfeature.ProviderResolutionDetail {
	return providerResolutionDetailError(
		openfeature.NewParseErrorResolutionError("Error parsing the treatment to the given type."),
		openfeature.ErrorReason,
		variant,
		config)
}

func resolutionDetailTargetingKeyMissing(config string) openfeature.ProviderResolutionDetail {
	return providerResolutionDetailError(
		openfeature.NewTargetingKeyMissingResolutionError("Targeting key is required and missing."),
		openfeature.ErrorReason,
		"",
		config)
}

func providerResolutionDetailError(err openfeature.ResolutionError, reason openfeature.Reason, variant string, config string) openfeature.ProviderResolutionDetail {
	return openfeature.ProviderResolutionDetail{
		ResolutionError: err,
		Reason:          reason,
		Variant:         variant,
		FlagMetadata:    flagMetadataWithConfig(config),
	}
}

func resolutionDetailTargetingMatch(variant string, config string) openfeature.ProviderResolutionDetail {
	return openfeature.ProviderResolutionDetail{
		Reason:       openfeature.TargetingMatchReason,
		Variant:      variant,
		FlagMetadata: flagMetadataWithConfig(config),
	}
}
