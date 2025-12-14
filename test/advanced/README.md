# Advanced Integration Test

Interactive test for configuration change event detection.

## What This Tests

**PROVIDER_CONFIGURATION_CHANGED Event Detection** - Validates that the provider correctly emits configuration change
events when flags are modified in the Split dashboard.

This test requires manual interaction: you must modify a flag in the Split dashboard while the test is running to
trigger the event.

All other cloud-only features (flag sets, targeting rules) are tested automatically in
the [integration test](../integration/) when `SPLIT_API_KEY` is provided.

## Prerequisites

- A Split account with SDK API key (server-side key)
- Any flag to modify during the test

Set `SPLIT_API_KEY` to your Split SDK API key.

## Provider Configuration

The test uses a 5-second monitoring interval for faster configuration change detection:

```go
provider, _ := split.New(apiKey,
    split.WithMonitoringInterval(5*time.Second),
)
```

## Running

```bash
cd test/advanced

# Run with API key
SPLIT_API_KEY=your-key go run main.go

# With debug logging
LOG_LEVEL=debug SPLIT_API_KEY=your-key go run main.go
```

## Test Flow

1. **Initialize Provider** - Connects to Split cloud with 5-second monitoring interval
2. **Wait for Configuration Change** - Waits up to 2 minutes for `PROVIDER_CONFIGURATION_CHANGED` event

    - Modify any flag in Split dashboard to trigger the event
    - Test automatically detects the change and reports success

3. **Event Summary** - Reports counts of all provider events received

## Notes

- **Monitoring Interval**: The provider polls every 5 seconds (configured via `WithMonitoringInterval`). Default is 30
  seconds, minimum is 5 seconds.
- **Configuration Change Detection**: While the Split SDK receives changes via SSE near-instantly, the provider polls
  for changes to emit `PROVIDER_CONFIGURATION_CHANGED` events.

## Learn More

- [Integration Test](../integration/) - Automated tests including flag sets and targeting (cloud mode)
- [Cloud Example](../../examples/cloud/) - Simple cloud mode example
