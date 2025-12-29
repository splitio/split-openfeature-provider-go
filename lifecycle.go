package split

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	of "github.com/open-feature/go-sdk/openfeature"
)

// Init implements StateHandler for backward compatibility.
// Delegates to InitWithContext with a timeout derived from BlockUntilReady config.
// Uses BlockUntilReady timeout + 5s buffer to ensure SDK has enough time.
func (p *Provider) Init(evaluationContext of.EvaluationContext) error {
	// Determine timeout: BlockUntilReady + buffer for SDK operations
	timeout := defaultInitTimeout // Default: 10s BlockUntilReady + 5s buffer
	if p.splitConfig != nil && p.splitConfig.BlockUntilReady > 0 {
		timeout = time.Duration(p.splitConfig.BlockUntilReady)*time.Second + initTimeoutBuffer
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return p.InitWithContext(ctx, evaluationContext)
}

// InitWithContext initializes the provider with context support.
//
// This method implements the ContextAwareStateHandler interface and provides
// context-aware initialization that respects cancellation and timeouts.
//
// The context is used to:
//   - Cancel initialization if the caller's deadline is exceeded
//   - Support graceful shutdown during initialization
//   - Propagate cancellation signals from the caller
//
// This method performs the same initialization sequence as Init(), but monitors
// ctx.Done() during the BlockUntilReady call to allow early termination.
func (p *Provider) InitWithContext(ctx context.Context, evaluationContext of.EvaluationContext) error {
	_ = evaluationContext // Currently unused but reserved for future enhancements

	p.initMu.Lock()
	defer p.initMu.Unlock()

	// Check if provider has been shut down - cannot re-initialize after shutdown
	// Once Shutdown() is called, the Split SDK client is destroyed and cannot be reused
	if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
		return fmt.Errorf("cannot initialize provider after shutdown: provider has been permanently shut down, create a new provider instance")
	}

	// Fast path: check if already initialized with read lock only
	p.mtx.RLock()
	if p.factory != nil && p.factory.IsReady() {
		p.mtx.RUnlock()
		p.logger.Debug("provider already initialized")
		return nil
	}
	p.mtx.RUnlock()

	// Use singleflight to ensure only one initialization happens
	// All concurrent InitWithContext() calls wait for the same result
	_, err, _ := p.initGroup.Do("init", func() (any, error) {
		// Double-check after acquiring singleflight lock
		p.mtx.RLock()
		if p.factory != nil && p.factory.IsReady() {
			p.mtx.RUnlock()
			p.logger.Debug("provider already initialized (concurrent init detected)")
			return nil, nil
		}
		p.mtx.RUnlock()

		// Block until Split SDK is ready WITH context monitoring
		// This can take 10+ seconds, so we monitor ctx.Done() for cancellation
		p.logger.Debug("waiting for Split SDK to be ready", "timeout_seconds", p.splitConfig.BlockUntilReady)

		// Run BlockUntilReady in goroutine since it doesn't support context
		readyErr := make(chan error, 1)
		p.initWg.Add(1)
		go func() {
			defer p.initWg.Done() // Signal goroutine completion
			readyErr <- p.client.BlockUntilReady(p.splitConfig.BlockUntilReady)
		}()

		// Wait for either ready or context cancellation
		select {
		case <-ctx.Done():
			// Context canceled before SDK ready - check if readyErr also completed
			select {
			case err := <-readyErr:
				// SDK completed after context canceled - check result
				if err != nil {
					// SDK failed AND context canceled - return SDK error
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
				// SDK succeeded even though context canceled - proceed with initialization
				p.logger.Debug("SDK initialized successfully despite context cancellation")
			default:
				// SDK still running, context truly canceled - return context error
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
			// SDK succeeded - check if context was canceled during initialization
			// If context canceled but SDK ready, we proceed (SDK is usable)
			p.logger.Debug("SDK became ready successfully")
		}

		// Atomically check shutdown and start monitoring to prevent race condition
		// We hold write lock to ensure:
		//   1. If Shutdown() is closing stopMonitor, we wait then see shutdown flag
		//   2. If we start monitoring, Shutdown() will wait for monitorDone
		// This prevents the deadlock where Shutdown waits for monitorDone that never closes
		p.mtx.Lock()

		// Check if shutdown happened during BlockUntilReady
		// This prevents starting monitoring goroutine after shutdown
		if atomic.LoadUint32(&p.shutdown) == shutdownStateActive {
			p.mtx.Unlock()
			return nil, fmt.Errorf("provider was shut down during initialization")
		}

		// Verify factory is ready (final confirmation that entire SDK is ready)
		// At this point, factory, client, and manager should all be ready
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

		// Get the number of splits loaded for informational logging
		splitCount := 0
		if manager := p.factory.Manager(); manager != nil {
			splitNames := manager.SplitNames()
			splitCount = len(splitNames)
		}

		// Start background monitoring while holding lock (atomic with shutdown check)
		// This guarantees that if we start monitoring, Shutdown() will wait for monitorDone
		go p.monitorSplitUpdates()
		p.mtx.Unlock()

		// Emit PROVIDER_READY event (emitEvent is concurrent-safe)
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

// Shutdown implements StateHandler for backward compatibility.
//
// Delegates to ShutdownWithContext with a timeout derived from BlockUntilReady config.
// Uses a generous timeout (30s default, or BlockUntilReady if larger) to allow clean shutdown.
//
// This method performs "best effort" shutdown within the timeout:
//   - Provider state is immediately marked as shut down (no new operations allowed)
//   - Cleanup operations run within timeout (monitoring stop, SDK destroy, channel close)
//   - If timeout expires, cleanup continues in background goroutines
//   - Always succeeds (never panics or hangs)
//
// See ShutdownWithContext for detailed best effort shutdown semantics.
func (p *Provider) Shutdown() {
	// Determine timeout: use default, or BlockUntilReady if larger
	timeout := defaultShutdownTimeout
	if p.splitConfig != nil && p.splitConfig.BlockUntilReady > 0 {
		configTimeout := time.Duration(p.splitConfig.BlockUntilReady) * time.Second
		if configTimeout > timeout {
			timeout = configTimeout
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_ = p.ShutdownWithContext(ctx) //nolint:errcheck // Shutdown() has no return value per OpenFeature interface
}

// ShutdownWithContext gracefully shuts down the provider with context support.
//
// This method implements the ContextAwareStateHandler interface and provides
// context-aware shutdown that respects cancellation and timeouts from the caller.
//
// # Return Values
//
// Returns nil if shutdown completes successfully within the context deadline.
// Returns ctx.Err() if the context expires before shutdown completes (context.DeadlineExceeded
// or context.Canceled). Note that even when an error is returned, the provider is logically
// shut down - the shutdown flag is set immediately and new operations will fail with
// PROVIDER_NOT_READY.
//
// # Shutdown Behavior
//
// The provider state is atomically set to "shut down" immediately upon entry, preventing
// new operations. Cleanup happens on a best-effort basis within the context deadline.
//
// If the context deadline expires during cleanup:
//  1. Warnings are logged about incomplete operations
//  2. ctx.Err() is returned to indicate timeout/cancellation
//  3. Cleanup continues in background goroutines that will eventually complete
//  4. Provider remains logically shut down (Status() returns NotReadyState)
//
// Cleanup operations and their timeout behavior:
//   - Event channel close: Always completes immediately
//   - Monitoring goroutine: May take up to 30s to terminate after stopMonitor signal
//   - Split SDK Destroy(): May take up to 1 hour in streaming mode (known SDK issue)
//
// The context is used to:
//   - Respect the caller's shutdown deadline
//   - Cancel long-running cleanup operations
//   - Provide graceful shutdown within time constraints
//
// Recommended minimum timeout: 30 seconds to allow monitoring goroutine to exit cleanly.
func (p *Provider) ShutdownWithContext(ctx context.Context) error {
	// Check if already shut down and set shutdown flag atomically
	// Using atomic operations to prevent race with emitEvent()
	if !atomic.CompareAndSwapUint32(&p.shutdown, shutdownStateInactive, shutdownStateActive) {
		p.logger.Debug("provider already shut down")
		return nil
	}

	p.logger.Debug("shutting down Split provider")

	// Track whether any timeout occurred during shutdown
	var shutdownErr error

	// Stop background monitoring (if it was started)
	// Note: Monitoring only starts after successful initialization
	// Atomically close stopMonitor and check if monitoring was started to prevent race condition
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

	// Wait for initialization goroutine(s) to finish
	// This prevents goroutine leak when Init is canceled but BlockUntilReady still running
	// This is a blocking wait with GUARANTEED termination within BlockUntilReady timeout
	// (BlockUntilReady has built-in timeout, so this wait is bounded and safe)
	p.logger.Debug("waiting for initialization goroutines to complete")
	p.initMu.Lock()
	p.initWg.Wait()
	p.initMu.Unlock()
	p.logger.Debug("initialization goroutines completed")

	// Destroy Split SDK client and close event channel
	// Order is critical: monitoring stopped -> init goroutines done -> NOW safe to close channel and destroy client
	operationMode := "unknown"
	if p.splitConfig != nil {
		operationMode = p.splitConfig.OperationMode
	}
	p.logger.Debug("destroying Split SDK client", "mode", operationMode)

	destroyStart := time.Now()
	destroyDone := make(chan struct{})
	go func() {
		p.mtx.Lock()
		clientToDestroy := p.client
		p.client = nil
		close(p.eventStream)
		p.mtx.Unlock()

		if clientToDestroy != nil {
			clientToDestroy.Destroy()
		}
		elapsed := time.Since(destroyStart).Milliseconds()
		p.logger.Debug("Split SDK client destroyed", "duration_ms", elapsed)
		close(destroyDone)
	}()

	// Wait for either destroy completion or context cancellation
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

// Status returns the current state of the provider.
//
// This method implements the StateHandler interface and returns one of:
//   - NotReadyState: Provider not initialized or shut down
//   - ReadyState: Provider initialized and ready for evaluations
//
// The state is derived from the Split SDK factory's ready status.
// This method is atomic - it checks both shutdown flag and factory state
// together to prevent race conditions during shutdown.
func (p *Provider) Status() of.State {
	// Atomic read of shutdown flag and factory state together
	// This prevents TOCTOU (time-of-check-time-of-use) race condition
	p.mtx.RLock()
	shutdown := atomic.LoadUint32(&p.shutdown) == shutdownStateActive
	factory := p.factory
	p.mtx.RUnlock()

	// If shut down, always NotReady
	if shutdown {
		return of.NotReadyState
	}

	// If we have a factory and it's ready, we're ready
	if factory != nil && factory.IsReady() {
		return of.ReadyState
	}

	// Otherwise, we're not ready
	return of.NotReadyState
}

// Metrics returns the current metrics and status of the provider.
//
// This method provides a comprehensive view of the provider's state for monitoring and diagnostics:
//   - provider: Provider name ("Split")
//   - initialized: Whether the provider has been initialized (derived from factory ready state)
//   - status: Current state (NotReady, Ready)
//   - splits_count: Number of split definitions loaded (only when ready)
//   - ready: Whether the Split SDK is ready (factory/client/manager)
//
// The splits_count field is only included when the SDK is ready, to avoid
// accessing the manager before initialization is complete.
//
// Concurrency Optimization:
// Minimizes lock hold time by releasing the lock before calling potentially
// expensive Manager() and SplitNames() operations. This prevents blocking
// write operations (Init/Shutdown) unnecessarily.
//
// Example:
//
//	metrics := provider.Metrics()
//	fmt.Printf("Provider: %s, Status: %s, Splits: %d\n",
//	    metrics["provider"], metrics["status"], metrics["splits_count"])
func (p *Provider) Metrics() map[string]any {
	// Check shutdown flag atomically
	shutdown := atomic.LoadUint32(&p.shutdown) == shutdownStateActive

	// Read factory with lock
	p.mtx.RLock()
	factory := p.factory
	p.mtx.RUnlock()

	// Compute derived state WITHOUT holding lock
	isReady := !shutdown && factory != nil && factory.IsReady()

	// Determine status from isReady (avoid redundant checks)
	var status of.State
	if isReady {
		status = of.ReadyState
	} else {
		status = of.NotReadyState
	}

	health := map[string]any{
		"provider":    "Split",
		"initialized": isReady,
		"status":      string(status),
		"ready":       isReady,
	}

	// Access manager WITHOUT holding lock (potentially expensive operation)
	// The manager requires the SDK to be fully initialized
	// Add defensive nil check even though factory is ready
	if isReady && factory != nil {
		if manager := factory.Manager(); manager != nil {
			health["splits_count"] = len(manager.SplitNames())
		}
	}

	return health
}
