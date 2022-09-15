package split_openfeature_provider_go

import (
	"encoding/json"
	"strconv"

	"github.com/open-feature/golang-sdk/pkg/openfeature"
	"github.com/splitio/go-client/splitio/client"
)

const (
	// errors
	flagNotFound string = "FLAG_NOT_FOUND"
	parseError   string = "PARSE_ERROR"

	// reasons
	defaultReason  string = "DEFAULT"
	targetingMatch string = "TARGETING_MATCH"
	errorReason    string = "ERROR"
)

type SplitProvider struct {
	client client.SplitClient
}

func NewProvider(splitClient client.SplitClient) *SplitProvider {
	return &SplitProvider{
		client: splitClient,
	}
}

// note: use pointer receiver if I need to make a modification to the provider

func (provider *SplitProvider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{
		Name: "Split",
	}
}

func (provider *SplitProvider) BooleanEvaluation(flag string, defaultValue bool, evalCtx openfeature.EvaluationContext) openfeature.BoolResolutionDetail {
	var evaluated = provider.evaluateTreatment(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.BoolResolutionDetail{
			Value:            defaultValue,
			ResolutionDetail: resolutionDetailNotFound(evaluated),
		}
	}
	var value bool
	if evaluated == "true" || evaluated == "on" {
		value = true
	} else if evaluated == "false" || evaluated == "off" {
		value = false
	} else {
		return openfeature.BoolResolutionDetail{
			Value:            defaultValue,
			ResolutionDetail: resolutionDetailParseError(evaluated),
		}
	}
	return openfeature.BoolResolutionDetail{
		Value:            value,
		ResolutionDetail: resolutionDetailFound(evaluated),
	}
}

func (provider *SplitProvider) StringEvaluation(flag string, defaultValue string, evalCtx openfeature.EvaluationContext) openfeature.StringResolutionDetail {
	var evaluated = provider.evaluateTreatment(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.StringResolutionDetail{
			Value:            defaultValue,
			ResolutionDetail: resolutionDetailNotFound(evaluated),
		}
	}
	return openfeature.StringResolutionDetail{
		Value:            evaluated,
		ResolutionDetail: resolutionDetailFound(evaluated),
	}
}

func (provider *SplitProvider) FloatEvaluation(flag string, defaultValue float64, evalCtx openfeature.EvaluationContext) openfeature.FloatResolutionDetail {
	var evaluated = provider.evaluateTreatment(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.FloatResolutionDetail{
			Value:            defaultValue,
			ResolutionDetail: resolutionDetailNotFound(evaluated),
		}
	}
	floatEvaluated, err := strconv.ParseFloat(evaluated, 64)
	if err != nil {
		return openfeature.FloatResolutionDetail{
			Value:            defaultValue,
			ResolutionDetail: resolutionDetailParseError(evaluated),
		}
	}
	return openfeature.FloatResolutionDetail{
		Value:            floatEvaluated,
		ResolutionDetail: resolutionDetailFound(evaluated),
	}
}

func (provider *SplitProvider) IntEvaluation(flag string, defaultValue int64, evalCtx openfeature.EvaluationContext) openfeature.IntResolutionDetail {
	var evaluated = provider.evaluateTreatment(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.IntResolutionDetail{
			Value:            defaultValue,
			ResolutionDetail: resolutionDetailNotFound(evaluated),
		}
	}
	intEvaluated, err := strconv.ParseInt(evaluated, 10, 64)
	if err != nil {
		return openfeature.IntResolutionDetail{
			Value:            defaultValue,
			ResolutionDetail: resolutionDetailParseError(evaluated),
		}
	}
	return openfeature.IntResolutionDetail{
		Value:            intEvaluated,
		ResolutionDetail: resolutionDetailFound(evaluated),
	}
}

func (provider *SplitProvider) ObjectEvaluation(flag string, defaultValue interface{}, evalCtx openfeature.EvaluationContext) openfeature.InterfaceResolutionDetail {
	var evaluated = provider.evaluateTreatment(flag, evalCtx)
	if noTreatment(evaluated) {
		return openfeature.InterfaceResolutionDetail{
			Value:            defaultValue,
			ResolutionDetail: resolutionDetailNotFound(evaluated),
		}
	}
	var data map[string]interface{}
	err := json.Unmarshal([]byte(evaluated), &data)
	if err != nil {
		return openfeature.InterfaceResolutionDetail{
			Value:            defaultValue,
			ResolutionDetail: resolutionDetailParseError(evaluated),
		}
	} else {
		return openfeature.InterfaceResolutionDetail{
			Value:            defaultValue,
			ResolutionDetail: resolutionDetailFound(evaluated),
		}
	}

}

func (provider *SplitProvider) Hooks() []openfeature.Hook {
	return []openfeature.Hook{}
}

// *** Helpers ***

func (provider *SplitProvider) evaluateTreatment(flag string, evalContext openfeature.EvaluationContext) string {
	targetingKey := evalContext.TargetingKey
	return provider.client.Treatment(targetingKey, flag, nil)
}

func noTreatment(treatment string) bool {
	return treatment == "" || treatment == "control"
}

func resolutionDetailNotFound(variant string) openfeature.ResolutionDetail {
	return resolutionDetail(flagNotFound, defaultReason, variant)
}
func resolutionDetailFound(variant string) openfeature.ResolutionDetail {
	return resolutionDetail("", targetingMatch, variant)
}

func resolutionDetailParseError(variant string) openfeature.ResolutionDetail {
	return resolutionDetail(parseError, errorReason, variant)
}

func resolutionDetail(error string, reason string, variant string) openfeature.ResolutionDetail {
	return openfeature.ResolutionDetail{
		ErrorCode: error,
		Reason:    reason,
		Variant:   variant,
	}
}
