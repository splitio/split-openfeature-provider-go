# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2025-11-24

**Complete architectural rewrite** with modern SDK support, production-grade lifecycle management, and critical bug
fixes.

See [MIGRATION.md](MIGRATION.md) for upgrade instructions.

### Breaking Changes

#### SDK Requirements

- **Split Go SDK upgraded to v6** (import: `github.com/splitio/go-client/v6`)
- **OpenFeature Go SDK upgraded to v1** (import: `github.com/open-feature/go-sdk/openfeature`)

#### API Changes

- **All evaluation methods now require `context.Context` as first parameter**
- **`Client()` renamed to `Factory()`** for Split SDK factory access
- **`NewWithClient()` constructor removed** - use `New()` instead

#### Behavioral Changes

- **`ObjectEvaluation()` return structure changed**:
  - v1: Returns treatment string only
  - v2: Returns `FlagSetResult` (typed struct with `Treatment` and `Config` fields)

### New Features

#### Context-Aware Lifecycle

- `InitWithContext(ctx)` - Context-aware initialization with timeout and cancellation
- `ShutdownWithContext(ctx)` - Graceful shutdown with timeout and proper cleanup
- Idempotent initialization with singleflight (prevents concurrent init races)
- Provider cannot be reused after shutdown (must create new instance)

#### Event System

- OpenFeature event support:
  - `PROVIDER_READY` - Provider initialized
  - `PROVIDER_ERROR` - Initialization or runtime errors
  - `PROVIDER_CONFIGURATION_CHANGED` - Flag definitions updated (detected via 30s polling)
- Background monitoring (30s interval) for configuration change detection

#### Event Tracking

- `Track()` method implementing OpenFeature Tracker interface
- Associates feature flag evaluations with user actions for A/B testing and experimentation
- Supports custom traffic types via `trafficType` attribute in evaluation context
- Supports event properties via `TrackingEventDetails.Add()`
- Events viewable in Split Data Hub

#### Observability

- Structured logging with `log/slog` throughout provider and Split SDK
- `Metrics()` method for health status and diagnostics
- Unified logging via `WithLogger()` option

### Bug Fixes

#### Critical Fixes

- **`ObjectEvaluation()` structure**: Now returns `FlagSetResult` with `Treatment` and `Config` fields (was: treatment
  string only)
- **Dynamic Configuration**: All config types (objects, primitives, arrays) consistently accessible via
  `FlagMetadata["value"]`
- **Dynamic Configuration JSON parsing**: Supports objects, arrays, and primitives (was: limited support)
- **Evaluation context attributes**: Now passed to Split SDK for targeting rules (was: ignored)
- **Shutdown resource cleanup**: Properly cleans up goroutines, channels, and SDK clients (was: resource leaks)

#### Error Handling

- **Shutdown timeout errors**: `ShutdownWithContext()` returns `ctx.Err()` when cleanup times out (was: no error
  indication)
- **JSON parse warnings**: Malformed Dynamic Configuration logged instead of silent failures
- **Targeting key validation**: Non-string keys rejected with clear errors (was: silent failures)

#### Concurrency & Reliability

- **Atomic initialization**: Factory, client, and manager ready together (was: race conditions)
- **Thread-safe health checks**: Eliminated race conditions in `Status()` and `Metrics()`
- **Event channel lifecycle**: Properly closed during shutdown (was: potential goroutine leaks)
- **Panic recovery**: Monitoring goroutine recovers from panics and terminates gracefully

## [1.0.1] - 2022-10-14

- Updated to OpenFeature spec v0.5.0 and OpenFeature Go SDK v0.6.0

## [1.0.0] - 2022-10-03

- Initial release
- OpenFeature spec v0.5.0 compliance
- OpenFeature Go SDK v0.5.0 support

[2.0.0]: https://github.com/splitio/split-openfeature-provider-go/compare/v1.0.1...v2.0.0

[1.0.1]: https://github.com/splitio/split-openfeature-provider-go/compare/v1.0.0...v1.0.1

[1.0.0]: https://github.com/splitio/split-openfeature-provider-go/releases/tag/v1.0.0
