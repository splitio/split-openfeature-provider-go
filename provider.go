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

// Provider implements the OpenFeature FeatureProvider interface for Split.
//
// The provider manages one background goroutine for monitoring configuration
// changes. Init may also spawn a goroutine for BlockUntilReady (tracked via
// WaitGroup). All goroutines are tracked and terminated during Shutdown.
type Provider struct {
	client             Client
	initGroup          singleflight.Group
	monitorDone        chan struct{}
	logger             *slog.Logger
	eventStream        chan of.Event
	stopMonitor        chan struct{}
	splitConfig        *conf.SplitSdkConfig
	factory            *client.SplitFactory
	loggedOnce         sync.Map // one-time log deduplication (small fixed set of keys)
	initWg             sync.WaitGroup
	monitoringInterval time.Duration
	mtx                sync.RWMutex // protects client, factory, eventStream access
	initMu             sync.Mutex   // serializes InitWithContext calls
	shutdown           uint32
}

// logOnce logs a message once per key.
func (p *Provider) logOnce(key string, logFn func()) {
	if _, loaded := p.loggedOnce.LoadOrStore(key, struct{}{}); !loaded {
		logFn()
	}
}

// Config holds provider configuration.
type Config struct {
	// SplitConfig is the Split SDK configuration.
	// If nil, conf.Default() is used.
	SplitConfig *conf.SplitSdkConfig

	// Logger is the slog.Logger used for provider and Split SDK logs.
	// If nil, slog.Default() is used.
	Logger *slog.Logger

	// SDKKey is the Split SDK key or "localhost" for local mode.
	SDKKey string

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
// The provider wraps this logger with SlogToSplitAdapter for Split SDK compatibility,
// unless SplitConfig.Logger is already set.
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
// The sdkKey parameter is required. Additional configuration can be provided
// via functional options. Returns error if sdkKey is empty or the Split SDK
// factory fails to create.
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
func New(sdkKey string, opts ...Option) (*Provider, error) {
	if sdkKey == "" {
		return nil, fmt.Errorf("split provider: sdkKey is required")
	}

	cfg := &Config{
		SDKKey:      sdkKey,
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

	factory, err := client.NewSplitFactory(cfg.SDKKey, cfg.SplitConfig)
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
