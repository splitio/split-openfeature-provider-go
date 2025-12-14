package split

import "time"

const (
	// SDK Timeouts

	// defaultSDKTimeout is the default timeout in seconds for Split SDK operations.
	// Used for both BlockUntilReady (initialization) and Destroy (shutdown).
	defaultSDKTimeout = 10

	// defaultInitTimeout is the default timeout for provider initialization when no BlockUntilReady is configured.
	// Provides 5 seconds buffer beyond the defaultSDKTimeout (10s SDK + 5s buffer = 15s total).
	defaultInitTimeout = 15 * time.Second

	// initTimeoutBuffer is added to BlockUntilReady to ensure initialization has time to complete gracefully.
	initTimeoutBuffer = 5 * time.Second

	// defaultShutdownTimeout is the default timeout for provider shutdown operations.
	// Allows time for monitoring goroutine cleanup, SDK destroy, and channel closes.
	defaultShutdownTimeout = 30 * time.Second

	// Event Handling

	// eventChannelBuffer is the buffer size for the provider's event channel.
	// Events are sent asynchronously to OpenFeature SDK handlers. Power of 2 for
	// memory allocator efficiency. Overflow events are dropped (logged as warnings).
	eventChannelBuffer = 128

	// Monitoring

	// defaultMonitoringInterval is the default interval for checking split definition changes.
	defaultMonitoringInterval = 30 * time.Second

	// minMonitoringInterval is the minimum allowed monitoring interval.
	minMonitoringInterval = 5 * time.Second

	// Atomic States

	// shutdownStateActive indicates the provider has been shut down (atomic flag = 1).
	shutdownStateActive = 1

	// shutdownStateInactive indicates the provider is active (atomic flag = 0).
	shutdownStateInactive = 0

	// Split SDK Constants

	// controlTreatment is the treatment returned by Split SDK when a flag doesn't exist
	// or evaluation fails. Used to detect missing flags and return defaults.
	controlTreatment = "control"

	// OpenFeature Context Keys

	// TrafficTypeKey is the evaluation context attribute key for Split traffic type.
	// Used by Track() to categorize events. Not used for flag evaluations
	// (traffic type is configured per flag in Split dashboard).
	TrafficTypeKey = "trafficType"

	// DefaultTrafficType is the default traffic type used when not specified in context.
	// "user" is the most common traffic type for user-based targeting and tracking.
	DefaultTrafficType = "user"
)
