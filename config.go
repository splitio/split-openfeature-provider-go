package split

import "github.com/splitio/go-client/v6/splitio/conf"

// TestConfig returns an optimized Split SDK configuration for tests and examples.
// This configuration minimizes timeouts, queue sizes, and sync intervals for faster
// execution while maintaining full functionality.
//
// Optimizations applied:
//   - BlockUntilReady: 5 seconds (faster initialization timeout)
//   - HTTPTimeout: 5 seconds (faster network failure detection)
//   - ImpressionsMode: debug (sends all impressions, not batched)
//   - Queue sizes: Reduced to 100 (faster event/impression flushing)
//   - Bulk sizes: Reduced to 100 (smaller batches, faster submission)
//   - Sync intervals: Set to minimums (faster updates)
//
// Usage:
//
//	cfg := split.TestConfig()
//	cfg.SplitFile = "./split.yaml"  // For localhost mode
//	provider, err := split.New(apiKey, split.WithSplitConfig(cfg))
func TestConfig() *conf.SplitSdkConfig {
	cfg := conf.Default()

	// Faster initialization timeout
	cfg.BlockUntilReady = 5

	// Faster network failure detection
	cfg.Advanced.HTTPTimeout = 5

	// Use debug mode for impression tracking (sends all impressions, 60s sync)
	// Default "optimized" batches impressions which can delay visibility
	cfg.ImpressionsMode = "debug"

	// Smaller queues for faster flushing in tests
	cfg.Advanced.EventsQueueSize = 100
	cfg.Advanced.ImpressionsQueueSize = 100

	// Smaller batches for faster submission
	cfg.Advanced.EventsBulkSize = 100
	cfg.Advanced.ImpressionsBulkSize = 100

	// Minimum sync intervals for faster updates
	cfg.TaskPeriods.SplitSync = 5       // minimum: 5s
	cfg.TaskPeriods.SegmentSync = 30    // minimum: 30s
	cfg.TaskPeriods.ImpressionSync = 60 // minimum: 60s (debug mode)
	cfg.TaskPeriods.EventsSync = 1      // minimum: 1s
	cfg.TaskPeriods.TelemetrySync = 60  // reduced from 3600s

	return cfg
}
