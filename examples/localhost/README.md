# Localhost Mode Example

**No Split account needed!** This example demonstrates offline flag evaluation using local YAML files.

## What This Demonstrates

- Localhost mode configuration (no network calls to Split)
- Loading feature flags from local YAML file
- User-specific targeting with key-based routing
- All flag types (Boolean, String, Integer, Float, Object)
- Flag metadata (JSON configurations attached to treatments)
- Colored structured logging with source attribution
- Perfect for CI/CD and integration tests

## Why Use Localhost Mode?

Perfect for:

- Local development without Split account
- Unit/integration testing with predictable values
- CI/CD pipelines requiring deterministic behavior
- Working offline or in restricted networks

**WARNING:** Localhost mode does NOT sync with Split servers. Development/testing only - never use in production.

## Running

```bash
cd examples/localhost
go run main.go
```

No environment variables or API keys needed! The example will:

1. Load flags from `split.yaml`
2. Evaluate flags for multiple users
3. Show targeting behavior
4. Display structured logs with source attribution

## Split File Format

The `split.yaml` file defines feature flags:

```yaml
- flag_name:
    treatment: "value"
    keys: "user-1,user-2"  # Optional: target specific users
    config: '{"key": "value"}'  # Optional: JSON configuration
```

## Limitations

**Flag Sets Not Supported:** Localhost mode does NOT support flag sets for bulk evaluation.

## Troubleshooting

### "File not found" Error

- Ensure `split.yaml` exists in the same directory
- Use absolute paths if needed: `cfg.SplitFile = "/path/to/split.yaml"`

### Flags Always Return Defaults

- Check YAML syntax (proper indentation)
- Verify flag names match exactly (case-sensitive)
- Check the `keys` field if using targeted rollouts

### Invalid YAML Format

- Ensure proper YAML structure
- Use quotes around string values with special characters
- Validate YAML with online tools

## Learn More

- [Cloud Example](../cloud/) - Cloud mode with streaming
- [Split OpenFeature Go Provider Documentation](../../README.md)
- [OpenFeature Go SDK](https://openfeature.dev/docs/reference/sdks/server/go)
- [Split Go SDK](https://github.com/splitio/go-client)
