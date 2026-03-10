# Split OpenFeature Provider (Go) – Examples

This folder contains runnable examples to manually test all provider scenarios.

## Requirements

- Go 1.21+ (same as the root module)
- Run from the repo root: `go run ./example` or from `example/`: `go run .`

## Main example (`main.go`)

Covers all scenarios in a single executable:

1. **Initialization** – Split client in localhost mode with `split.yaml` (no real API key required).
2. **Basic evaluations** – Boolean, String, Int, Float, and Object with an example user.
3. **Context at different levels** – Global (API), client, and invocation.
4. **Evaluation with details** – `*ValueDetails`, variant, reason, and `FlagMetadata["config"]`.
5. **Tracking** – Sending events with `trafficType`, value, and attributes.
6. **Error handling** – Missing targeting key, non-existent flag, parse error.
7. **Realistic flow** – Simulation of multiple users (key, randomKey, guest) and per-user decisions.

### How to run

From the `example/` directory (recommended, so `./split.yaml` is used):

```bash
cd example
go mod tidy
go run .
```

From the repo root (set path to the splits file):

```bash
cd /path/to/split-openfeature-provider-go
SPLIT_FILE=example/split.yaml go run ./example
```

The `split.yaml` in the `example/` directory defines the example flags in localhost mode.
