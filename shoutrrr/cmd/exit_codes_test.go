package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExitError_Error tests the Error() method of ExitError.
//
//nolint:gosmopolitan // Intentional string literal containing rune in Han script (gosmopolitan)
func TestExitError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		e    ExitError
		want string
	}{
		{
			name: "empty message",
			e:    ExitError{ExitCode: ExSuccess, Message: ""},
			want: "",
		},
		{
			name: "usage error with invalid flag",
			e:    ExitError{ExitCode: ExUsage, Message: "invalid flag"},
			want: "invalid flag",
		},
		{
			name: "config error with missing token",
			e:    ExitError{ExitCode: ExConfig, Message: "missing token"},
			want: "missing token",
		},
		{
			name: "unicode message",
			e:    ExitError{ExitCode: ExUnavailable, Message: "通知错误"},
			want: "通知错误",
		},
		{
			name: "long message",
			e:    ExitError{ExitCode: ExUsage, Message: strings.Repeat("a", 1000)},
			want: strings.Repeat("a", 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.e.Error()
			assert.Equal(t, tt.want, got, "ExitError.Error() returned unexpected value")
		})
	}
}

// TestExitError_Error_ImplementsInterface verifies ExitError implements the error interface.
func TestExitError_Error_ImplementsInterface(t *testing.T) {
	t.Parallel()

	// Verify ExitError implements error interface
	var _ error = ExitError{ExitCode: ExSuccess, Message: "test"}

	// Verify error interface methods work correctly
	err := ExitError{ExitCode: ExConfig, Message: "interface test"}
	assert.Equal(t, "interface test", err.Error())
}

// TestInvalidUsage tests the InvalidUsage() factory function.
func TestInvalidUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
		want    ExitError
	}{
		{
			name:    "simple message with bad args",
			message: "bad args",
			want:    ExitError{ExitCode: ExUsage, Message: "bad args"},
		},
		{
			name:    "empty message",
			message: "",
			want:    ExitError{ExitCode: ExUsage, Message: ""},
		},
		{
			name:    "message with formatting usage",
			message: "usage: cmd --flag",
			want:    ExitError{ExitCode: ExUsage, Message: "usage: cmd --flag"},
		},
		{
			name:    "message with special characters",
			message: "error: $VAR & | pipe",
			want:    ExitError{ExitCode: ExUsage, Message: "error: $VAR & | pipe"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := InvalidUsage(tt.message)
			assert.Equal(t, tt.want, got, "InvalidUsage() returned unexpected ExitError")
			assert.Equal(t, ExUsage, got.ExitCode, "InvalidUsage() should have ExitCode ExUsage")
			assert.Equal(t, tt.message, got.Message, "InvalidUsage() should preserve message")
		})
	}
}

// TestTaskUnavailable tests the TaskUnavailable() factory function.
func TestTaskUnavailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
		want    ExitError
	}{
		{
			name:    "service down with smtp server unreachable",
			message: "smtp server unreachable",
			want:    ExitError{ExitCode: ExUnavailable, Message: "smtp server unreachable"},
		},
		{
			name:    "network error with connection timeout",
			message: "connection timeout",
			want:    ExitError{ExitCode: ExUnavailable, Message: "connection timeout"},
		},
		{
			name:    "empty message",
			message: "",
			want:    ExitError{ExitCode: ExUnavailable, Message: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := TaskUnavailable(tt.message)
			assert.Equal(t, tt.want, got, "TaskUnavailable() returned unexpected ExitError")
			assert.Equal(t, ExUnavailable, got.ExitCode, "TaskUnavailable() should have ExitCode ExUnavailable")
			assert.Equal(t, tt.message, got.Message, "TaskUnavailable() should preserve message")
		})
	}
}

// TestConfigurationError tests the ConfigurationError() factory function.
func TestConfigurationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
		want    ExitError
	}{
		{
			name:    "missing token for discord",
			message: "discord: missing webhook",
			want:    ExitError{ExitCode: ExConfig, Message: "discord: missing webhook"},
		},
		{
			name:    "invalid URL with parse error",
			message: "parse error: scheme",
			want:    ExitError{ExitCode: ExConfig, Message: "parse error: scheme"},
		},
		{
			name:    "empty message",
			message: "",
			want:    ExitError{ExitCode: ExConfig, Message: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ConfigurationError(tt.message)
			assert.Equal(t, tt.want, got, "ConfigurationError() returned unexpected ExitError")
			assert.Equal(t, ExConfig, got.ExitCode, "ConfigurationError() should have ExitCode ExConfig")
			assert.Equal(t, tt.message, got.Message, "ConfigurationError() should preserve message")
		})
	}
}
