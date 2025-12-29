<div align="center">

<img src="docs/images/of_banner.png" alt="OpenFeature Banner">

# Split OpenFeature Go Provider

[![Report Card](https://goreportcard.com/badge/github.com/splitio/split-openfeature-provider-go?style=for-the-badge&logo=go)](https://goreportcard.com/report/github.com/splitio/split-openfeature-provider-go)
[![Coverage](https://img.shields.io/badge/coverage-75.3%25-brightgreen?style=for-the-badge&logo=go)](https://github.com/splitio/split-openfeature-provider-go)
[![Reference](https://img.shields.io/badge/reference-docs-007d9c?style=for-the-badge&logo=go&logoColor=white)](https://pkg.go.dev/github.com/splitio/split-openfeature-provider-go/v2)

**OpenFeature Go Provider for Split.io**

[Installation](#installation) • [Usage](#usage) • [Examples](#examples) • [API](#api) • [Contributing](#contributing)

</div>

---

## Overview

OpenFeature provider for Split.io enabling feature flag evaluation through the OpenFeature SDK with support for
attribute-based targeting and flag metadata (JSON configurations attached to treatments).

## Features

- All OpenFeature flag types (boolean, string, number, object)
- Event tracking for experimentation and analytics
- Attribute-based targeting and flag metadata
- Configuration change detection via background monitoring
- Thread-safe concurrent evaluations
- Structured logging via `slog`

## Installation

```bash
go get github.com/splitio/split-openfeature-provider-go/v2
go get github.com/open-feature/go-sdk
go get github.com/splitio/go-client/v6
```

## Usage

### Basic Setup

```go
import (
	"context"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/split-openfeature-provider-go/v2/split"
)

provider, err := split.New("YOUR_API_KEY")
if err != nil {
	log.Fatal(err)
}

defer func() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := openfeature.ShutdownWithContext(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}()

ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

if err := openfeature.SetProviderWithContextAndWait(ctx, provider); err != nil {
	log.Fatal(err)
}

client := openfeature.NewClient("my-app")
```

### Advanced Setup

```go
import "github.com/splitio/go-client/v6/splitio/conf"

cfg := conf.Default()
cfg.BlockUntilReady = 15 // Default is 10 seconds

provider, err := split.New("YOUR_API_KEY", split.WithSplitConfig(cfg))
```

See [examples](./examples/) for complete configuration patterns including logging setup.

### Server-Side Evaluation Pattern

In server-side SDKs, create client once at startup, then evaluate per-request with transaction-specific context:

```go
// Application startup - create client once
client := openfeature.NewClient("my-app")

// Per-request handler
func handleRequest(w http.ResponseWriter, r *http.Request) {
	// Create evaluation context with targeting key and attributes
	evalCtx := openfeature.NewEvaluationContext("user-123", map[string]any{
		"email": "user@example.com",
		"plan":  "premium",
	})

	// Option 1: Pass evaluation context directly to each call
	enabled, _ := client.BooleanValue(r.Context(), "new-feature", false, evalCtx)

	// Option 2: Use transaction context propagation (set once, use throughout request)
	ctx := openfeature.WithTransactionContext(r.Context(), evalCtx)
	enabled, _ = client.BooleanValue(ctx, "new-feature", false, openfeature.EvaluationContext{})
	theme, _ := client.StringValue(ctx, "ui-theme", "light", openfeature.EvaluationContext{})
}
```

**Required:** Targeting key in evaluation context.

**Transaction context:** Use `openfeature.WithTransactionContext()` to embed evaluation context in `context.Context`
once, then reuse across multiple evaluations.

### Domain-Specific Providers

Use named providers for multi-tenant or service-isolated configurations:

```go
defer func() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	openfeature.ShutdownWithContext(ctx)
}()

ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

tenant1Provider, _ := split.New("TENANT_1_API_KEY")
openfeature.SetNamedProviderWithContextAndWait(ctx, "tenant-1", tenant1Provider)

tenant2Provider, _ := split.New("TENANT_2_API_KEY")
openfeature.SetNamedProviderWithContextAndWait(ctx, "tenant-2", tenant2Provider)

// Create clients for each named provider domain
client1 := openfeature.NewClient("tenant-1")
client2 := openfeature.NewClient("tenant-2")
```

### Lifecycle Management

#### Context-Aware Initialization

The provider supports context-aware initialization with timeout and cancellation:

```go
// Initialization with context (recommended)
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

if err := openfeature.SetProviderWithContextAndWait(ctx, provider); err != nil {
	log.Fatal(err)
}
```

**Key Behaviors:**

- Respects context deadline (returns error if timeout exceeded)
- Cancellable via context cancellation
- Idempotent - safe to call multiple times (fast path if already initialized)
- Thread-safe - concurrent Init calls use singleflight (only one initialization happens)

#### Graceful Shutdown with Timeout

Shutdown is a graceful best-effort operation that returns an error if cleanup doesn't complete within the context
deadline:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := openfeature.ShutdownWithContext(ctx); err != nil {
	// Error means cleanup timed out, but provider is still logically shut down
	log.Printf("Shutdown timeout: %v (cleanup continuing in background)", err)
}
```

**Shutdown Behavior:**

The provider is **immediately** marked as shut down (all new operations fail with `PROVIDER_NOT_READY`), then cleanup
happens within the context deadline:

1. **Within Deadline:** Complete cleanup, return `nil`
2. **After Deadline:** Log warnings, return `ctx.Err()` (context.DeadlineExceeded), continue cleanup in background

**Return Values:**

- `nil` - shutdown completed successfully within timeout
- `context.DeadlineExceeded` - cleanup timed out (provider still logically shut down)
- `context.Canceled` - context was cancelled (provider still logically shut down)

**Cleanup Timing:**

- Event channel close: Immediate
- Monitoring goroutine: Up to 30 seconds to terminate
- Split SDK Destroy: Up to 1 hour in streaming mode (known SDK limitation)

**Recommended Timeout:** 30 seconds minimum to allow monitoring goroutine to exit cleanly.

**Important:** Even when an error is returned, the provider is logically shut down:

- Provider state is atomically set to "shut down" immediately
- All new operations (Init, evaluations) will fail with PROVIDER_NOT_READY
- Background cleanup continues safely even after error is returned

#### Provider Reusability

**Important:** Once shut down, a provider instance cannot be reused. Attempting to initialize after shutdown returns an
error:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
_ = provider.ShutdownWithContext(ctx)

// This will fail with error: "cannot initialize provider after shutdown"
initCtx, initCancel := context.WithTimeout(context.Background(), 15*time.Second)
defer initCancel()
err := openfeature.SetProviderWithContextAndWait(initCtx, provider)
```

To use a provider again after shutdown, create a new instance:

```go
newProvider, _ := split.New("YOUR_API_KEY")
```

#### Thread Safety Guarantees

The provider is fully thread-safe with the following guarantees:

- **Concurrent Evaluations:** Multiple goroutines can safely call evaluation methods simultaneously
- **Evaluation During Shutdown:** In-flight evaluations complete safely before client destruction
- **Concurrent Init Calls:** Multiple Init calls use singleflight - only one initialization happens
- **Status Consistency:** Status() and Metrics() return consistent atomic state even during transitions
- **Factory Access:** Factory() can be called safely during concurrent operations

### Provider Status

The provider follows OpenFeature's state lifecycle with the following states:

| State        | When It Occurs                                  | Evaluations Behavior              | Status() Returns   |
|--------------|-------------------------------------------------|-----------------------------------|--------------------|
| **NotReady** | After `New()`, before `Init()` completes        | Return `PROVIDER_NOT_READY` error | `of.NotReadyState` |
| **Ready**    | After successful `Init()` / `BlockUntilReady()` | Execute normally with Split SDK   | `of.ReadyState`    |
| **NotReady** | After `Shutdown()` called                       | Return `PROVIDER_NOT_READY` error | `of.NotReadyState` |

**State Transitions:**

```
New() → NotReady
  ↓
Init() → Ready (if SDK becomes ready)
  ↓
  └─→ NotReady (if Shutdown() called)
        ↓
      [Terminal State - Cannot re-initialize]
```

**Important Notes:**

- Once `Shutdown()` is called, the provider **cannot be re-initialized** - create a new instance instead
- `Init()` can fail due to timeout, invalid API key, or shutdown during initialization
- State transitions emit OpenFeature events (`PROVIDER_READY`, `PROVIDER_ERROR`, `PROVIDER_CONFIGURATION_CHANGED`)

**Staleness Detection Limitation:**
The Split SDK's `IsReady()` method only indicates initial readiness and does not change when network connectivity is
lost. The SDK handles connectivity issues internally (switching between streaming and polling modes) but does not expose
this state. As a result, `PROVIDER_STALE` events are not emitted. When connectivity is lost, the SDK continues serving
cached data silently. See [CONTRIBUTING.md](CONTRIBUTING.md) for details on this limitation.

**Check provider readiness:**

```go
// Check via client (works for both default and named providers)
client := openfeature.NewClient("my-app") // or named domain like "tenant-1"
if client.State() == openfeature.ReadyState {
	// Provider ready for evaluations
}

// Get provider metadata
metadata := client.Metadata()
domain := metadata.Domain() // Client's domain name
```

**For diagnostics and monitoring:**

```go
// Provider-specific health metrics
metrics := provider.Metrics()
// Returns map with: provider, initialized, status, splits_count, ready
```

### Known Limitations

**Context Cancellation During Evaluation**

Evaluation methods (`BooleanValue`, `StringValue`, etc.) accept a `context.Context` parameter but **cannot cancel
in-flight evaluations**. This is because the underlying Split SDK's `TreatmentWithConfig()` method does not support
context cancellation.

**Impact:**

- Context cancellation/timeout is only checked **before** calling the Split SDK
- Once evaluation starts, it runs to completion even if context expires
- In localhost mode: evaluations are fast (~microseconds), low risk
- In cloud mode: evaluations read from cache, typically <1ms, but network issues could cause delays

**Affected operations:**

- ✅ `InitWithContext` - respects context cancellation
- ✅ `ShutdownWithContext` - respects context timeout
- ❌ Flag evaluations - cannot cancel once started

**Workarounds:**

```go
// Option 1: Use HTTP-level timeouts (recommended)
cfg := conf.Default()
cfg.Advanced.HTTPTimeout = 5 * time.Second

// Option 2: Set aggressive evaluation context timeout
ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()
// Note: timeout only applies BEFORE evaluation starts
value, err := client.BooleanValue(ctx, "flag", false, evalCtx)
```

**Split SDK Destroy() Blocking (Streaming Mode)**

In cloud/streaming mode, the Split SDK's `Destroy()` method can block for up to 1 hour due to SSE connection handling.
This is a known Split SDK limitation tracked
in [splitio/go-client#243](https://github.com/splitio/go-client/issues/243).

**Impact:** During shutdown, cleanup may continue in background if context timeout expires. The provider is logically
shut down immediately (all new operations return defaults), only cleanup may be delayed.

**Mitigation:** Use appropriate shutdown timeout (30s recommended):

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
openfeature.ShutdownWithContext(ctx)
```

## Examples

Complete working examples with detailed code:

- **[localhost/](./examples/localhost/)** - Local development mode (YAML file, no API key required)
- **[cloud/](./examples/cloud/)** - Cloud mode with streaming updates and all flag types
- **[test/integration/](./test/integration/)** - Comprehensive integration test suite

Run examples:

```bash
task example-localhost    # No API key needed
task example-cloud        # Requires SPLIT_API_KEY
task test-integration     # Full integration tests
```

## API

### Flag Evaluation

All methods require targeting key in evaluation context:

```go
ctx := context.Background()
evalCtx := openfeature.NewEvaluationContext("user-123", map[string]any{
	"email": "user@example.com",
	"plan":  "premium",
})

// Boolean
enabled, err := client.BooleanValue(ctx, "new-feature", false, evalCtx)

// String
theme, err := client.StringValue(ctx, "ui-theme", "light", evalCtx)

// Number
maxRetries, err := client.IntValue(ctx, "max-retries", 3, evalCtx)
discount, err := client.FloatValue(ctx, "discount-rate", 0.0, evalCtx)

// Object
result, err := client.ObjectValue(ctx, "flag-key", split.FlagSetResult{}, evalCtx)
```

### Object Evaluation - Mode-Specific Behavior

Object evaluation returns `split.FlagSetResult` in both modes:

```go
type FlagResult struct {
	Config    any    // Parsed JSON config, or nil
	Treatment string // Split treatment name (e.g., "on", "off", "v1")
}

type FlagSetResult map[string]FlagResult
```

**Cloud Mode:**

```go
result, _ := client.ObjectValue(ctx, "my-flag-set", split.FlagSetResult{}, evalCtx)
flags := result.(split.FlagSetResult)
for name, flag := range flags {
	fmt.Printf("%s: %s\n", name, flag.Treatment)
}
```

**Important:** Cloud mode ONLY supports flag sets (using Split SDK's `TreatmentsWithConfigByFlagSet`). Single flag
evaluation is not available in cloud mode.

**Localhost Mode:**

```go
result, _ := client.ObjectValue(ctx, "single-flag", split.FlagSetResult{}, evalCtx)
flags := result.(split.FlagSetResult)
flag := flags["single-flag"]
fmt.Println(flag.Treatment, flag.Config)
```

**Note:** Flag sets are NOT supported in localhost mode - only individual flags

### Extracting Configuration Metadata

All `*ValueDetails` methods return evaluation metadata including flag metadata:

```go
details, err := client.StringValueDetails(ctx, "ui-theme", "light", evalCtx)

// Standard fields
value := details.Value       // Evaluated value: "dark" (for strings, same as treatment)
treatment := details.Variant // Split treatment name: "dark", "light", etc.
reason := details.Reason     // TARGETING_MATCH, DEFAULT, ERROR

// Extract flag metadata (configurations attached to treatments)
// All config types are wrapped in FlagMetadata["value"] for consistency
if configValue, ok := details.FlagMetadata["value"]; ok {
	// Object config: {"bgColor": "#000", "fontSize": 14}
	if configMap, ok := configValue.(map[string]any); ok {
		bgColor := configMap["bgColor"]
		fontSize := configMap["fontSize"]
	}
	// Primitive config: 42
	if num, ok := configValue.(float64); ok {
		// Use primitive value
	}
	// Array config: ["a", "b", "c"]
	if arr, ok := configValue.([]any); ok {
		// Use array
	}
}
```

### Evaluation Reasons

| Reason            | Description                                                               |
|-------------------|---------------------------------------------------------------------------|
| `TARGETING_MATCH` | Flag successfully evaluated                                               |
| `DEFAULT`         | Flag not found, returned default value                                    |
| `ERROR`           | Evaluation error (missing targeting key, provider not ready, parse error) |

### Error Codes

Provider implements OpenFeature error codes. All errors return default value:

- `PROVIDER_NOT_READY` - Provider not initialized
- `FLAG_NOT_FOUND` - Flag doesn't exist in Split
- `PARSE_ERROR` - Treatment can't parse to requested type
- `TARGETING_KEY_MISSING` - No targeting key in context
- `INVALID_CONTEXT` - Malformed evaluation context
- `GENERAL` - Context canceled/timeout or other errors

### Default Value Behavior

OpenFeature's design philosophy: **evaluations never return Go errors**. Instead, they return the default value you
provide with resolution details indicating what happened.

**When Split SDK Returns "control" Treatment:**

The Split SDK returns a special `"control"` treatment to indicate evaluation failure. Our provider translates this to
OpenFeature's default value pattern:

| Condition                  | Split SDK Returns | Caller Receives | Resolution Details                                |
|----------------------------|-------------------|-----------------|---------------------------------------------------|
| Flag doesn't exist         | `"control"`       | Default value   | `Reason: DEFAULT`<br>`Error: FLAG_NOT_FOUND`      |
| Provider not initialized   | `"control"`       | Default value   | `Reason: ERROR`<br>`Error: PROVIDER_NOT_READY`    |
| Provider shut down         | `"control"`       | Default value   | `Reason: ERROR`<br>`Error: PROVIDER_NOT_READY`    |
| Targeting key missing      | `"control"`       | Default value   | `Reason: ERROR`<br>`Error: TARGETING_KEY_MISSING` |
| Context canceled           | `"control"`       | Default value   | `Reason: ERROR`<br>`Error: GENERAL`               |
| Network error (cloud mode) | `"control"`       | Default value   | `Reason: DEFAULT`<br>`Error: FLAG_NOT_FOUND`      |

**Example:**

```go
// Flag doesn't exist in Split
enabled, err := client.BooleanValue(ctx, "nonexistent-flag", false, evalCtx)
// Result:
// - enabled = false (your default value)
// - err = nil (OpenFeature doesn't return errors)

// To check what happened, use *ValueDetails methods:
details, err := client.BooleanValueDetails(ctx, "nonexistent-flag", false, evalCtx)
// - details.Value = false
// - details.Reason = of.DefaultReason
// - details.ErrorCode = of.FlagNotFoundCode
// - details.ErrorMessage = "flag not found"
```

**Key Points:**

- Your application continues running normally with safe default values
- No panic, no nil pointers, no error handling required for normal operation
- Use `*ValueDetails` methods when you need to distinguish between success and fallback
- This design enables graceful degradation during outages or misconfigurations

### Event Tracking

Track user actions for experimentation and analytics:

```go
evalCtx := openfeature.NewEvaluationContext("user-123", nil)

// Basic tracking with value
details := openfeature.NewTrackingEventDetails(99.99)
client.Track(ctx, "purchase_completed", evalCtx, details)

// Tracking with custom traffic type
evalCtxAccount := openfeature.NewEvaluationContext("account-456", map[string]any{
	"trafficType": "account", // Optional, defaults to "user"
})
client.Track(ctx, "subscription_created", evalCtxAccount, details)

// Tracking with properties
purchaseDetails := openfeature.NewTrackingEventDetails(149.99).
	Add("currency", "USD").
	Add("item_count", 3).
	Add("category", "electronics")
client.Track(ctx, "purchase", evalCtx, purchaseDetails)
```

**Supported Property Types:**

The Split SDK accepts the following property value types:

| Type                       | Supported | Example                    |
|----------------------------|-----------|----------------------------|
| `string`                   | ✅         | `Add("currency", "USD")`   |
| `bool`                     | ✅         | `Add("is_premium", true)`  |
| `int`, `int32`, `int64`    | ✅         | `Add("item_count", 3)`     |
| `uint`, `uint32`, `uint64` | ✅         | `Add("quantity", uint(5))` |
| `float32`, `float64`       | ✅         | `Add("price", 99.99)`      |
| `nil`                      | ✅         | `Add("optional", nil)`     |
| Arrays, maps, structs      | ❌         | Silently set to `nil`      |

**⚠️ Important:** Unsupported types (arrays, maps, nested objects) are **silently set to `nil`** by the Split SDK - no
error is returned. Always use primitive types for event properties.

**Parameters:**

- `trackingEventName`: Event name (e.g., "checkout", "signup")
- `evaluationContext`: Contains targeting key and optional `trafficType` attribute
- `details`: Event value and custom properties

**Traffic Type:**

- Defaults to `"user"` if not specified
- Set via `trafficType` attribute in evaluation context
- Must match a defined traffic type in Split

**Localhost Mode:** Track events are accepted but not persisted (no server to send them to). Code using `Track()` runs
unchanged in local development.

**View Events:** Track events appear in Split Data Hub (Live Tail tab).

### Event Handling

Subscribe to provider lifecycle events:

```go
openfeature.AddHandler(openfeature.ProviderReady, func(details openfeature.EventDetails) {
	log.Println("Provider ready")
})

openfeature.AddHandler(openfeature.ProviderConfigChange, func(details openfeature.EventDetails) {
	log.Println("Configuration updated")
})
```

**Events:**

- `PROVIDER_READY` - Provider initialized successfully
- `PROVIDER_CONFIG_CHANGE` - Flag configurations updated (detected via polling, default 30s, configurable via
  `WithMonitoringInterval`)
- `PROVIDER_ERROR` - Initialization or runtime error

**Event Limitations:**

- `PROVIDER_STALE` events are not emitted due to Split SDK limitations. See [Provider Status](#provider-status) for
  details.
- `PROVIDER_CONFIG_CHANGE` is detected by polling (default 30 seconds, configurable via `WithMonitoringInterval`,
  minimum
  5 seconds), not via real-time SSE streaming. While the Split SDK receives changes instantly via SSE, it doesn't expose
  a callback for configuration changes, so the provider polls `manager.Splits()` to detect changes. See
  [CONTRIBUTING.md](CONTRIBUTING.md) for details.

### Direct SDK Access

**⚠️ Advanced Usage Only**

The provider manages the Split SDK lifecycle (initialization, shutdown, cleanup). Direct factory access should only be
used for Split-specific features not available through OpenFeature.

**Lifecycle Constraints:**

- ❌ **DO NOT** call `factory.Client().Destroy()` - provider owns lifecycle
- ❌ **DO NOT** call `factory.Client().BlockUntilReady()` - use `openfeature.Status()` instead
- ⚠️ Factory is only valid between `Init` and `Shutdown`
- ⚠️ After `Shutdown()`, factory and client are destroyed

**Example:**

```go
factory := provider.Factory()
// Use factory for Split-specific features not available in OpenFeature
```

See [Split Go SDK documentation](https://github.com/splitio/go-client) for available methods.

## Testing

**Unit tests:** Use OpenFeature test provider, not Split provider.

**Integration tests:** Use localhost mode with YAML files. See [test/integration/](./test/integration/).

**Provider tests:**

```bash
task test              # Run all tests
task test-race         # Run with race detection
task test-coverage     # Generate coverage report
```

## Development

Development workflow managed via [Taskfile](./Taskfile.yml):

```bash
task                       # List all tasks
task example-localhost     # Run localhost example
task example-cloud         # Run cloud example
task test-integration      # Run integration tests
task lint                  # Run linters
```

## Logging

Provider uses `slog` for structured logging. Configure via `slog.SetDefault()` or `split.WithLogger()` option.

**Source attribution:**

- `source="split-provider"` - Provider logs
- `source="split-sdk"` - Split SDK logs
- `source="openfeature-sdk"` - OpenFeature SDK logs (via hooks)

See [examples/](./examples/) for logging configuration patterns.

## Contributing

Contributions welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing requirements, and PR
process.

## License

Apache License 2.0. See [LICENSE](http://www.apache.org/licenses/LICENSE-2.0).

## Links

- [Split.io](https://www.split.io/)
- [OpenFeature](https://openfeature.dev/)
- [API Documentation](https://pkg.go.dev/github.com/splitio/split-openfeature-provider-go/v2)
- [Issue Tracker](https://github.com/splitio/split-openfeature-provider-go/issues)
