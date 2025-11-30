package split

import (
	"fmt"
	"log/slog"
)

// SlogToSplitAdapter adapts Go's standard *slog.Logger to Split SDK's LoggerInterface.
//
// This adapter allows the Split SDK to use the same logger configured for the application
// via slog.SetDefault(), ensuring consistent logging across the provider and Split SDK.
// All Split SDK log levels are mapped directly to slog levels (Error→Error, Warning→Warn, etc.)
//
// This type is exported for advanced use cases where you need to configure the Split SDK
// client directly with structured logging support.
type SlogToSplitAdapter struct {
	logger *slog.Logger
}

// NewSplitLogger creates a Split SDK logger adapter from a slog.Logger.
//
// This function allows you to use Go's structured logging (slog) with the Split SDK
// by configuring the SDK before creating the provider.
//
// Example usage with custom logger configuration:
//
//	import (
//	    "log/slog"
//	    "github.com/splitio/go-client/v6/splitio/conf"
//	    split "github.com/splitio/split-openfeature-provider-go/v2"
//	)
//
//	// Configure custom slog logger
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
//	    Level: slog.LevelInfo,
//	}))
//
//	// Configure Split SDK with slog adapter
//	cfg := conf.Default()
//	cfg.Logger = split.NewSplitLogger(logger) // Use slog adapter
//	cfg.BlockUntilReady = 10
//
//	// Create provider with configured logging
//	provider, _ := split.New("YOUR_SDK_KEY", cfg)
//	defer provider.Shutdown()
//
// For local development/testing, you can use localhost mode with a local splits file.
//
// If logger is nil, slog.Default() is used.
func NewSplitLogger(logger *slog.Logger) *SlogToSplitAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogToSplitAdapter{logger: logger}
}

// Error logs an error message.
// If multiple arguments are provided, the first is treated as the message
// and remaining arguments are logged as structured "details" field.
func (a *SlogToSplitAdapter) Error(msg ...any) {
	a.log(a.logger.Error, msg...)
}

// Warning logs a warning message.
// If multiple arguments are provided, the first is treated as the message
// and remaining arguments are logged as structured "details" field.
func (a *SlogToSplitAdapter) Warning(msg ...any) {
	a.log(a.logger.Warn, msg...)
}

// Info logs an informational message.
// If multiple arguments are provided, the first is treated as the message
// and remaining arguments are logged as structured "details" field.
func (a *SlogToSplitAdapter) Info(msg ...any) {
	a.log(a.logger.Info, msg...)
}

// Debug logs a debug message.
// If multiple arguments are provided, the first is treated as the message
// and remaining arguments are logged as structured "details" field.
func (a *SlogToSplitAdapter) Debug(msg ...any) {
	a.log(a.logger.Debug, msg...)
}

// Verbose logs a verbose message (mapped to Debug level in slog).
// If multiple arguments are provided, the first is treated as the message
// and remaining arguments are logged as structured "details" field.
func (a *SlogToSplitAdapter) Verbose(msg ...any) {
	a.log(a.logger.Debug, msg...)
}

// log is a helper that preserves structured logging when multiple arguments are provided.
// Single argument: logged as message only.
// Multiple arguments: first as message, rest as structured "details" field.
func (a *SlogToSplitAdapter) log(logFunc func(string, ...any), msg ...any) {
	if len(msg) == 0 {
		logFunc("")
		return
	}
	if len(msg) == 1 {
		logFunc(fmt.Sprint(msg[0]))
		return
	}
	logFunc(fmt.Sprint(msg[0]), "details", msg[1:])
}
