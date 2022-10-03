package split_openfeature_provider_go

import (
	"encoding/json"
	"github.com/splitio/go-client/splitio/conf"
	"strconv"

	"github.com/open-feature/go-sdk/pkg/openfeature"
	"github.com/splitio/go-client/splitio/client"
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

func (provider *SplitProvider) BooleanEvaluation(flag string, defaultValue bool, evalCtx map[string]interface{}) openfeature.BoolResolutionDetail {
	var evaluated, err = provider.evaluateTreatment(flag, evalCtx)
	if err != "" {
		return openfeature.BoolResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailError(err),
		}
	}
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
		ProviderResolutionDetail: resolutionDetailFound(evaluated),
	}
}

func (provider *SplitProvider) StringEvaluation(flag string, defaultValue string, evalCtx map[string]interface{}) openfeature.StringResolutionDetail {
	var evaluated, err = provider.evaluateTreatment(flag, evalCtx)
	if err != "" {
		return openfeature.StringResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailError(err),
		}
	}
	if noTreatment(evaluated) {
		return openfeature.StringResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailNotFound(evaluated),
		}
	}
	return openfeature.StringResolutionDetail{
		Value:                    evaluated,
		ProviderResolutionDetail: resolutionDetailFound(evaluated),
	}
}

func (provider *SplitProvider) FloatEvaluation(flag string, defaultValue float64, evalCtx map[string]interface{}) openfeature.FloatResolutionDetail {
	var evaluated, err = provider.evaluateTreatment(flag, evalCtx)
	if err != "" {
		return openfeature.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailError(err),
		}
	}
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
		ProviderResolutionDetail: resolutionDetailFound(evaluated),
	}
}

func (provider *SplitProvider) IntEvaluation(flag string, defaultValue int64, evalCtx map[string]interface{}) openfeature.IntResolutionDetail {
	var evaluated, err = provider.evaluateTreatment(flag, evalCtx)
	if err != "" {
		return openfeature.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailError(err),
		}
	}
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
		ProviderResolutionDetail: resolutionDetailFound(evaluated),
	}
}

func (provider *SplitProvider) ObjectEvaluation(flag string, defaultValue interface{}, evalCtx map[string]interface{}) openfeature.InterfaceResolutionDetail {
	var evaluated, err = provider.evaluateTreatment(flag, evalCtx)
	if err != "" {
		return openfeature.InterfaceResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: resolutionDetailError(err),
		}
	}
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
			ProviderResolutionDetail: resolutionDetailFound(evaluated),
		}
	}

}

func (provider *SplitProvider) Hooks() []openfeature.Hook {
	return []openfeature.Hook{}
}

// *** Helpers ***

func (provider *SplitProvider) evaluateTreatment(flag string, evalContext map[string]interface{}) (string, openfeature.ErrorCode) {
	if targetingKey, ok := evalContext[openfeature.TargetingKey]; ok {
		return provider.client.Treatment(targetingKey, flag, nil), ""
	} else {
		return "control", openfeature.TargetingKeyMissingCode
	}

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
func resolutionDetailFound(variant string) openfeature.ProviderResolutionDetail {
	return providerResolutionDetail(
		openfeature.TargetingMatchReason,
		variant)
}

func resolutionDetailParseError(variant string) openfeature.ProviderResolutionDetail {
	return providerResolutionDetailError(
		openfeature.NewParseErrorResolutionError("Error parsing the treatment to the given type."),
		openfeature.ErrorReason,
		variant)
}

func resolutionDetailError(err openfeature.ErrorCode) openfeature.ProviderResolutionDetail {
	return providerResolutionDetailError(
		openfeature.NewGeneralResolutionError(string(err)),
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

func providerResolutionDetail(reason openfeature.Reason, variant string) openfeature.ProviderResolutionDetail {
	return openfeature.ProviderResolutionDetail{
		Reason:  reason,
		Variant: variant,
	}
}
