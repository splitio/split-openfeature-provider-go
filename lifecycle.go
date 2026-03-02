package split

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	of "github.com/open-feature/go-sdk/openfeature"
)

// Init implements StateHandler, which is embedded in ContextAwareStateHandler.
// Required by the interface contract but never called by the OpenFeature SDK
// (it calls InitWithContext instead). Delegates to InitWithContext with a
// timeout derived from BlockUntilReady config + 5s buffer.
func (p *Provider) Init(evaluationContext of.EvaluationContext) error {
	timeout := defaultInitTimeout // Defensive fallback (New() guarantees BlockUntilReady > 0)
	if p.splitConfig != nil && p.splitConfig.BlockUntilReady > 0 {
		timeout = time.Duration(p.splitConfig.BlockUntilReady)*time.Second + initTimeoutBuffer
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return p.InitWithContext(ctx, evaluationContext)
}

// InitWithContext initializes the provider with context support.
//
// Implements ContextAwareStateHandler. The OpenFeature SDK calls this method
// directly. Init() also delegates here with a derived timeout context.
// Idempotent: returns nil immediately if already initialized.
// Returns error if called after Shutdown (create a new provider instance instead).
func (p *Provider) InitWithContext(ctx context.Context, evaluationContext of.EvaluationContext) error {
	_ = evaluationContext // Intentionally unused; required by ContextAwareStateHandler interface

	p.initMu.Lock()
	defer p.initMu.Unlock()

	// Cannot re-initialize after shutdown
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return fmt.Errorf("cannot initialize provider after shutdown: provider has been permanently shut down, create a new provider instance")
	}

	p.mtx.RLock()
	if p.factory != nil && p.factory.IsReady() {
		p.mtx.RUnlock()
		p.logger.Debug("provider already initialized")
		return nil
	}
	p.mtx.RUnlock()

	_, err, _ := p.initGroup.Do("init", func() (any, error) {
		p.mtx.RLock()
		if p.factory != nil && p.factory.IsReady() {
			p.mtx.RUnlock()
			p.logger.Debug("provider already initialized (concurrent init detected)")
			return nil, nil
		}
		p.mtx.RUnlock()

		// BlockUntilReady can take 10+ seconds, so we monitor ctx.Done() for cancellation
		p.logger.Debug("waiting for Split SDK to be ready", "timeout_seconds", p.splitConfig.BlockUntilReady)

		// Run in goroutine since BlockUntilReady doesn't support context
		readyErr := make(chan error, 1)
		p.initWg.Add(1)
		go func() {
			defer p.initWg.Done()
			readyErr <- p.client.BlockUntilReady(p.splitConfig.BlockUntilReady)
		}()

		select {
		case <-ctx.Done():
			select {
			case err := <-readyErr:
				if err != nil {
					errMsg := fmt.Errorf("split SDK failed to become ready within %d seconds: %w",
						p.splitConfig.BlockUntilReady, err)
					p.emitEvent(&of.Event{
						ProviderName: p.Metadata().Name,
						EventType:    of.ProviderError,
						ProviderEventDetails: of.ProviderEventDetails{
							Message: errMsg.Error(),
						},
					})
					return nil, errMsg
				}
				p.logger.Debug("SDK initialized successfully despite context cancellation")
			default:
				errMsg := fmt.Errorf("initialization canceled: %w", ctx.Err())
				p.emitEvent(&of.Event{
					ProviderName: p.Metadata().Name,
					EventType:    of.ProviderError,
					ProviderEventDetails: of.ProviderEventDetails{
						Message: errMsg.Error(),
					},
				})
				return nil, errMsg
			}
		case err := <-readyErr:
			if err != nil {
				errMsg := fmt.Errorf("split SDK failed to become ready within %d seconds: %w",
					p.splitConfig.BlockUntilReady, err)
				p.emitEvent(&of.Event{
					ProviderName: p.Metadata().Name,
					EventType:    of.ProviderError,
					ProviderEventDetails: of.ProviderEventDetails{
						Message: errMsg.Error(),
					},
				})
				return nil, errMsg
			}
			p.logger.Debug("SDK became ready successfully")
		}

		// We hold write lock to ensure:
		//   1. If Shutdown() is closing stopMonitor, we wait then see shutdown flag
		//   2. If we start monitoring, Shutdown() will wait for monitorDone
		// This prevents the deadlock where Shutdown waits for monitorDone that never closes
		p.mtx.Lock()

		// Prevents starting monitoring goroutine after shutdown
		if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
			p.mtx.Unlock()
			return nil, fmt.Errorf("provider was shut down during initialization")
		}

		if !p.factory.IsReady() {
			p.mtx.Unlock()
			err := fmt.Errorf("split SDK BlockUntilReady succeeded but factory not ready")
			p.emitEvent(&of.Event{
				ProviderName: p.Metadata().Name,
				EventType:    of.ProviderError,
				ProviderEventDetails: of.ProviderEventDetails{
					Message: err.Error(),
				},
			})
			return nil, err
		}

		splitCount := 0
		if manager := p.factory.Manager(); manager != nil {
			splitNames := manager.SplitNames()
			splitCount = len(splitNames)
		}

		// Guarantees that if we start monitoring, Shutdown() will wait for monitorDone
		go p.monitorSplitUpdates()
		p.mtx.Unlock()

		p.emitEvent(&of.Event{
			ProviderName: p.Metadata().Name,
			EventType:    of.ProviderReady,
			ProviderEventDetails: of.ProviderEventDetails{
				Message: "Split provider initialized successfully",
			},
		})

		p.logger.Info("Split provider ready", "splits_loaded", splitCount)
		return nil, nil
	})

	return err
}

// Shutdown implements StateHandler, which is embedded in ContextAwareStateHandler.
// Required by the interface contract but never called by the OpenFeature SDK
// (it calls ShutdownWithContext instead). Delegates to ShutdownWithContext with a
// default timeout of 30s, or BlockUntilReady duration if it exceeds 30s.
// Errors from ShutdownWithContext are logged but cannot be returned (void interface).
func (p *Provider) Shutdown() {
	timeout := defaultShutdownTimeout
	if p.splitConfig != nil && p.splitConfig.BlockUntilReady > 0 {
		configTimeout := time.Duration(p.splitConfig.BlockUntilReady) * time.Second
		if configTimeout > timeout {
			timeout = configTimeout
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := p.ShutdownWithContext(ctx); err != nil {
		p.logger.Warn("shutdown completed with errors",
			"error", err,
			"note", "StateHandler.Shutdown() has no return value, error cannot be propagated")
	}
}

// ShutdownWithContext gracefully shuts down the provider with context support.
//
// Implements ContextAwareStateHandler. Idempotent: subsequent calls return nil.
// The provider is immediately marked as shut down (new operations return
// PROVIDER_NOT_READY). Cleanup runs on a best-effort basis within the context
// deadline; if the deadline expires, ctx.Err() is returned but cleanup continues
// in background. Never panics; SDK panics during Destroy are recovered and logged.
//
// Shutdown order: stop monitor goroutine → wait for init goroutines → close event channel → destroy SDK client.
//
// Split SDK Destroy() may block up to 1 hour in streaming mode (known SDK issue).
// Recommended minimum timeout: 30 seconds.
func (p *Provider) ShutdownWithContext(ctx context.Context) error {
	// Atomic CAS prevents race with emitEvent()
	if !atomic.CompareAndSwapUint32(&p.shutdown, shutdownStateInactive, shutdownStateActive) {
		p.logger.Debug("provider already shut down")
		return nil
	}

	p.logger.Debug("shutting down Split provider")

	var shutdownErr error

	// We hold write lock to ensure:
	//   1. If Init() is starting monitoring, we wait then close stopMonitor safely
	//   2. Our wasInitialized check happens atomically with stopMonitor close
	// This prevents the deadlock where we wait for monitorDone that was never started
	p.logger.Debug("stopping background monitoring goroutine")
	p.mtx.Lock()
	close(p.stopMonitor)
	wasInitialized := p.factory != nil && p.factory.IsReady()
	p.mtx.Unlock()

	if wasInitialized {
		p.logger.Debug("waiting for background monitoring to stop")
		select {
		case <-p.monitorDone:
			p.logger.Debug("background monitoring stopped")
		case <-ctx.Done():
			shutdownErr = ctx.Err()
			p.logger.Warn("context deadline exceeded while waiting for monitoring goroutine, forcing shutdown",
				"reason", "monitoring goroutine may still be running",
				"error", shutdownErr)
		}
	} else {
		p.logger.Debug("provider was never initialized, skipping monitoring cleanup")
	}

	// Prevents goroutine leak when Init is canceled but BlockUntilReady still running.
	// BlockUntilReady has a built-in timeout, so initWg.Wait() is bounded.
	if err := p.waitForInitGoroutines(ctx); err != nil && shutdownErr == nil {
		shutdownErr = err
	}

	// Order matters: monitoring stopped -> init goroutines done -> NOW safe to close channel and destroy client
	operationMode := "unknown"
	if p.splitConfig != nil {
		operationMode = p.splitConfig.OperationMode
	}
	p.logger.Debug("destroying Split SDK client", "mode", operationMode)

	destroyStart := time.Now()
	destroyDone := make(chan struct{})
	go func() {
		defer close(destroyDone)

		p.mtx.Lock()
		clientToDestroy := p.client
		p.client = nil
		close(p.eventStream)
		p.mtx.Unlock()

		if clientToDestroy != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						p.logger.Error("panic during Split SDK destroy",
							"panic", r,
							"advice", "this may indicate a bug in Split SDK")
					}
				}()
				clientToDestroy.Destroy()
			}()
		}
		elapsed := time.Since(destroyStart).Milliseconds()
		p.logger.Debug("Split SDK client destroyed", "duration_ms", elapsed)
	}()

	select {
	case <-destroyDone:
		elapsed := time.Since(destroyStart).Milliseconds()
		p.logger.Debug("Split SDK client destroyed successfully", "duration_ms", elapsed)
	case <-ctx.Done():
		if shutdownErr == nil {
			shutdownErr = ctx.Err()
		}
		elapsed := time.Since(destroyStart).Milliseconds()
		p.logger.Warn("context deadline exceeded during Split SDK destroy, forcing shutdown",
			"elapsed_ms", elapsed,
			"mode", operationMode,
			"reason", "known Split SDK streaming mode issue - SSE connection blocks on read",
			"error", shutdownErr)
	}

	if shutdownErr != nil {
		p.logger.Warn("Split provider shutdown completed with errors",
			"error", shutdownErr,
			"note", "provider is logically shut down but cleanup may be incomplete")
		return shutdownErr
	}

	p.logger.Debug("Split provider shut down successfully")
	return nil
}

// waitForInitGoroutines waits for initialization goroutines to complete,
// respecting the context deadline. Returns ctx.Err() if the deadline expires.
func (p *Provider) waitForInitGoroutines(ctx context.Context) error {
	p.logger.Debug("waiting for initialization goroutines to complete")
	initDone := make(chan struct{})
	go func() {
		p.initMu.Lock()
		p.initWg.Wait()
		p.initMu.Unlock()
		close(initDone)
	}()

	select {
	case <-initDone:
		p.logger.Debug("initialization goroutines completed")
		return nil
	case <-ctx.Done():
		p.logger.Warn("context deadline exceeded while waiting for initialization goroutines",
			"error", ctx.Err())
		return ctx.Err()
	}
}

// Status returns the current state of the provider.
//
// Used by the OpenFeature SDK to determine provider readiness. Returns one of:
//   - NotReadyState: Provider not initialized or shut down
//   - ReadyState: Provider initialized and ready for evaluations
//
// The state is derived from the Split SDK factory's ready status.
// This method is atomic - it checks both shutdown flag and factory state
// together to prevent race conditions during shutdown.
func (p *Provider) Status() of.State {
	// Prevents TOCTOU race: reads shutdown flag and factory under the same lock hold
	p.mtx.RLock()
	shutdown := atomic.LoadUint32(&p.shutdown) == shutdownStateActive
	factory := p.factory
	p.mtx.RUnlock()

	if shutdown {
		return of.NotReadyState
	}

	if factory != nil && factory.IsReady() {
		return of.ReadyState
	}

	return of.NotReadyState
}

// ProviderMetrics contains provider health and diagnostic information.
//
// All fields are always populated. SplitsCount is -1 when the provider
// is not ready (not initialized or shut down).
type ProviderMetrics struct {
	// Provider is the provider name ("Split").
	Provider string

	// Status is the current provider state (NotReadyState or ReadyState).
	Status of.State

	// SplitsCount is the number of split definitions loaded.
	// Set to -1 when the provider is not ready.
	SplitsCount int

	// Initialized indicates whether the provider is initialized and ready.
	Initialized bool

	// Ready indicates whether the provider is ready for evaluations.
	// Same as Initialized — both derived from factory ready state and shutdown flag.
	Ready bool
}

// Metrics returns the current metrics and status of the provider.
//
// Example:
//
//	m := provider.Metrics()
//	fmt.Printf("Provider: %s, Status: %s, Splits: %d\n",
//	    m.Provider, m.Status, m.SplitsCount)
func (p *Provider) Metrics() ProviderMetrics {
	// Consistent with Status(): prevents TOCTOU race
	p.mtx.RLock()
	shutdown := atomic.LoadUint32(&p.shutdown) == shutdownStateActive
	factory := p.factory
	p.mtx.RUnlock()

	isReady := !shutdown && factory != nil && factory.IsReady()

	var status of.State
	if isReady {
		status = of.ReadyState
	} else {
		status = of.NotReadyState
	}

	m := ProviderMetrics{
		Provider:    "Split",
		Initialized: isReady,
		Status:      status,
		Ready:       isReady,
		SplitsCount: -1,
	}

	if isReady && factory != nil {
		if manager := factory.Manager(); manager != nil {
			m.SplitsCount = len(manager.SplitNames())
		}
	}

	return m
}
