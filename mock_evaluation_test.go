package split

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/client"
	"github.com/splitio/go-client/v6/splitio/conf"
	"github.com/splitio/go-toolkit/v5/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock Test Infrastructure
// =============================================================================

// createTestProvider creates a provider initialized in localhost mode, ready for
// mock client swapping. Each top-level test should call this once and share the
// provider across subtests to minimize Split SDK factory goroutine count.
//
// The returned provider's client should be swapped with swapMockClient per subtest.
func createTestProvider(t *testing.T, logBuffer *strings.Builder) *Provider {
	t.Helper()

	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	var opts []Option
	opts = append(opts, WithSplitConfig(cfg))
	if logBuffer != nil {
		logger := slog.New(slog.NewTextHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
		opts = append(opts, WithLogger(logger))
	}

	provider, err := New("localhost", opts...)
	require.NoError(t, err)

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	require.NoError(t, err)

	t.Cleanup(func() {
		// Swap in a permissive mock for Destroy during shutdown
		permissive := &MockClient{}
		permissive.On("Destroy").Maybe()
		provider.mtx.Lock()
		provider.client = permissive
		provider.mtx.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = provider.ShutdownWithContext(ctx)
	})

	return provider
}

// swapMockClient creates a fresh MockClient for a subtest and installs it in the
// shared provider. NewMockClient(t) auto-registers AssertExpectations via t.Cleanup,
// so all expected calls are verified when the subtest completes.
func swapMockClient(t *testing.T, provider *Provider) *MockClient {
	t.Helper()
	mockClient := NewMockClient(t)
	provider.mtx.Lock()
	provider.client = mockClient
	provider.mtx.Unlock()
	return mockClient
}

// createCloudTestProvider creates a test provider that reports as cloud mode
// by overriding the splitConfig.OperationMode.
func createCloudTestProvider(t *testing.T, logBuffer *strings.Builder) *Provider {
	t.Helper()
	provider := createTestProvider(t, logBuffer)
	provider.splitConfig.OperationMode = ""
	return provider
}

// =============================================================================
// Track: SDK Call Argument Verification (Table-Driven)
// =============================================================================

func TestTrackMock_SDKArguments(t *testing.T) {
	provider := createTestProvider(t, nil)

	tests := []struct {
		name             string
		targetingKey     string
		evalAttrs        map[string]any
		eventName        string
		detailsValue     float64
		useNoMetricValue bool
		wantKey          string
		wantTrafficType  string
		wantEvent        string
		wantValue        interface{}
		wantProps        interface{} // mock.Anything or specific map
	}{
		{
			name:            "default traffic type",
			targetingKey:    "user-123",
			eventName:       "checkout",
			detailsValue:    42.0,
			wantKey:         "user-123",
			wantTrafficType: "user",
			wantEvent:       "checkout",
			wantValue:       42.0,
			wantProps:       mock.Anything,
		},
		{
			name:            "custom traffic type from eval context",
			targetingKey:    "acct-456",
			evalAttrs:       map[string]any{"trafficType": "account"},
			eventName:       "upgrade",
			detailsValue:    99.99,
			wantKey:         "acct-456",
			wantTrafficType: "account",
			wantEvent:       "upgrade",
			wantValue:       99.99,
			wantProps:       mock.Anything,
		},
		{
			name:             "WithoutMetricValue passes nil instead of 0",
			targetingKey:     "user-123",
			eventName:        "page_view",
			detailsValue:     0,
			useNoMetricValue: true,
			wantKey:          "user-123",
			wantTrafficType:  "user",
			wantEvent:        "page_view",
			wantValue:        nil,
			wantProps:        mock.Anything,
		},
		{
			name:            "metric value passed through",
			targetingKey:    "user-123",
			eventName:       "purchase",
			detailsValue:    149.99,
			wantKey:         "user-123",
			wantTrafficType: "user",
			wantEvent:       "purchase",
			wantValue:       149.99,
			wantProps:       mock.Anything,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := swapMockClient(t, provider)

			mockClient.EXPECT().
				Track(tt.wantKey, tt.wantTrafficType, tt.wantEvent, tt.wantValue, tt.wantProps).
				Once().
				Return(nil)

			ctx := context.Background()
			if tt.useNoMetricValue {
				ctx = WithoutMetricValue(ctx)
			}

			evalCtx := openfeature.NewEvaluationContext(tt.targetingKey, tt.evalAttrs)
			details := openfeature.NewTrackingEventDetails(tt.detailsValue)

			provider.Track(ctx, tt.eventName, evalCtx, details)
			// AssertExpectations is called automatically via t.Cleanup
		})
	}
}

func TestTrackMock_PropertiesPassthrough(t *testing.T) {
	provider := createTestProvider(t, nil)
	mockClient := swapMockClient(t, provider)

	expectedProps := map[string]interface{}{
		"currency":   "USD",
		"item_count": 3,
	}

	mockClient.EXPECT().
		Track("user-123", "user", "purchase", 99.0, expectedProps).
		Once().
		Return(nil)

	evalCtx := openfeature.NewEvaluationContext("user-123", nil)
	details := openfeature.NewTrackingEventDetails(99.0).
		Add("currency", "USD").
		Add("item_count", 3)

	provider.Track(context.Background(), "purchase", evalCtx, details)
}

// =============================================================================
// Track: SDK Error Handling
// =============================================================================

func TestTrackMock_SDKError_LoggedAtError(t *testing.T) {
	var logBuffer strings.Builder
	provider := createTestProvider(t, &logBuffer)
	mockClient := swapMockClient(t, provider)

	mockClient.EXPECT().
		Track("user-123", "user", "bad_event", 1.0, mock.Anything).
		Once().
		Return(errors.New("SDK validation error: event name too long"))

	evalCtx := openfeature.NewEvaluationContext("user-123", nil)
	details := openfeature.NewTrackingEventDetails(1.0)

	provider.Track(context.Background(), "bad_event", evalCtx, details)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "tracking event failed")
	assert.Contains(t, logOutput, "SDK validation error")
	assert.Contains(t, logOutput, "level=ERROR")
}

// =============================================================================
// Track: Precondition Guard Tests (SDK NOT called)
// =============================================================================

func TestTrackMock_NotCalled_WhenEmptyTargetingKey(t *testing.T) {
	provider := createTestProvider(t, nil)
	mockClient := swapMockClient(t, provider)

	evalCtx := openfeature.NewEvaluationContext("", nil)
	details := openfeature.NewTrackingEventDetails(1.0)

	provider.Track(context.Background(), "ignored_event", evalCtx, details)

	mockClient.AssertNotCalled(t, "Track", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTrackMock_NotCalled_WhenContextCanceled(t *testing.T) {
	provider := createTestProvider(t, nil)
	mockClient := swapMockClient(t, provider)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	evalCtx := openfeature.NewEvaluationContext("user-123", nil)
	details := openfeature.NewTrackingEventDetails(1.0)

	provider.Track(ctx, "canceled_event", evalCtx, details)

	mockClient.AssertNotCalled(t, "Track", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// =============================================================================
// ObjectEvaluation: Cloud Mode Flag Set (Table-Driven)
// =============================================================================

func TestObjectEvaluation_CloudMode(t *testing.T) {
	configJSON := `{"primary":"#000"}`
	provider := createCloudTestProvider(t, nil)

	tests := []struct {
		name          string
		mode          EvaluationMode
		flagParam     string
		setupMock     func(*MockClient)
		assertResult  func(*testing.T, openfeature.InterfaceResolutionDetail)
		wantFlagCount int
		wantNotFound  bool
	}{
		{
			name:      "default mode uses TreatmentsWithConfigByFlagSet",
			flagParam: "ui-features",
			setupMock: func(m *MockClient) {
				m.EXPECT().
					TreatmentsWithConfigByFlagSet("key", "ui-features", mock.Anything).
					Once().
					Return(map[string]client.TreatmentResult{
						"theme":  {Treatment: "dark", Config: &configJSON},
						"layout": {Treatment: "grid", Config: nil},
					})
			},
			wantFlagCount: 2,
			assertResult: func(t *testing.T, result openfeature.InterfaceResolutionDetail) {
				t.Helper()
				flagSet := result.Value.(FlagSetResult)
				assert.Equal(t, "dark", flagSet["theme"].Treatment)
				assert.Equal(t, map[string]any{"primary": "#000"}, flagSet["theme"].Config)
				assert.Equal(t, "grid", flagSet["layout"].Treatment)
				assert.Nil(t, flagSet["layout"].Config)
			},
		},
		{
			name:      "explicit set mode uses TreatmentsWithConfigByFlagSet",
			mode:      EvaluationModeSet,
			flagParam: "my-set",
			setupMock: func(m *MockClient) {
				m.EXPECT().
					TreatmentsWithConfigByFlagSet("key", "my-set", mock.Anything).
					Once().
					Return(map[string]client.TreatmentResult{
						"flag_a": {Treatment: "on", Config: nil},
					})
			},
			wantFlagCount: 1,
		},
		{
			name:      "individual mode uses TreatmentWithConfig",
			mode:      EvaluationModeIndividual,
			flagParam: "single-flag",
			setupMock: func(m *MockClient) {
				m.EXPECT().
					TreatmentWithConfig("key", "single-flag", mock.Anything).
					Once().
					Return(client.TreatmentResult{Treatment: "on", Config: &configJSON})
			},
			wantFlagCount: 1,
			assertResult: func(t *testing.T, result openfeature.InterfaceResolutionDetail) {
				t.Helper()
				flagSet := result.Value.(FlagSetResult)
				assert.Equal(t, "on", flagSet["single-flag"].Treatment)
				assert.Equal(t, map[string]any{"primary": "#000"}, flagSet["single-flag"].Config)
			},
		},
		{
			name:      "empty flag set returns FLAG_NOT_FOUND",
			flagParam: "nonexistent-set",
			setupMock: func(m *MockClient) {
				m.EXPECT().
					TreatmentsWithConfigByFlagSet("key", "nonexistent-set", mock.Anything).
					Once().
					Return(map[string]client.TreatmentResult{})
			},
			wantNotFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := swapMockClient(t, provider)
			tt.setupMock(mockClient)

			ctx := context.Background()
			if tt.mode != "" {
				ctx = WithEvaluationMode(ctx, tt.mode)
			}

			flatCtx := openfeature.FlattenedContext{
				openfeature.TargetingKey: "key",
			}

			result := provider.ObjectEvaluation(ctx, tt.flagParam, FlagSetResult{}, flatCtx)

			if tt.wantNotFound {
				assert.Contains(t, result.ResolutionError.Error(), string(openfeature.FlagNotFoundCode))
				return
			}

			flagSet, ok := result.Value.(FlagSetResult)
			require.True(t, ok, "Value should be FlagSetResult")
			assert.Len(t, flagSet, tt.wantFlagCount)
			assert.Equal(t, openfeature.TargetingMatchReason, result.Reason)

			if tt.assertResult != nil {
				tt.assertResult(t, result)
			}
		})
	}
}

// =============================================================================
// ObjectEvaluation: Cloud Mode JSON Config Parsing
// =============================================================================

func TestObjectEvaluation_CloudMode_ConfigParsesAllJSONTypes(t *testing.T) {
	provider := createCloudTestProvider(t, nil)
	mockClient := swapMockClient(t, provider)

	arrayConfig := `[1,2,3]`
	stringConfig := `"hello"`
	numberConfig := `42`
	boolConfig := `true`
	nullConfig := `null`

	mockClient.EXPECT().
		TreatmentsWithConfigByFlagSet("key", "json-types", mock.Anything).
		Once().
		Return(map[string]client.TreatmentResult{
			"array_flag":  {Treatment: "on", Config: &arrayConfig},
			"string_flag": {Treatment: "on", Config: &stringConfig},
			"number_flag": {Treatment: "on", Config: &numberConfig},
			"bool_flag":   {Treatment: "on", Config: &boolConfig},
			"null_flag":   {Treatment: "on", Config: &nullConfig},
		})

	flatCtx := openfeature.FlattenedContext{openfeature.TargetingKey: "key"}
	result := provider.ObjectEvaluation(context.Background(), "json-types", nil, flatCtx)

	flagSet, ok := result.Value.(FlagSetResult)
	require.True(t, ok)
	assert.Len(t, flagSet, 5)

	assert.Equal(t, []any{float64(1), float64(2), float64(3)}, flagSet["array_flag"].Config)
	assert.Equal(t, "hello", flagSet["string_flag"].Config)
	assert.Equal(t, float64(42), flagSet["number_flag"].Config)
	assert.Equal(t, true, flagSet["bool_flag"].Config)
	assert.Nil(t, flagSet["null_flag"].Config)
}

// =============================================================================
// Scalar Evaluation: Mock-Based (Table-Driven)
// =============================================================================

func TestScalarEvaluation_MockClient(t *testing.T) {
	configJSON := `{"rollout":"gradual"}`
	provider := createTestProvider(t, nil)

	tests := []struct {
		name      string
		mockSetup func(*MockClient)
		evaluate  func(*Provider) interface{}
		wantValue interface{}
		wantMeta  map[string]any // expected FlagMetadata["value"], nil if none
	}{
		{
			name: "BooleanEvaluation/on returns true with config",
			mockSetup: func(m *MockClient) {
				m.EXPECT().
					TreatmentWithConfig("user-1", "feature_x", mock.Anything).
					Once().
					Return(client.TreatmentResult{Treatment: "on", Config: &configJSON})
			},
			evaluate: func(p *Provider) interface{} {
				flatCtx := openfeature.FlattenedContext{openfeature.TargetingKey: "user-1"}
				return p.BooleanEvaluation(context.Background(), "feature_x", false, flatCtx)
			},
			wantValue: true,
			wantMeta:  map[string]any{"rollout": "gradual"},
		},
		{
			name: "StringEvaluation/returns treatment string",
			mockSetup: func(m *MockClient) {
				m.EXPECT().
					TreatmentWithConfig("user-1", "color_flag", mock.Anything).
					Once().
					Return(client.TreatmentResult{Treatment: "blue", Config: nil})
			},
			evaluate: func(p *Provider) interface{} {
				flatCtx := openfeature.FlattenedContext{openfeature.TargetingKey: "user-1"}
				return p.StringEvaluation(context.Background(), "color_flag", "red", flatCtx)
			},
			wantValue: "blue",
		},
		{
			name: "IntEvaluation/parses treatment as int",
			mockSetup: func(m *MockClient) {
				m.EXPECT().
					TreatmentWithConfig("user-1", "retry_count", mock.Anything).
					Once().
					Return(client.TreatmentResult{Treatment: "5", Config: nil})
			},
			evaluate: func(p *Provider) interface{} {
				flatCtx := openfeature.FlattenedContext{openfeature.TargetingKey: "user-1"}
				return p.IntEvaluation(context.Background(), "retry_count", 3, flatCtx)
			},
			wantValue: int64(5),
		},
		{
			name: "FloatEvaluation/parses treatment as float",
			mockSetup: func(m *MockClient) {
				m.EXPECT().
					TreatmentWithConfig("user-1", "discount", mock.Anything).
					Once().
					Return(client.TreatmentResult{Treatment: "0.15", Config: nil})
			},
			evaluate: func(p *Provider) interface{} {
				flatCtx := openfeature.FlattenedContext{openfeature.TargetingKey: "user-1"}
				return p.FloatEvaluation(context.Background(), "discount", 0.0, flatCtx)
			},
			wantValue: 0.15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := swapMockClient(t, provider)
			tt.mockSetup(mockClient)

			result := tt.evaluate(provider)

			// Assert value based on result type
			switch r := result.(type) {
			case openfeature.BoolResolutionDetail:
				assert.Equal(t, tt.wantValue, r.Value)
				assert.Equal(t, openfeature.TargetingMatchReason, r.Reason)
				if tt.wantMeta != nil {
					assert.Equal(t, tt.wantMeta, r.FlagMetadata["value"])
				}
			case openfeature.StringResolutionDetail:
				assert.Equal(t, tt.wantValue, r.Value)
				assert.Equal(t, openfeature.TargetingMatchReason, r.Reason)
			case openfeature.IntResolutionDetail:
				assert.Equal(t, tt.wantValue, r.Value)
				assert.Equal(t, openfeature.TargetingMatchReason, r.Reason)
			case openfeature.FloatResolutionDetail:
				assert.Equal(t, tt.wantValue, r.Value)
				assert.Equal(t, openfeature.TargetingMatchReason, r.Reason)
			default:
				t.Fatalf("unexpected result type: %T", result)
			}
		})
	}
}

// =============================================================================
// Attributes Passthrough: Mock-Based (Table-Driven)
// =============================================================================

func TestEvaluation_MockClient_AttributeFiltering(t *testing.T) {
	provider := createTestProvider(t, nil)

	tests := []struct {
		name      string
		flatCtx   openfeature.FlattenedContext
		wantAttrs map[string]interface{}
	}{
		{
			name: "passes user attributes, excludes targetingKey",
			flatCtx: openfeature.FlattenedContext{
				openfeature.TargetingKey: "user-1",
				"email":                  "user@test.com",
				"plan":                   "premium",
			},
			wantAttrs: map[string]interface{}{
				"email": "user@test.com",
				"plan":  "premium",
			},
		},
		{
			name: "excludes trafficType from attributes",
			flatCtx: openfeature.FlattenedContext{
				openfeature.TargetingKey: "user-1",
				TrafficTypeKey:           "account",
				"email":                  "user@test.com",
			},
			wantAttrs: map[string]interface{}{
				"email": "user@test.com",
			},
		},
		{
			name: "no extra attributes passes empty map",
			flatCtx: openfeature.FlattenedContext{
				openfeature.TargetingKey: "user-1",
			},
			wantAttrs: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := swapMockClient(t, provider)

			mockClient.EXPECT().
				TreatmentWithConfig("user-1", "feature", tt.wantAttrs).
				Once().
				Return(client.TreatmentResult{Treatment: "on", Config: nil})

			result := provider.BooleanEvaluation(context.Background(), "feature", false, tt.flatCtx)
			assert.True(t, result.Value)
		})
	}
}
