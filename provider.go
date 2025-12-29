package split

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	of "github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/client"
	"github.com/splitio/go-client/v6/splitio/conf"
	"golang.org/x/sync/singleflight"
)

// Provider implements the OpenFeature FeatureProvider interface for Split.io.
//
// # Goroutine Management and Lifecycle
//
// This provider spawns and manages goroutines with the following guarantees:
//
// 1. **Background Monitoring Goroutine** (monitorSplitUpdates in events.go)
//   - Spawned: During InitWithContext after SDK is ready
//   - Purpose: Monitors Split SDK for configuration changes
//   - Shutdown: Gracefully terminated via close(stopMonitor) in ShutdownWithContext
//   - Guarantee: Always terminates within monitoring interval (30s) after stopMonitor closed
//   - Tracking: monitorDone channel closed when goroutine exits (see defer in monitorSplitUpdates)
//   - Safety: Panic recovery ensures monitorDone always closed
//
// 2. **Initialization Goroutine** (BlockUntilReady wrapper in InitWithContext)
//   - Spawned: During InitWithContext to monitor SDK initialization
//   - Purpose: Wraps SDK's BlockUntilReady to allow context cancellation
//   - Termination: Always terminates when BlockUntilReady completes (max: BlockUntilReady timeout)
//   - Tracking: Tracked via sync.WaitGroup (initWg) - Add(1) before spawn, Done() on completion
//   - Cleanup: ShutdownWithContext blocks on initWg.Wait() before destroying client
//   - Guarantee: GUARANTEED no leak - Shutdown cannot complete until all init goroutines terminate
//   - Lifecycle: Short-lived, terminates within BlockUntilReady timeout (default 10s)
//
// 3. **Shutdown Goroutine** (client.Destroy wrapper in ShutdownWithContext)
//   - Spawned: During ShutdownWithContext to destroy Split SDK client
//   - Purpose: Wraps SDK's Destroy() to allow context timeout
//   - Termination: Terminates when client.Destroy() completes
//   - Known Issue: In streaming mode, Destroy() can block up to 1 hour (Split SDK SSE issue)
//   - Guarantee: Eventually terminates, but may outlive ShutdownWithContext's context timeout
//   - Impact: Acceptable - goroutine performs cleanup and terminates, doesn't affect functionality
//
// All goroutines are properly tracked and either terminate gracefully or have documented
// termination guarantees. No unbounded goroutine leaks exist in normal operation.
type Provider struct {
	// Pointer fields (8 bytes each on 64-bit)
	client      *client.SplitClient
	factory     *client.SplitFactory
	splitConfig *conf.SplitSdkConfig
	logger      *slog.Logger

	// Channel fields (pointer-sized)
	eventStream chan of.Event
	stopMonitor chan struct{}
	monitorDone chan struct{}

	// Large struct fields
	initGroup singleflight.Group
	mtx       sync.RWMutex
	initWg    sync.WaitGroup // Tracks initialization goroutines
	initMu    sync.Mutex     // Serializes Init/Shutdown lifecycle transitions to prevent initWg race

	// Smaller fields
	monitoringInterval time.Duration
	shutdown           uint32
}

// Config holds provider configuration.
type Config struct {
	// SplitConfig is the Split SDK configuration.
	// If nil, conf.Default() is used.
	SplitConfig *conf.SplitSdkConfig

	// Logger is the slog.Logger used for provider and Split SDK logs.
	// If nil, slog.Default() is used.
	Logger *slog.Logger

	// APIKey is the Split SDK key or "localhost" for local mode.
	APIKey string

	// MonitoringInterval is how often the provider checks for split definition changes.
	// Default: 30 seconds. Minimum: 5 seconds.
	// Lower values increase responsiveness but also CPU usage.
	MonitoringInterval time.Duration
}

// Option configures a provider Config.
type Option interface {
	apply(*Config)
}

// WithSplitConfig sets the Split SDK configuration.
func WithSplitConfig(cfg *conf.SplitSdkConfig) Option {
	return withSplitConfig{cfg}
}

type withSplitConfig struct {
	cfg *conf.SplitSdkConfig
}

func (o withSplitConfig) apply(c *Config) {
	c.SplitConfig = o.cfg
}

// WithLogger sets the logger for provider and Split SDK logs.
// This ensures unified logging across the provider, Split SDK, and OpenFeature SDK
// when the same logger is also passed to hooks.NewLoggingHook().
func WithLogger(logger *slog.Logger) Option {
	return withLogger{logger}
}

type withLogger struct {
	logger *slog.Logger
}

func (o withLogger) apply(c *Config) {
	c.Logger = o.logger
}

// WithMonitoringInterval sets how often the provider checks for split definition changes.
// Default: 30 seconds. Minimum: 5 seconds. Values below minimum are clamped.
func WithMonitoringInterval(interval time.Duration) Option {
	return withMonitoringInterval{interval}
}

type withMonitoringInterval struct {
	interval time.Duration
}

func (o withMonitoringInterval) apply(c *Config) {
	c.MonitoringInterval = o.interval
}

// New creates a Split provider with the given configuration.
//
// The apiKey parameter is required. Additional configuration can be provided
// via functional options.
//
// Example with defaults:
//
//	provider, _ := split.New("YOUR_SDK_KEY")
//
// Example with custom logger:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	provider, _ := split.New("YOUR_SDK_KEY", split.WithLogger(logger))
//
// Example with custom Split SDK config:
//
//	cfg := conf.Default()
//	cfg.OperationMode = "localhost"
//	provider, _ := split.New("localhost", split.WithSplitConfig(cfg))
//
// Example with unified logging (provider, Split SDK, and OpenFeature SDK):
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	slog.SetDefault(logger)
//	provider, _ := split.New("YOUR_SDK_KEY", split.WithLogger(logger))
//	openfeature.AddHooks(hooks.NewLoggingHook(false, logger))
//
// The provider is created in NotReady state. Call Init() (or use OpenFeature's
// SetProviderAndWait) to wait for the SDK to download splits. Always call Shutdown()
// when done to clean up resources.
func New(apiKey string, opts ...Option) (*Provider, error) {
	cfg := &Config{
		APIKey:      apiKey,
		SplitConfig: nil,
		Logger:      nil,
	}

	for _, opt := range opts {
		opt.apply(cfg)
	}

	if cfg.SplitConfig == nil {
		cfg.SplitConfig = conf.Default()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.SplitConfig.BlockUntilReady <= 0 {
		cfg.SplitConfig.BlockUntilReady = defaultSDKTimeout
	}

	providerLogger := cfg.Logger.With("source", "split-provider")

	// Apply monitoring interval defaults and minimum
	monitoringInterval := cfg.MonitoringInterval
	if monitoringInterval == 0 {
		monitoringInterval = defaultMonitoringInterval
	} else if monitoringInterval < minMonitoringInterval {
		providerLogger.Warn("monitoring interval below minimum, using minimum",
			"requested", monitoringInterval,
			"minimum", minMonitoringInterval)
		monitoringInterval = minMonitoringInterval
	}

	if cfg.SplitConfig.Logger == nil {
		splitSDKLogger := cfg.Logger.With("source", "split-sdk")
		cfg.SplitConfig.Logger = NewSplitLogger(splitSDKLogger)
	}

	factory, err := client.NewSplitFactory(cfg.APIKey, cfg.SplitConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Split factory: %w", err)
	}

	provider := &Provider{
		client:             factory.Client(),
		factory:            factory,
		eventStream:        make(chan of.Event, eventChannelBuffer),
		stopMonitor:        make(chan struct{}),
		monitorDone:        make(chan struct{}),
		splitConfig:        cfg.SplitConfig,
		monitoringInterval: monitoringInterval,
		logger:             providerLogger,
	}

	mode := "cloud"
	if provider.isLocalhostMode() {
		mode = "localhost"
	}
	providerLogger.Info("Split provider created",
		"mode", mode,
		"block_until_ready", cfg.SplitConfig.BlockUntilReady)

	return provider, nil
}

// Metadata returns provider metadata with name "Split".
func (p *Provider) Metadata() of.Metadata {
	return of.Metadata{
		Name: "Split",
	}
}
