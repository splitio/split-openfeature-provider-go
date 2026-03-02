package split

import "time"

const (
	// SDK Timeouts

	// defaultSDKTimeout is the BlockUntilReady fallback when not configured.
	defaultSDKTimeout = 10

	// defaultInitTimeout is the default timeout for provider initialization when no BlockUntilReady is configured.
	// Provides 5 seconds buffer beyond the defaultSDKTimeout (10s SDK + 5s buffer = 15s total).
	defaultInitTimeout = 15 * time.Second

	// initTimeoutBuffer is added to BlockUntilReady to ensure initialization has time to complete.
	initTimeoutBuffer = 5 * time.Second

	// defaultShutdownTimeout is the default timeout for provider shutdown operations.
	// Allows time for monitoring goroutine cleanup, SDK destroy, and channel closes.
	defaultShutdownTimeout = 30 * time.Second

	// Event Handling

	// eventChannelBuffer is the buffer size for the provider's event channel.
	// Provides headroom for burst events. Overflow events are dropped (logged as warnings).
	eventChannelBuffer = 128

	// Monitoring

	defaultMonitoringInterval = 30 * time.Second
	minMonitoringInterval     = 5 * time.Second

	// Atomic States

	shutdownStateActive   = 1
	shutdownStateInactive = 0

	// Split SDK Constants

	// controlTreatment is the treatment returned by Split SDK when a flag doesn't exist
	// or evaluation fails. Used to detect missing flags and return defaults.
	controlTreatment = "control"

	// treatmentOn is the conventional Split treatment for boolean "true".
	treatmentOn = "on"

	// treatmentOff is the conventional Split treatment for boolean "false".
	treatmentOff = "off"

	// OpenFeature Context Keys

	// TrafficTypeKey is the evaluation context attribute key for Split traffic type.
	// Used by Track() to categorize events. Not used for flag evaluations
	// (traffic type is configured per flag in Split dashboard).
	TrafficTypeKey = "trafficType"

	// DefaultTrafficType is the default traffic type when not specified in context.
	DefaultTrafficType = "user"
)
