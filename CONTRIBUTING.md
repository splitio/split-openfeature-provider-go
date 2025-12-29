# Contributing to Split OpenFeature Go Provider

We welcome contributions! This guide covers how to build, test, and submit changes.

**Quick Links:**

- [README.md](README.md) - Main documentation
- [MIGRATION.md](MIGRATION.md) - v1 → v2 migration guide
- [CHANGELOG.md](CHANGELOG.md) - Version history

---

## Prerequisites

- **Go 1.25.4+**
- **Task** - [taskfile.dev](https://taskfile.dev)
- **golangci-lint** - For linting

### Install Task

```bash
# macOS
brew install go-task/tap/go-task

# Linux
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin

# Via Go
go install github.com/go-task/task/v3/cmd/task@latest
```

### Install Development Tools

```bash
task install-tools    # Install golangci-lint and other tools
task check-tools      # Verify installation
```

---

## Development Workflow

### 1. Fork and Clone

```bash
git clone https://github.com/YOUR_USERNAME/split-openfeature-provider-go.git
cd split-openfeature-provider-go
git remote add upstream https://github.com/splitio/split-openfeature-provider-go.git
```

### 2. Create Feature Branch

```bash
git fetch upstream
git checkout -b feat/your-feature-name upstream/main
```

### 3. Make Changes

**Run tests first:**

```bash
task test              # Run all tests with race detector
```

**Make your changes:**

- Add tests for new functionality (use testify/assert)
- Follow Go idioms and best practices
- Add godoc comments for exported symbols
- Keep functions focused and small

**Validate:**

```bash
task                   # Run lint + test + coverage
task pre-commit        # Quick pre-commit checks
```

### 4. Write Tests

**Requirements:**

- Use `testify/assert` or `testify/require` for assertions
- Maintain >70% coverage (`task coverage-check`)
- Tests must pass race detector
- Test both success and error cases

**Example:**

```go
func TestFeatureName(t *testing.T) {
	provider, err := setupTestProvider(t)
	require.NoError(t, err, "Setup failed")

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{"valid input", "test", "expected", false},
		{"invalid input", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.YourMethod(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
```

### 5. Commit and Push

**Use Conventional Commits:**

```bash
git commit -m "feat: add new feature"
git commit -m "fix: resolve bug with shutdown"
git commit -m "docs: update README examples"
```

**Types:** `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `chore`

```bash
git push origin feat/your-feature-name
```

### 6. Create Pull Request

**PR Checklist:**

- [ ] All tests pass (`task test`)
- [ ] Linter passes (`task lint`)
- [ ] Coverage maintained at >70% (`task coverage-check`)
- [ ] Documentation updated
- [ ] Godoc comments added
- [ ] No goroutine leaks
- [ ] Concurrency safety verified

---

## Testing

### Unit Tests

```bash
task test              # All tests with race detector
task test-short        # Quick test run
task coverage          # View coverage report
task coverage-check    # Verify 70% threshold
```

### Integration Tests

```bash
task test-integration  # Uses SPLIT_API_KEY if set, otherwise localhost mode
task test-cloud        # Cloud-only features (requires SPLIT_API_KEY)
```

**Integration Test (`test/integration/`)** - Automated test suite:

- Localhost mode: 73 tests (no API key needed)
- Cloud mode: 81 tests (requires SPLIT_API_KEY)
- All evaluation types (boolean, string, int, float, object)
- Lifecycle management and concurrent evaluations
- Event handling and dynamic configurations

**Cloud Test (`test/advanced/`)** - Cloud-only features:

- Event tracking (view in Split Data Hub)
- Configuration change detection
- Interactive testing for cloud-specific functionality

**Cloud Mode Testing Setup:**

To run integration tests in cloud mode, create the required flags in your Split.io account.
See `test/cloud_flags.yaml` for the flag definitions:

1. Create 11 flags as documented in `test/cloud_flags.yaml`
2. Create a flag set named `split_provider_test`
3. Add `ui_theme` and `api_version` flags to the flag set
4. Run tests:

```bash
SPLIT_API_KEY="your-key" task test-integration
```

**When Are These Tests Executed?**

Neither test suite runs as part of CI (`task ci`). Run manually:

```bash
# Integration test - localhost mode (no API key)
task test-integration

# Integration test - cloud mode (requires API key and flags)
SPLIT_API_KEY="your-key" task test-integration

# Cloud test - cloud mode (requires API key)
SPLIT_API_KEY="your-key" task test-cloud
```

**Recommendation:** Run `task test-integration` before submitting PRs that affect:

- Provider initialization/shutdown
- Flag evaluation logic
- Event handling
- Dynamic configuration parsing

---

## Code Quality

### Required Standards

- All exported symbols must have godoc comments
- golangci-lint must pass
- Coverage >70%
- No race conditions
- No goroutine leaks
- Thread-safety verified for shared state

### Common Commands

```bash
# Workflows
task                   # Show available tasks
task check             # Run all quality checks
task pre-commit        # Quick pre-commit
task ci                # Full CI suite

# Testing
task test              # Unit tests with race detector
task test-integration  # Integration tests (localhost or cloud)
task test-cloud        # Cloud-only tests (requires API key)
task coverage          # Coverage report

# Code Quality
task lint              # Run linter
task lint-fix          # Auto-fix issues
task fmt               # Format code
task vet               # Run go vet

# Examples
task example-cloud     # Cloud mode (requires SPLIT_API_KEY)
task example-localhost # Localhost mode (no API key)

# Tools
task install-tools     # Install dev tools
task clean             # Clean artifacts
```

---

## Project Structure

```
split-openfeature-provider-go/
├── provider.go                 # Core provider
├── lifecycle.go                # Init/Shutdown (context-aware)
├── events.go                   # Event system
├── evaluation.go               # Flag evaluations
├── helpers.go                  # Helpers and Factory()
├── logging.go                  # Slog adapter
├── constants.go                # Constants
├── provider_test.go            # Unit tests
├── lifecycle_edge_cases_test.go # Concurrency tests
├── examples/
│   ├── cloud/                  # Cloud mode example
│   └── localhost/              # Localhost mode example
└── test/
    ├── cloud_flags.yaml        # Flag definitions for cloud testing
    ├── integration/            # Integration tests (localhost + cloud)
    └── advanced/               # Advanced tests (cloud-only features)
```

---

## v2 Status: Production Ready ✅

- ✅ Context-aware lifecycle with timeouts
- ✅ Full OpenFeature event compliance
- ✅ Optimal test coverage with race detection
- ✅ Structured logging with slog
- ✅ Thread-safe concurrent operations

---

## Known Limitations & Future Enhancements

### PROVIDER_STALE Event Not Emitted

**Status:** Known limitation (Split SDK dependency)

The provider cannot emit `PROVIDER_STALE` events when network connectivity is lost. This is due to a limitation in the
Split Go SDK:

- `factory.IsReady()` only indicates **initial** readiness after `BlockUntilReady()` completes
- The method does **not** change when the SDK loses network connectivity during operation
- Internally, the SDK handles connectivity issues (switching between streaming and polling modes) but does not expose
  this state through its public API

**Impact:**

- When network connectivity is lost, the SDK continues serving cached data silently
- Applications cannot detect when they are receiving potentially stale feature flag values
- The `PROVIDER_CONFIGURATION_CHANGED` event still works correctly when flags are updated

**Potential Future Enhancement:**
If the Split SDK exposes streaming/connectivity status in a future version, this provider could be updated to:

1. Monitor the streaming status channel for `StatusUp`/`StatusDown` events
2. Emit `PROVIDER_STALE` when streaming disconnects and polling begins
3. Emit `PROVIDER_READY` when streaming reconnects

**Workaround for Applications:**
Applications requiring staleness awareness should implement application-level health checks, such as:

- Periodic test evaluations with known flags
- Monitoring SDK debug logs for connectivity errors
- External health check endpoints to Split.io APIs

**References:**

- Split SDK sync manager: `go-split-commons/synchronizer/manager.go`
- Push status constants: `StatusUp`, `StatusDown`, `StatusRetryableError`, `StatusNonRetryableError`
- SSE keepAlive timeout: 70 seconds (hardcoded in SDK)

### PROVIDER_CONFIGURATION_CHANGED Detected via Polling

**Status:** Known limitation (Split SDK dependency)

The `PROVIDER_CONFIGURATION_CHANGED` event is detected by polling, not via real-time SSE streaming. The polling interval
is configurable via `WithMonitoringInterval` (default: 30 seconds, minimum: 5 seconds).

**Why Polling?**

- The Split SDK receives configuration changes instantly via SSE streaming
- However, the SDK does **not** expose a callback or event for configuration changes
- The only way to detect changes is by polling `manager.Splits()` and comparing `ChangeNumber` values

**Impact:**

- Flag evaluations reflect changes immediately (SDK updates its cache via SSE)
- `PROVIDER_CONFIGURATION_CHANGED` events have latency up to the configured monitoring interval
- Applications relying on this event for cache invalidation may see delayed notifications

**Potential Future Enhancement:**
If the Split SDK exposes a configuration change callback in a future version, this provider could be updated to:

1. Register a callback for real-time change notifications
2. Emit `PROVIDER_CONFIGURATION_CHANGED` immediately when changes arrive via SSE
3. Remove the polling-based detection

---

## Resources

**Documentation:**

- [OpenFeature Specification](https://openfeature.dev/specification/sections/providers)
- [OpenFeature Go SDK](https://openfeature.dev/docs/reference/sdks/server/go/)
- [Split Go SDK](https://github.com/splitio/go-client)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)

**Help:**

- [GitHub Issues](https://github.com/splitio/split-openfeature-provider-go/issues) - Bug reports and feature requests
- [Pull Requests](https://github.com/splitio/split-openfeature-provider-go/pulls) - Contributions

---

## License

By contributing, you agree your contributions will be licensed under Apache License 2.0.
