package split_openfeature_provider_go

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/splitio/go-client/v6/splitio/conf"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/client"
)

type SplitProvider struct {
	client client.SplitClient
}

func NewProvider(splitClient client.SplitClient) (*SplitProvider, error) {
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
	return NewProvider(*splitClient)
}

func (provider *SplitProvider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{
		Name: "Split",
	}
}

func (provider *SplitProvider) BooleanEvaluation(ctx context.Context, flag string, defaultValue bool, evalCtx openfeature.FlattenedContext) openfeature.BoolResolutionDetail {
	if noTargetingKey(evalCtx) {
		return openfeature.BoolResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailTargetingKeyMissing(),
		}
	}
	evaluated := provider.evaluateTreatment(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.BoolResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailNotFound(evaluated),
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
			ProviderResolutionDetail: resolutionDetailParseError(evaluated),
		}
	}
	return openfeature.BoolResolutionDetail{
		Value:                    value,
		ProviderResolutionDetail: resolutionDetailTargetingMatch(evaluated),
	}
}

func (provider *SplitProvider) StringEvaluation(ctx context.Context, flag string, defaultValue string, evalCtx openfeature.FlattenedContext) openfeature.StringResolutionDetail {
	if noTargetingKey(evalCtx) {
		return openfeature.StringResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailTargetingKeyMissing(),
		}
	}
	evaluated := provider.evaluateTreatment(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.StringResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailNotFound(evaluated),
		}
	}
	return openfeature.StringResolutionDetail{
		Value:                    evaluated,
		ProviderResolutionDetail: resolutionDetailTargetingMatch(evaluated),
	}
}

func (provider *SplitProvider) FloatEvaluation(ctx context.Context, flag string, defaultValue float64, evalCtx openfeature.FlattenedContext) openfeature.FloatResolutionDetail {
	if noTargetingKey(evalCtx) {
		return openfeature.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailTargetingKeyMissing(),
		}
	}
	evaluated := provider.evaluateTreatment(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailNotFound(evaluated),
		}
	}
	floatEvaluated, parseErr := strconv.ParseFloat(evaluated, 64)
	if parseErr != nil {
		return openfeature.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailParseError(evaluated),
		}
	}
	return openfeature.FloatResolutionDetail{
		Value:                    floatEvaluated,
		ProviderResolutionDetail: resolutionDetailTargetingMatch(evaluated),
	}
}

func (provider *SplitProvider) IntEvaluation(ctx context.Context, flag string, defaultValue int64, evalCtx openfeature.FlattenedContext) openfeature.IntResolutionDetail {
	if noTargetingKey(evalCtx) {
		return openfeature.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailTargetingKeyMissing(),
		}
	}
	evaluated := provider.evaluateTreatment(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailNotFound(evaluated),
		}
	}
	intEvaluated, parseErr := strconv.ParseInt(evaluated, 10, 64)
	if parseErr != nil {
		return openfeature.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailParseError(evaluated),
		}
	}
	return openfeature.IntResolutionDetail{
		Value:                    intEvaluated,
		ProviderResolutionDetail: resolutionDetailTargetingMatch(evaluated),
	}
}

func (provider *SplitProvider) ObjectEvaluation(ctx context.Context, flag string, defaultValue interface{}, evalCtx openfeature.FlattenedContext) openfeature.InterfaceResolutionDetail {
	if noTargetingKey(evalCtx) {
		return openfeature.InterfaceResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailTargetingKeyMissing(),
		}
	}
	evaluated := provider.evaluateTreatment(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.InterfaceResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailNotFound(evaluated),
		}
	}
	var data map[string]interface{}
	parseErr := json.Unmarshal([]byte(evaluated), &data)
	if parseErr != nil {
		return openfeature.InterfaceResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailParseError(evaluated),
		}
	} else {
		return openfeature.InterfaceResolutionDetail{
			Value:                    data,
			ProviderResolutionDetail: resolutionDetailTargetingMatch(evaluated),
		}
	}

}

func (provider *SplitProvider) Hooks() []openfeature.Hook {
	return []openfeature.Hook{}
}

// *** Helpers ***

func (provider *SplitProvider) evaluateTreatment(flag string, evalContext openfeature.FlattenedContext) string {
	return provider.client.Treatment(evalContext[openfeature.TargetingKey], flag, nil)
}

func noTargetingKey(evalContext openfeature.FlattenedContext) bool {
	_, ok := evalContext[openfeature.TargetingKey]
	return !ok
}

func noTreatment(treatment string) bool {
	return treatment == "" || treatment == "control"
}

func resolutionDetailNotFound(variant string) openfeature.ProviderResolutionDetail {
	return providerResolutionDetailError(
		openfeature.NewFlagNotFoundResolutionError(
			"Flag not found."),
		openfeature.DefaultReason,
		variant)
}

func resolutionDetailParseError(variant string) openfeature.ProviderResolutionDetail {
	return providerResolutionDetailError(
		openfeature.NewParseErrorResolutionError("Error parsing the treatment to the given type."),
		openfeature.ErrorReason,
		variant)
}

func resolutionDetailTargetingKeyMissing() openfeature.ProviderResolutionDetail {
	return providerResolutionDetailError(
		openfeature.NewTargetingKeyMissingResolutionError("Targeting key is required and missing."),
		openfeature.ErrorReason,
		"")
}

func providerResolutionDetailError(error openfeature.ResolutionError, reason openfeature.Reason, variant string) openfeature.ProviderResolutionDetail {
	return openfeature.ProviderResolutionDetail{
		ResolutionError: error,
		Reason:          reason,
		Variant:         variant,
	}
}

func resolutionDetailTargetingMatch(variant string) openfeature.ProviderResolutionDetail {
	return openfeature.ProviderResolutionDetail{
		Reason:  openfeature.TargetingMatchReason,
		Variant: variant,
	}
}
