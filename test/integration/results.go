// results.go provides test result tracking infrastructure.
// It includes atomic counters for pass/fail tracking and proper error aggregation.
package main

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-multierror"
)

// TestResults tracks test results with atomic counters and proper error aggregation.
// Uses atomic.Int64 for lock-free counter updates and go-multierror for proper error handling.
type TestResults struct {
	passed atomic.Int64
	failed atomic.Int64
	total  atomic.Int64
	mu     sync.Mutex        // Protects result during concurrent Append operations
	result *multierror.Error // Accumulated test failures using go-multierror
}

func (tr *TestResults) Pass(testName string) {
	tr.passed.Add(1)
	tr.total.Add(1)
	slog.Info("PASS", "test", testName)
}

func (tr *TestResults) Fail(testName string, reason string) {
	tr.failed.Add(1)
	tr.total.Add(1)

	// Thread-safe error accumulation using go-multierror
	tr.mu.Lock()
	tr.result = multierror.Append(tr.result, fmt.Errorf("%s: %s", testName, reason))
	tr.mu.Unlock()

	slog.Error("FAIL", "test", testName, "reason", reason)
}

func (tr *TestResults) Summary() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	passed := tr.passed.Load()
	failed := tr.failed.Load()
	total := tr.total.Load()

	percentage := 0.0
	if total > 0 {
		percentage = float64(passed) / float64(total) * 100
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Test Results: %d/%d passed (%.1f%%)\n", passed, total, percentage)
	if tr.result != nil {
		fmt.Println()
		fmt.Printf("Failed tests (%d):\n", failed)
		fmt.Println(tr.result.Error())
	} else if total > 0 {
		fmt.Println("All tests passed!")
	} else {
		fmt.Println("No tests were run")
	}
	fmt.Println(strings.Repeat("=", 60))
}

var results = new(TestResults)

// section logs a visually distinct section header for test phases.
func section(name string) {
	slog.Info(strings.Repeat("-", 60))
	slog.Info(fmt.Sprintf(">> %s", name))
	slog.Info(strings.Repeat("-", 60))
}
