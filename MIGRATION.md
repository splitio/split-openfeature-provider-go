# Migration Guide: v1 to v2

## Overview

Version 2.0.0 includes critical bug fixes and SDK upgrades.

### Bug Fixes

- `ObjectEvaluation()` returns structured map with treatment and config fields
- Dynamic Configuration supports any JSON type (objects, arrays, primitives)
- Evaluation context attributes passed to Split SDK for targeting rules
- `Shutdown()` properly cleans up all resources
- Non-string targeting keys validated and rejected

### SDK Updates

- Split Go SDK updated to v6
- OpenFeature Go SDK updated to v1
- Go minimum version: 1.25

## Breaking Changes

### Import Paths

```go
// v1
import (
	"github.com/splitio/go-client/splitio/client"
	"github.com/open-feature/go-sdk/pkg/openfeature"
)

// v2
import (
	"github.com/splitio/go-client/v6/splitio/client"
	"github.com/open-feature/go-sdk/openfeature"
)
```

### Provider Initialization

Use `SetProviderWithContextAndWait()` for synchronous initialization with timeout:

```go
// v1
openfeature.SetProvider(provider)

// v2 - Recommended with context and timeout
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

err := openfeature.SetProviderWithContextAndWait(ctx, provider)
if err != nil {
	log.Fatal(err)
}

// v2 - Alternative: No timeout (uses default from BlockUntilReady config)
err := openfeature.SetProviderAndWait(provider)
if err != nil {
	log.Fatal(err)
}
```

### Context Required

```go
// v1
result, _ := client.BooleanValue(nil, "flag-key", false, evalCtx)

// v2
ctx := context.Background()
result, _ := client.BooleanValue(ctx, "flag-key", false, evalCtx)
```

## Migration Steps

### 1. Update Dependencies

```bash
go get github.com/splitio/split-openfeature-provider-go/v2@latest
go get github.com/splitio/go-client/v6@latest
go get github.com/open-feature/go-sdk@latest
go mod tidy
```

### 2. Update Imports

```go
import (
	"context"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/split-openfeature-provider-go/v2"
)
```

### 3. Update Initialization

```go
provider, err := split.New(apiKey)
if err != nil {
	log.Fatal(err)
}

// Defer shutdown with context
defer func() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := openfeature.ShutdownWithContext(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}()

// Initialize with context and timeout
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

err = openfeature.SetProviderWithContextAndWait(ctx, provider)
if err != nil {
	log.Fatal(err)
}

client := openfeature.NewClient("my-app")
```

### 4. Add Context to Evaluations

```go
ctx := context.Background()
evalCtx := openfeature.NewEvaluationContext("user-123", map[string]any{
	"email": "user@example.com",
})
result, _ := client.BooleanValue(ctx, "my-feature", false, evalCtx)
```

## Behavioral Changes

### Dynamic Configurations

v1 returned treatment name. v2 returns structured map with treatment and config:

```go
result, _ := client.ObjectValue(ctx, "my-flag", nil, evalCtx)
// v1: "on" (treatment only)
// v2: {"my-flag": {"treatment": "on", "config": {"feature": "enabled", "limit": 100}}}

// Dynamic Configuration is accessible via FlagMetadata["value"]
details, _ := client.StringValueDetails(ctx, "my-flag", "default", evalCtx)
if configValue, ok := details.FlagMetadata["value"]; ok {
	// All config types wrapped in "value" key for consistent access
	// Object: configValue.(map[string]any)
	// Primitive: configValue.(float64), configValue.(string), etc.
	// Array: configValue.([]any)
}
```

### Targeting Rules

v1 ignored evaluation context attributes. v2 passes them correctly:

```go
evalCtx := openfeature.NewEvaluationContext("user-123", map[string]any{
	"plan": "premium",
})
result, _ := client.BooleanValue(ctx, "premium-feature", false, evalCtx)
// v1: attributes ignored
// v2: targeting rules work
```

### Logging

v1 used plain text logs. v2 uses structured JSON logs with `slog`.

## New Features

### Custom Logger

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))
slog.SetDefault(logger)
```

### Health Check

```go
metrics := provider.Metrics()
```

### Factory Access

```go
factory := provider.Factory()
manager := factory.Manager()
```

## Compatibility

| Component       | v1.x  | v2.x  |
|-----------------|-------|-------|
| Go Version      | 1.19+ | 1.25+ |
| Split SDK       | v5/v6 | v6    |
| OpenFeature SDK | v0    | v1    |

## Complete Example

### v1

```go
import (
	"github.com/open-feature/go-sdk/pkg/openfeature"
	"github.com/splitio/split-openfeature-provider-go"
)

provider, _ := split.NewProviderSimple("YOUR_API_KEY")
openfeature.SetProvider(provider)
client := openfeature.NewClient("my-app")

evalCtx := openfeature.NewEvaluationContext("user-123", nil)
result, _ := client.BooleanValue(nil, "my-feature", false, evalCtx)
```

### v2

```go
import (
	"context"
	"log"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/split-openfeature-provider-go/v2"
)

provider, err := split.New("YOUR_API_KEY")
if err != nil {
	log.Fatal(err)
}

defer func() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	openfeature.ShutdownWithContext(ctx)
}()

ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

err := openfeature.SetProviderWithContextAndWait(ctx, provider)
if err != nil {
	log.Fatal(err)
}

client := openfeature.NewClient("my-app")

evalCtx := openfeature.NewEvaluationContext("user-123", nil)
result, _ := client.BooleanValue(context.Background(), "my-feature", false, evalCtx)
```
