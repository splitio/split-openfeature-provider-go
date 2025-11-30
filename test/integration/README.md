# Integration Test

Comprehensive integration test validating all provider features with automated assertions.

## What This Tests

- Custom Split SDK configuration with flexible mode (localhost or cloud)
- Structured logging with slog and colored output via tint
- Event handling (PROVIDER_READY, PROVIDER_ERROR, PROVIDER_CONFIGURATION_CHANGED)
- Graceful shutdown with context cancellation and interrupt handling
- All evaluation types (boolean, string, int, float, object)
- Evaluation details (variant, reason, flag key)
- Flag metadata (configurations attached to flags)
- Flag set evaluation (cloud mode only)
- Targeting with user attributes
- Context cancellation and timeout handling
- Direct Split SDK access (Track, Treatments)
- Concurrent evaluations (100 goroutines x 10 evaluations)
- Provider lifecycle (init, shutdown, named providers)

**Test Coverage:**
- Localhost mode: 73 tests
- Cloud mode: 81 tests (includes flag set tests)

## File Structure

| File             | Purpose                                                                |
|------------------|------------------------------------------------------------------------|
| `main.go`        | Entry point, setup, and test orchestration                             |
| `results.go`     | Test result tracking with atomic counters                              |
| `evaluations.go` | Flag evaluation tests (boolean, string, int, float, object, targeting) |
| `lifecycle.go`   | Provider lifecycle tests (init, shutdown, named providers, timeouts)   |
| `sdk.go`         | SDK access, concurrent evaluations, metrics, and health tests          |

## Running

```bash
cd test/integration

# Localhost mode (recommended - no API key needed)
go run .

# With debug logging
LOG_LEVEL=debug go run .

# Cloud mode (requires flags created per test/cloud_flags.yaml)
SPLIT_API_KEY=your-key go run .
```

## Test Modes

### Localhost Mode (default)

Uses `split.yaml` file with test flags. Runs all tests except flag set evaluation (73 tests).

### Cloud Mode (with SPLIT_API_KEY)

Connects to Split cloud. Runs flag set evaluation tests in addition to all other tests (81 tests total).
Requires flags created per `test/cloud_flags.yaml`.

## Exit Codes

- `0`: All tests passed
- `1`: One or more tests failed
- `2`: Timeout or fatal error

Timeout: 5 minutes.

## Learn More

- [Advanced Test](../advanced/) - Configuration change event detection (requires manual flag modification)
- [Split OpenFeature Go Provider Documentation](../../README.md)
- [OpenFeature Go SDK](https://openfeature.dev/docs/reference/sdks/server/go)
- [Split Go SDK](https://github.com/splitio/go-client)
