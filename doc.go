// Package split provides an OpenFeature provider implementation for Split.io
// feature flags and A/B testing platform.
//
// # Basic Usage
//
//	provider, err := split.New("YOUR_API_KEY")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
//	defer cancel()
//	if err := openfeature.SetProviderWithContextAndWait(ctx, provider); err != nil {
//	    log.Fatal(err)
//	}
//
//	client := openfeature.NewClient("my-app")
//	evalCtx := openfeature.NewEvaluationContext("user-123", map[string]any{
//	    "email": "user@example.com",
//	})
//	enabled, _ := client.BooleanValue(context.Background(), "new-feature", false, evalCtx)
//
// Evaluations return default values on errors. Use *ValueDetails methods to
// distinguish success from fallback via Reason and ErrorCode fields.
//
// # Configuration
//
//	cfg := conf.Default()
//	cfg.BlockUntilReady = 15
//
//	provider, _ := split.New("YOUR_API_KEY",
//	    split.WithSplitConfig(cfg),
//	    split.WithLogger(logger),
//	)
//
// # Concurrency
//
// The provider is thread-safe. Multiple goroutines can evaluate flags
// concurrently. Shutdown waits for in-flight evaluations to complete.
//
// See README.md for complete documentation and examples.
package split
