package split_openfeature_provider_go

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/client"
	"github.com/splitio/go-client/v6/splitio/conf"
)

const (
	// Metadata key for Split treatment config (JSON string), aligned with other Split OpenFeature providers.
	flagMetadataConfigKey = "config"
)

// SplitProvider implements openfeature.FeatureProvider and openfeature.Tracker,
// evaluating flags via the Split Go SDK.
type SplitProvider struct {
	client *client.SplitClient
}

// NewProvider creates a SplitProvider with an existing Split client.
// Use this when you manage the Split client yourself (e.g. custom config, ready timeout).
func NewProvider(splitClient *client.SplitClient) (*SplitProvider, error) {
	if splitClient == nil {
		return nil, errNilSplitClient
	}
	return &SplitProvider{client: splitClient}, nil
}

// NewProviderWithAPIKey creates a SplitProvider using the given API key and default config.
func NewProviderWithAPIKey(apiKey string) (*SplitProvider, error) {
	cfg := conf.Default()

	factory, err := client.NewSplitFactory(apiKey, cfg)
	if err != nil {
		return nil, err
	}
	splitClient := factory.Client()
	if err := splitClient.BlockUntilReady(10); err != nil {
		return nil, err
	}
	return NewProvider(splitClient)
}

// Metadata returns the provider name for OpenFeature.
func (p *SplitProvider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{Name: "Split"}
}

// BooleanEvaluation evaluates a boolean flag.
func (p *SplitProvider) BooleanEvaluation(ctx context.Context, flag string, defaultValue bool, flatCtx openfeature.FlattenedContext) openfeature.BoolResolutionDetail {
	if noTargetingKey(flatCtx) {
		return openfeature.BoolResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailTargetingKeyMissing(),
		}
	}
	treatment, config := p.evaluateTreatmentWithConfig(flag, flatCtx)
	if noTreatment(treatment) {
		return openfeature.BoolResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailFlagNotFound(treatment),
		}
	}
	var value bool
	switch treatment {
	case "true", "on":
		value = true
	case "false", "off":
		value = false
	default:
		return openfeature.BoolResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailParseError(treatment),
		}
	}
	return openfeature.BoolResolutionDetail{
		Value:                    value,
		ProviderResolutionDetail: detailSuccess(treatment, config),
	}
}

// StringEvaluation evaluates a string flag.
func (p *SplitProvider) StringEvaluation(ctx context.Context, flag string, defaultValue string, flatCtx openfeature.FlattenedContext) openfeature.StringResolutionDetail {
	if noTargetingKey(flatCtx) {
		return openfeature.StringResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailTargetingKeyMissing(),
		}
	}
	treatment, config := p.evaluateTreatmentWithConfig(flag, flatCtx)
	if noTreatment(treatment) {
		return openfeature.StringResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailFlagNotFound(treatment),
		}
	}
	return openfeature.StringResolutionDetail{
		Value:                    treatment,
		ProviderResolutionDetail: detailSuccess(treatment, config),
	}
}

// FloatEvaluation evaluates a float64 flag.
func (p *SplitProvider) FloatEvaluation(ctx context.Context, flag string, defaultValue float64, flatCtx openfeature.FlattenedContext) openfeature.FloatResolutionDetail {
	if noTargetingKey(flatCtx) {
		return openfeature.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailTargetingKeyMissing(),
		}
	}
	treatment, config := p.evaluateTreatmentWithConfig(flag, flatCtx)
	if noTreatment(treatment) {
		return openfeature.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailFlagNotFound(treatment),
		}
	}
	value, parseErr := strconv.ParseFloat(treatment, 64)
	if parseErr != nil {
		return openfeature.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailParseError(treatment),
		}
	}
	return openfeature.FloatResolutionDetail{
		Value:                    value,
		ProviderResolutionDetail: detailSuccess(treatment, config),
	}
}

// IntEvaluation evaluates an int64 flag.
func (p *SplitProvider) IntEvaluation(ctx context.Context, flag string, defaultValue int64, flatCtx openfeature.FlattenedContext) openfeature.IntResolutionDetail {
	if noTargetingKey(flatCtx) {
		return openfeature.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailTargetingKeyMissing(),
		}
	}
	treatment, config := p.evaluateTreatmentWithConfig(flag, flatCtx)
	if noTreatment(treatment) {
		return openfeature.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailFlagNotFound(treatment),
		}
	}
	value, parseErr := strconv.ParseInt(treatment, 10, 64)
	if parseErr != nil {
		return openfeature.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailParseError(treatment),
		}
	}
	return openfeature.IntResolutionDetail{
		Value:                    value,
		ProviderResolutionDetail: detailSuccess(treatment, config),
	}
}

// ObjectEvaluation evaluates an object (map) flag.
func (p *SplitProvider) ObjectEvaluation(ctx context.Context, flag string, defaultValue any, flatCtx openfeature.FlattenedContext) openfeature.InterfaceResolutionDetail {
	if noTargetingKey(flatCtx) {
		return openfeature.InterfaceResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailTargetingKeyMissing(),
		}
	}
	treatment, config := p.evaluateTreatmentWithConfig(flag, flatCtx)
	if noTreatment(treatment) {
		return openfeature.InterfaceResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailFlagNotFound(treatment),
		}
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(treatment), &data); err != nil {
		return openfeature.InterfaceResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: detailParseError(treatment),
		}
	}
	return openfeature.InterfaceResolutionDetail{
		Value:                    data,
		ProviderResolutionDetail: detailSuccess(treatment, config),
	}
}

// Hooks returns no provider-specific hooks.
func (p *SplitProvider) Hooks() []openfeature.Hook {
	return nil
}

// Track sends a tracking event to Split. The evaluation context must contain
// a non-blank targetingKey and a trafficType (e.g. "user" or "account").
// Optional: use TrackingEventDetails for value and attributes.
func (p *SplitProvider) Track(ctx context.Context, eventName string, evalCtx openfeature.EvaluationContext, details openfeature.TrackingEventDetails) {
	key := evalCtx.TargetingKey()
	if key == "" {
		return
	}
	trafficType := getTrafficType(evalCtx)
	if trafficType == "" {
		return
	}
	value := details.Value()
	attrs := details.Attributes()
	if attrs == nil {
		attrs = make(map[string]any)
	}
	props := make(map[string]interface{}, len(attrs))
	for k, v := range attrs {
		props[k] = v
	}
	_ = p.client.Track(key, trafficType, eventName, value, props)
}

// evaluateTreatmentWithConfig returns treatment and optional config from Split.
// key and attributes are derived from flatCtx (targetingKey + rest as attributes).
func (p *SplitProvider) evaluateTreatmentWithConfig(flag string, flatCtx openfeature.FlattenedContext) (treatment string, config *string) {
	key, attrs := splitKeyAndAttributes(flatCtx)
	result := p.client.TreatmentWithConfig(key, flag, attrs)
	return result.Treatment, result.Config
}

func noTargetingKey(flatCtx openfeature.FlattenedContext) bool {
	v, ok := flatCtx[openfeature.TargetingKey]
	if !ok {
		return true
	}
	s, _ := v.(string)
	return s == ""
}

func noTreatment(treatment string) bool {
	return treatment == "" || treatment == "control"
}

func detailFlagNotFound(variant string) openfeature.ProviderResolutionDetail {
	return openfeature.ProviderResolutionDetail{
		ResolutionError: openfeature.NewFlagNotFoundResolutionError("flag not found"),
		Reason:          openfeature.DefaultReason,
		Variant:         variant,
	}
}

func detailParseError(variant string) openfeature.ProviderResolutionDetail {
	return openfeature.ProviderResolutionDetail{
		ResolutionError: openfeature.NewParseErrorResolutionError("error parsing treatment to requested type"),
		Reason:          openfeature.ErrorReason,
		Variant:         variant,
	}
}

func detailTargetingKeyMissing() openfeature.ProviderResolutionDetail {
	return openfeature.ProviderResolutionDetail{
		ResolutionError: openfeature.NewTargetingKeyMissingResolutionError("targeting key is required and missing"),
		Reason:          openfeature.ErrorReason,
		Variant:         "",
	}
}

func detailSuccess(variant string, config *string) openfeature.ProviderResolutionDetail {
	meta := openfeature.FlagMetadata(nil)
	if config != nil && *config != "" {
		meta = openfeature.FlagMetadata{flagMetadataConfigKey: *config}
	}
	return openfeature.ProviderResolutionDetail{
		Reason:       openfeature.TargetingMatchReason,
		Variant:      variant,
		FlagMetadata: meta,
	}
}

// splitKeyAndAttributes returns the targeting key and a map of attributes for Split.
// The targeting key is used as the Split key; other context fields become attributes.
func splitKeyAndAttributes(flatCtx openfeature.FlattenedContext) (key string, attributes map[string]interface{}) {
	keyVal, ok := flatCtx[openfeature.TargetingKey]
	if ok {
		if k, _ := keyVal.(string); k != "" {
			key = k
		}
	}
	attributes = make(map[string]interface{})
	for k, v := range flatCtx {
		if k == string(openfeature.TargetingKey) {
			continue
		}
		attributes[k] = v
	}
	return key, attributes
}

func getTrafficType(evalCtx openfeature.EvaluationContext) string {
	attrs := evalCtx.Attributes()
	if attrs == nil {
		return ""
	}
	v, ok := attrs["trafficType"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
