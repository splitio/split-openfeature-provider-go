package split

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewSplitLoggerCreatesValidAdapter verifies factory function creates valid adapter.
func TestNewSplitLoggerCreatesValidAdapter(t *testing.T) {
	t.Run("with custom logger", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		adapter := NewSplitLogger(logger)

		require.NotNil(t, adapter)
		assert.NotNil(t, adapter.logger)
	})

	t.Run("with nil logger uses default", func(t *testing.T) {
		adapter := NewSplitLogger(nil)

		require.NotNil(t, adapter)
		assert.NotNil(t, adapter.logger)
		assert.Equal(t, slog.Default(), adapter.logger)
	})
}

// TestLogAdapterLogsAtCorrectLevel verifies all log levels produce correct output.
func TestLogAdapterLogsAtCorrectLevel(t *testing.T) {
	tests := []struct {
		name          string
		logFunc       func(*SlogToSplitAdapter, ...any)
		expectedLevel string
		slogLevel     slog.Level
		message       string
	}{
		{
			name:          "Error level",
			logFunc:       (*SlogToSplitAdapter).Error,
			expectedLevel: "ERROR",
			slogLevel:     slog.LevelError,
			message:       "test error message",
		},
		{
			name:          "Warning level",
			logFunc:       (*SlogToSplitAdapter).Warning,
			expectedLevel: "WARN",
			slogLevel:     slog.LevelWarn,
			message:       "test warning message",
		},
		{
			name:          "Info level",
			logFunc:       (*SlogToSplitAdapter).Info,
			expectedLevel: "INFO",
			slogLevel:     slog.LevelInfo,
			message:       "test info message",
		},
		{
			name:          "Debug level",
			logFunc:       (*SlogToSplitAdapter).Debug,
			expectedLevel: "DEBUG",
			slogLevel:     slog.LevelDebug,
			message:       "test debug message",
		},
		{
			name:          "Verbose maps to Debug level",
			logFunc:       (*SlogToSplitAdapter).Verbose,
			expectedLevel: "DEBUG",
			slogLevel:     slog.LevelDebug,
			message:       "test verbose message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
				Level: tt.slogLevel,
			}))
			adapter := NewSplitLogger(logger)

			tt.logFunc(adapter, tt.message)

			logOutput := buf.String()
			assert.Contains(t, logOutput, tt.expectedLevel)
			assert.Contains(t, logOutput, tt.message)

			// Verify JSON structure
			var logEntry map[string]any
			err := json.Unmarshal([]byte(logOutput), &logEntry)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedLevel, logEntry["level"])
			assert.Equal(t, tt.message, logEntry["msg"])
		})
	}
}

// TestLogAdapterPreservesStructuredData verifies structured logging with multiple arguments.
func TestLogAdapterPreservesStructuredData(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	adapter := NewSplitLogger(logger)

	// Log with structured details
	adapter.Info("operation completed", "duration_ms", 150, "success", true)

	logOutput := buf.String()

	// Verify JSON structure
	var logEntry map[string]any
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "INFO", logEntry["level"])
	assert.Equal(t, "operation completed", logEntry["msg"])

	// Verify structured details field exists
	require.Contains(t, logEntry, "details")
	details, ok := logEntry["details"].([]any)
	require.True(t, ok, "details should be an array")

	// Verify details contain the arguments
	require.Len(t, details, 4)
	assert.Equal(t, "duration_ms", details[0])
	assert.Equal(t, float64(150), details[1]) // JSON numbers are float64
	assert.Equal(t, "success", details[2])
	assert.Equal(t, true, details[3])
}

// TestLogAdapterDoesNotCreateDetailsForSingleArgument verifies single argument behavior.
func TestLogAdapterDoesNotCreateDetailsForSingleArgument(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	adapter := NewSplitLogger(logger)

	adapter.Info("simple message")

	logOutput := buf.String()

	// Verify JSON structure
	var logEntry map[string]any
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "INFO", logEntry["level"])
	assert.Equal(t, "simple message", logEntry["msg"])

	// Should NOT have details field for single argument
	assert.NotContains(t, logEntry, "details")
}

// TestLogAdapterSupportsStructuredLoggingAtAllLevels verifies all levels support structured data.
func TestLogAdapterSupportsStructuredLoggingAtAllLevels(t *testing.T) {
	tests := []struct {
		name     string
		logFunc  func(*SlogToSplitAdapter, ...any)
		expected string
	}{
		{"Error", (*SlogToSplitAdapter).Error, "ERROR"},
		{"Warning", (*SlogToSplitAdapter).Warning, "WARN"},
		{"Info", (*SlogToSplitAdapter).Info, "INFO"},
		{"Debug", (*SlogToSplitAdapter).Debug, "DEBUG"},
		{"Verbose", (*SlogToSplitAdapter).Verbose, "DEBUG"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))
			adapter := NewSplitLogger(logger)

			tt.logFunc(adapter, "message", "key", "value")

			logOutput := buf.String()

			var logEntry map[string]any
			err := json.Unmarshal([]byte(logOutput), &logEntry)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, logEntry["level"])
			assert.Equal(t, "message", logEntry["msg"])
			assert.Contains(t, logEntry, "details")
		})
	}
}

// TestLogAdapterCreatesStructuredDetailsFromMultipleArguments verifies details array creation.
func TestLogAdapterCreatesStructuredDetailsFromMultipleArguments(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	adapter := NewSplitLogger(logger)

	adapter.Info("message", " ", "with", " ", "multiple", " ", "args")

	logOutput := buf.String()

	// With structured logging, first arg is message, rest are details
	var logEntry map[string]any
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "message", logEntry["msg"])
	assert.Contains(t, logEntry, "details")
	details, ok := logEntry["details"].([]any)
	require.True(t, ok)
	assert.Len(t, details, 6) // All the remaining arguments
}

// TestLogAdapterFiltersLogsByLevel verifies log level filtering works correctly.
func TestLogAdapterFiltersLogsByLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Only warn and above
	}))
	adapter := NewSplitLogger(logger)

	adapter.Debug("debug message")
	adapter.Info("info message")
	adapter.Warning("warning message")
	adapter.Error("error message")

	logOutput := buf.String()

	// Debug and Info should be filtered out
	assert.NotContains(t, logOutput, "debug message")
	assert.NotContains(t, logOutput, "info message")

	// Warning and Error should be present
	assert.Contains(t, logOutput, "warning message")
	assert.Contains(t, logOutput, "error message")
}

// TestLogAdapterHandlesEmptyMessage verifies empty message logging.
func TestLogAdapterHandlesEmptyMessage(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	adapter := NewSplitLogger(logger)

	adapter.Info()

	logOutput := buf.String()
	// Empty message should still produce a log entry
	assert.Contains(t, logOutput, "INFO")

	var logEntry map[string]any
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "", logEntry["msg"])
}

// TestLogAdapterEscapesSpecialCharactersInJSON verifies JSON escaping.
func TestLogAdapterEscapesSpecialCharactersInJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	adapter := NewSplitLogger(logger)

	specialMsg := "message with \"quotes\" and \n newlines"
	adapter.Info(specialMsg)

	logOutput := buf.String()

	// Verify JSON is valid despite special characters
	var logEntry map[string]any
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	require.NoError(t, err)

	// Message should be properly escaped in JSON
	assert.Contains(t, logEntry["msg"], "quotes")
}

// TestLogAdapterFormatsNonStringArgumentsCorrectly verifies non-string formatting.
func TestLogAdapterFormatsNonStringArgumentsCorrectly(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	adapter := NewSplitLogger(logger)

	adapter.Info("count:", 42, "enabled:", true, "rate:", 3.14)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "count:")
	assert.Contains(t, logOutput, "42")
	assert.Contains(t, logOutput, "enabled:")
	assert.Contains(t, logOutput, "true")
	assert.Contains(t, logOutput, "rate:")
	assert.Contains(t, logOutput, "3.14")
}

// TestLogAdapterWorksWithTextHandler verifies text handler compatibility.
func TestLogAdapterWorksWithTextHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	adapter := NewSplitLogger(logger)

	adapter.Info("text handler message")

	logOutput := buf.String()
	assert.Contains(t, logOutput, "level=INFO")
	assert.Contains(t, logOutput, "text handler message")
}

// TestLogAdapterIsThreadSafe verifies concurrent logging safety.
func TestLogAdapterIsThreadSafe(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	adapter := NewSplitLogger(logger)

	// Launch 10 goroutines, each logging 10 messages
	const goroutines = 10
	const messagesPerGoroutine = 10

	done := make(chan bool)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			for j := 0; j < messagesPerGoroutine; j++ {
				adapter.Info("goroutine", id, "message", j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < goroutines; i++ {
		<-done
	}

	logOutput := buf.String()

	// Count the number of log entries
	logLines := strings.Split(strings.TrimSpace(logOutput), "\n")
	expectedLines := goroutines * messagesPerGoroutine
	assert.Equal(t, expectedLines, len(logLines), "should have %d log lines", expectedLines)

	// Verify all logs are valid JSON
	for i, line := range logLines {
		var logEntry map[string]any
		err := json.Unmarshal([]byte(line), &logEntry)
		assert.NoError(t, err, "line %d should be valid JSON: %s", i, line)
	}
}
