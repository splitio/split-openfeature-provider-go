# Cloud Example

**Cloud mode example** demonstrating Split OpenFeature Provider in streaming/cloud mode.

## What This Demonstrates

- Provider initialization in **streaming/cloud mode** with structured colored logging
- Boolean, String, Integer, Float, and **Object** flag evaluations
- Evaluation context with targeting keys and attributes
- Getting evaluation details (variant, reason, flag metadata)
- **Flag sets** evaluation (object evaluations in cloud mode)
- **Flag metadata** (JSON configurations attached to treatments)
- Provider health checks
- Source-attributed logs for debugging

**Requires Split API key** - Connects to Split's cloud service for real-time flag updates via streaming.

## Prerequisites

Get your Split API key from [Split.io](https://split.io) (use server-side SDK key).

## Running

```bash
cd examples/cloud
export SPLIT_API_KEY="your-server-side-sdk-key"
go run main.go
```

The example will:

1. Initialize the Split provider in cloud/streaming mode
2. Evaluate multiple flag types (boolean, string, int, float, object)
3. Demonstrate flag sets and flag metadata
4. Show evaluation details and provider health
5. Display structured colored logs with source attribution

## Troubleshooting

### "SPLIT_API_KEY environment variable is required"

- Make sure you've set the environment variable: `export SPLIT_API_KEY="your-key"`
- Verify your key is correct in the UI under Admin → API Keys

### Flags returning default values

- This is normal if flags don't exist in Split
- Create the flags in the UI to see different behaviors
- Check that you're using the correct SDK key (server-side, not client-side)

### Provider initialization timeout

- Check your network connection
- Verify the API key is valid
- The SDK needs to download flag definitions on first run

## Learn More

- [Split OpenFeature Go Provider Documentation](../../README.md)
- [OpenFeature Go SDK](https://openfeature.dev/docs/reference/sdks/server/go)
- [Split Go SDK](https://github.com/splitio/go-client)
