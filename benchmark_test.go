package split

import (
	"context"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/conf"
	"github.com/splitio/go-toolkit/v5/logging"
)

// BenchmarkBooleanEvaluation benchmarks single boolean flag evaluation performance.
func BenchmarkBooleanEvaluation(b *testing.B) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	if err != nil {
		b.Fatalf("Failed to initialize provider: %v", err)
	}

	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "bench-user",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.BooleanEvaluation(context.TODO(), flagSomeOther, false, flatCtx)
	}
}

// BenchmarkStringEvaluation benchmarks single string flag evaluation performance.
func BenchmarkStringEvaluation(b *testing.B) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	if err != nil {
		b.Fatalf("Failed to initialize provider: %v", err)
	}

	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "bench-user",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.StringEvaluation(context.TODO(), flagSomeOther, "default", flatCtx)
	}
}

// BenchmarkConcurrentEvaluations benchmarks concurrent flag evaluations.
func BenchmarkConcurrentEvaluations(b *testing.B) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	if err != nil {
		b.Fatalf("Failed to initialize provider: %v", err)
	}

	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "bench-user",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = provider.BooleanEvaluation(context.TODO(), flagSomeOther, false, flatCtx)
		}
	})
}

// BenchmarkProviderInitialization measures provider initialization time.
func BenchmarkProviderInitialization(b *testing.B) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider, err := New("localhost", WithSplitConfig(cfg))
		if err != nil {
			b.Fatalf("Failed to create provider: %v", err)
		}

		err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
		if err != nil {
			b.Fatalf("Failed to initialize provider: %v", err)
		}

		_ = provider.ShutdownWithContext(context.Background())
	}
}

// BenchmarkAttributeHeavyEvaluation measures evaluation performance with many attributes.
func BenchmarkAttributeHeavyEvaluation(b *testing.B) {
	cfg := conf.Default()
	cfg.SplitFile = testSplitFile
	cfg.LoggerConfig.LogLevel = logging.LevelNone
	cfg.BlockUntilReady = 10

	provider, err := New("localhost", WithSplitConfig(cfg))
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}
	defer func() { _ = provider.ShutdownWithContext(context.Background()) }()

	err = provider.InitWithContext(context.Background(), openfeature.NewEvaluationContext("", nil))
	if err != nil {
		b.Fatalf("Failed to initialize provider: %v", err)
	}

	flatCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "bench-user",
		"email":                  "user@example.com",
		"plan":                   "enterprise",
		"region":                 "us-east-1",
		"org_id":                 "org-12345",
		"user_id":                "user-67890",
		"account_type":           "premium",
		"feature_flags_enabled":  true,
		"beta_tester":            true,
		"signup_date":            "2024-01-15",
		"last_login":             "2025-01-18",
		"session_count":          42,
		"total_spend":            1299.99,
		"conversion_rate":        0.25,
		"engagement_score":       87.5,
		"device_type":            "desktop",
		"browser":                "chrome",
		"os":                     "macos",
		"language":               "en-US",
		"timezone":               "America/New_York",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.BooleanEvaluation(context.TODO(), flagSomeOther, false, flatCtx)
	}
}
