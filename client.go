package split

import "github.com/splitio/go-client/v6/splitio/client"

// Compile-time check that *client.SplitClient satisfies Client.
var _ Client = (*client.SplitClient)(nil)

// Client defines the Split SDK client methods used by this provider.
// This interface enables dependency injection for testing (mock generation via mockery).
// Method signatures match *client.SplitClient exactly (verified by compile-time check above).
type Client interface {
	// TreatmentWithConfig evaluates a single flag and returns the treatment with optional JSON config.
	TreatmentWithConfig(key interface{}, featureFlagName string, attributes map[string]interface{}) client.TreatmentResult

	// TreatmentsWithConfigByFlagSet evaluates all flags in a flag set.
	TreatmentsWithConfigByFlagSet(key interface{}, flagSet string, attributes map[string]interface{}) map[string]client.TreatmentResult

	// Track sends a tracking event to Split for analytics.
	Track(key string, trafficType string, eventType string, value interface{}, properties map[string]interface{}) error

	// BlockUntilReady blocks until the SDK is ready or the timeout (seconds) expires.
	BlockUntilReady(timer int) error

	// Destroy shuts down the SDK client and releases resources.
	Destroy()
}
