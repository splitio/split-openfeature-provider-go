package split

import (
	"fmt"
	"sync/atomic"
	"time"

	of "github.com/open-feature/go-sdk/openfeature"
)

// EventChannel returns a channel for receiving provider lifecycle events.
//
// This method implements the EventHandler interface. The OpenFeature SDK
// uses this channel to receive events about provider state changes.
//
// Events Emitted:
//   - PROVIDER_READY: Provider initialized successfully
//   - PROVIDER_ERROR: Provider encountered initialization error
//   - PROVIDER_CONFIGURATION_CHANGED: Split definitions updated (detected via polling)
//
// Configuration Change Detection Limitation:
// PROVIDER_CONFIGURATION_CHANGED is detected by polling, not via real-time SSE streaming.
// While the Split SDK receives changes instantly via SSE, it doesn't expose a callback
// for configuration changes. The provider polls manager.Splits() and compares ChangeNumber
// values to detect changes. The polling interval is configurable via WithMonitoringInterval
// (default: 30 seconds, minimum: 5 seconds).
//
// Staleness Detection Limitation:
// PROVIDER_STALE events are NOT currently emitted. The Split SDK's IsReady()
// method only indicates initial readiness and does not change when network
// connectivity is lost during operation. The SDK handles connectivity issues
// internally (switching between streaming and polling modes) but does not
// expose this state through its public API.
//
// When network connectivity is lost, the SDK continues serving cached data
// silently. Applications requiring staleness awareness should implement
// application-level health checks or monitor SDK debug logs.
//
// See CONTRIBUTING.md for details on this known limitation and potential
// future enhancements if Split SDK exposes streaming/connectivity status.
//
// The channel is buffered (100 events) to prevent blocking event emission.
// Applications can register handlers via openfeature.AddHandler() to react to events.
//
// Example:
//
//	openfeature.AddHandler(openfeature.ProviderReady, func(details openfeature.EventDetails) {
//	    log.Println("Split provider is ready!")
//	})
//
//	openfeature.AddHandler(openfeature.ProviderConfigChange, func(details openfeature.EventDetails) {
//	    log.Println("Feature flags updated - may want to re-evaluate")
//	})
func (p *Provider) EventChannel() <-chan of.Event {
	return p.eventStream
}

// emitEvent sends an event to the event channel without blocking.
//
// If the channel buffer is full, the event is dropped and a warning is logged.
// This prevents slow event consumers from blocking provider operations.
// If the provider is shut down and the channel is closed, the send is silently ignored.
//
// Concurrency Safety Design:
// Uses atomic shutdown check as a fast path, then acquires a brief read lock
// for the actual channel send. This prevents race detector warnings while
// keeping the lock duration minimal (just the non-blocking select).
func (p *Provider) emitEvent(event *of.Event) {
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return
	}

	// Acquire read lock for channel send to prevent race with close()
	// The lock duration is minimal - just the non-blocking select
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	// Double-check shutdown after acquiring lock
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return
	}

	select {
	case p.eventStream <- *event:
	default:
		p.logger.Warn("event channel full, dropping event", "eventType", event.EventType)
	}
}

// monitorSplitUpdates runs in a background goroutine to monitor Split SDK updates.
//
// This goroutine:
//   - Polls the Split SDK for changes in split definitions
//   - Emits PROVIDER_CONFIGURATION_CHANGED events when splits are updated
//   - Gracefully shuts down when stopMonitor channel is closed
//
// The monitoring interval is configurable via WithMonitoringInterval (default: 30s, min: 5s).
//
// Panic Recovery:
// If a panic occurs (e.g., nil pointer in SDK), the goroutine recovers, logs the error,
// and terminates gracefully. This prevents the monitoring goroutine from leaving
// monitorDone unclosed, which would cause shutdown to hang.
func (p *Provider) monitorSplitUpdates() {
	defer func() {
		// Panic recovery MUST be first defer to catch any panic
		// before closing monitorDone (which would propagate the panic)
		if r := recover(); r != nil {
			p.logger.Error("monitoring goroutine panicked, terminating gracefully",
				"panic", r,
				"advice", "this may indicate a bug in Split SDK or provider implementation")
		}
		close(p.monitorDone)
		p.logger.Debug("monitoring goroutine stopped")
	}()

	p.mtx.RLock()
	if p.factory == nil {
		p.mtx.RUnlock()
		p.logger.Warn("no factory available for monitoring")
		return
	}

	manager := p.factory.Manager()
	if manager == nil {
		p.mtx.RUnlock()
		p.logger.Warn("factory manager is nil, stopping monitoring",
			"reason", "Split SDK may not be fully initialized or factory is in invalid state")
		return
	}

	// Track splits by name and change number to detect any configuration changes
	lastKnownSplits := make(map[string]int64)
	splits := manager.Splits()
	for i := range splits {
		lastKnownSplits[splits[i].Name] = splits[i].ChangeNumber
	}
	p.mtx.RUnlock()

	p.logger.Debug("starting background Split monitoring",
		"interval", p.monitoringInterval,
		"initial_splits", len(lastKnownSplits))

	ticker := time.NewTicker(p.monitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopMonitor:
			p.logger.Debug("received shutdown signal, stopping monitoring")
			return

		case <-ticker.C:
			p.mtx.RLock()
			currentSplits := make(map[string]int64)
			currentSplitList := manager.Splits()
			for i := range currentSplitList {
				currentSplits[currentSplitList[i].Name] = currentSplitList[i].ChangeNumber
			}
			p.mtx.RUnlock()

			if splitsChanged(lastKnownSplits, currentSplits) {
				p.logger.Debug("Split definitions changed",
					"oldCount", len(lastKnownSplits),
					"newCount", len(currentSplits))
				p.emitEvent(&of.Event{
					ProviderName: p.Metadata().Name,
					EventType:    of.ProviderConfigChange,
					ProviderEventDetails: of.ProviderEventDetails{
						Message: fmt.Sprintf("Split definitions updated (count: %d)", len(currentSplits)),
					},
				})
				lastKnownSplits = currentSplits
			}
		}
	}
}

// splitsChanged checks if splits have changed by comparing names and change numbers.
// Returns true if any split was added, removed, or modified.
func splitsChanged(old, current map[string]int64) bool {
	if len(old) != len(current) {
		return true
	}
	for name, changeNum := range current {
		oldChangeNum, exists := old[name]
		if !exists || oldChangeNum != changeNum {
			return true
		}
	}
	return false
}
