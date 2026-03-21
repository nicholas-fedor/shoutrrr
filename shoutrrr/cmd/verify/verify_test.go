package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRun_FlagRetrievalError tests the Run function with flag retrieval error.
func TestRun_FlagRetrievalError(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		// Create url flag with wrong type to cause type mismatch error
		cmd.Flags().BoolP("url", "u", false, "")

		Run(cmd, []string{})

		return
	}

	// Run the test in a subprocess
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_FlagRetrievalError")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	var outputBuf bytes.Buffer

	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		assert.Equal(t, 1, exitErr.ExitCode(), "Exit code mismatch")
	} else {
		t.Fatalf("Expected ExitError, got: %v", err)
	}
}

// TestRun_UnknownService tests the Run function with unknown service error.
func TestRun_UnknownService(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		cmd.Flags().StringArrayP("url", "u", []string{}, "")

		_ = cmd.Flags().Set("url", "unknownservice://test")

		Run(cmd, []string{})

		return
	}

	// Run the test in a subprocess
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_UnknownService")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	var outputBuf bytes.Buffer

	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	err := cmd.Run()
	output := outputBuf.String()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		assert.Equal(t, 1, exitErr.ExitCode(), "Exit code mismatch")
	} else {
		t.Fatalf("Expected ExitError, got: %v", err)
	}

	// Error message should be sanitized
	assert.Contains(t, output, "service not recognized", "Error should be sanitized for unknown service")
}

// TestRun_InvalidURLFormat tests the Run function with invalid URL format.
func TestRun_InvalidURLFormat(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		cmd.Flags().StringArrayP("url", "u", []string{}, "")

		_ = cmd.Flags().Set("url", "://invalid")

		Run(cmd, []string{})

		return
	}

	// Run the test in a subprocess
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_InvalidURLFormat")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		assert.Equal(t, 1, exitErr.ExitCode(), "Exit code mismatch")
	} else {
		t.Fatalf("Expected ExitError, got: %v", err)
	}
}

// TestRun_SubprocessErrorOutput tests the error output messages from verify command.
// This specifically tests that sanitizeError is properly applied to error messages.
func TestRun_SubprocessErrorOutput(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		cmd.Flags().StringArrayP("url", "u", []string{}, "")

		// Use an unknown service URL
		_ = cmd.Flags().Set("url", "xyzservice://test")

		Run(cmd, []string{})

		return
	}

	// Run in subprocess
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_SubprocessErrorOutput")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	var outputBuf bytes.Buffer

	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	err := cmd.Run()
	output := outputBuf.String()

	// Should exit with code 1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		assert.Equal(t, 1, exitErr.ExitCode(), "Should exit with code 1")
	} else {
		t.Fatalf("Expected ExitError, got: %v", err)
	}

	// Error message should be sanitized
	assert.Contains(t, output, "service not recognized", "Error should be sanitized for unknown service")
}

// Test_sanitizeError tests the error sanitization function with various error inputs.
// This is a pure function and can be tested directly with table-driven tests.
func Test_sanitizeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "unknown service error returns service not recognized",
			err:  errors.New("unknown service: xyz"),
			want: "service not recognized",
		},
		{
			name: "parse invalid URL error returns invalid URL format",
			err:  errors.New("parse \"://\": invalid URL"),
			want: "invalid URL format",
		},
		{
			name: "random error returns unable to process URL",
			err:  errors.New("some random error"),
			want: "unable to process URL",
		},
		{
			name: "parse error in query returns invalid URL format",
			err:  errors.New("parse error in query"),
			want: "invalid URL format",
		},
		{
			name: "error containing 'invalid' returns invalid URL format",
			err:  errors.New("invalid scheme provided"),
			want: "invalid URL format",
		},
		{
			name: "wrapped unknown service error returns service not recognized",
			err:  fmt.Errorf("locating service for URL: %w", errors.New("unknown service: test")),
			want: "service not recognized",
		},
		{
			name: "empty error returns unable to process URL",
			err:  errors.New(""),
			want: "unable to process URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := sanitizeError(tt.err)
			assert.Equal(t, tt.want, got, "sanitizeError() output mismatch")
		})
	}
}

// TestRun_CaptureStdout tests that successful verification outputs to stdout.
func TestRun_CaptureStdout(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		cmd.Flags().StringArrayP("url", "u", []string{}, "")

		_ = cmd.Flags().Set("url", "logger://")

		Run(cmd, []string{})

		return
	}

	// Run in subprocess
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_CaptureStdout")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	var stdoutBuf, stderrBuf bytes.Buffer

	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	// Should succeed
	require.NoError(t, err, "Valid URL verification should succeed")

	// Should output configuration to stdout
	stdout := stdoutBuf.String()
	assert.NotEmpty(t, stdout, "Successful verification should output configuration")
}

// TestRun_IntegrationWithExitCodes_ValidURL tests valid URL exits with success code.
func TestRun_IntegrationWithExitCodes_ValidURL(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		cmd.Flags().StringArrayP("url", "u", []string{}, "")

		_ = cmd.Flags().Set("url", "logger://")

		Run(cmd, []string{})

		return
	}

	// Run in subprocess
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_IntegrationWithExitCodes_ValidURL")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	err := cmd.Run()
	require.NoError(t, err, "Expected successful exit")
}

// TestRun_IntegrationWithExitCodes_InvalidURL tests invalid URL exits with error code.
func TestRun_IntegrationWithExitCodes_InvalidURL(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		cmd.Flags().StringArrayP("url", "u", []string{}, "")

		_ = cmd.Flags().Set("url", "://badurl")

		Run(cmd, []string{})

		return
	}

	// Run in subprocess
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_IntegrationWithExitCodes_InvalidURL")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		assert.Equal(t, 1, exitErr.ExitCode(), "Exit code mismatch")
	} else {
		t.Fatalf("Expected ExitError with code 1, got: %v", err)
	}
}

// TestRun_WithSimpleLoggerURL tests verification with simple logger URL.
func TestRun_WithSimpleLoggerURL(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		cmd.Flags().StringArrayP("url", "u", []string{}, "")

		_ = cmd.Flags().Set("url", "logger://")

		Run(cmd, []string{})

		return
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_WithSimpleLoggerURL")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	var outputBuf bytes.Buffer

	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	err := cmd.Run()
	output := outputBuf.String()

	require.NoError(t, err, "Valid logger URL should succeed")
	assert.NotEmpty(t, output, "Output should contain configuration")
}

// TestRun_WithLoggerURLWithPath tests verification with logger URL with path.
func TestRun_WithLoggerURLWithPath(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		cmd.Flags().StringArrayP("url", "u", []string{}, "")

		_ = cmd.Flags().Set("url", "logger://test/path")

		Run(cmd, []string{})

		return
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_WithLoggerURLWithPath")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	var outputBuf bytes.Buffer

	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	err := cmd.Run()
	output := outputBuf.String()

	require.NoError(t, err, "Valid logger URL with path should succeed")
	assert.NotEmpty(t, output, "Output should contain configuration")
}

// TestRun_StderrOutput tests that flag errors output to stderr.
func TestRun_StderrOutput(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		// Create a flag type mismatch to trigger stderr output
		cmd.Flags().BoolP("url", "u", false, "")

		Run(cmd, []string{})

		return
	}

	// Run in subprocess
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_StderrOutput")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	var stderrBuf bytes.Buffer

	cmd.Stderr = &stderrBuf
	cmd.Stdout = io.Discard

	err := cmd.Run()

	// Should fail
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		assert.Equal(t, 1, exitErr.ExitCode(), "Should exit with code 1")
	} else {
		t.Fatalf("Expected ExitError, got: %v", err)
	}

	// Error should be in stderr
	stderr := stderrBuf.String()
	assert.Contains(t, stderr, "Error getting URL flags", "Error message should be written to stderr")
}

// Test_sanitizeError_EdgeCases tests edge cases for error sanitization.
func Test_sanitizeError_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "error with both unknown service and parse",
			err:  errors.New("parse error: unknown service"),
			// Note: "unknown service" check comes before "parse" check
			want: "service not recognized",
		},
		{
			name: "error with invalid keyword",
			err:  errors.New("invalid token"),
			want: "invalid URL format",
		},
		{
			name: "error with parse keyword",
			err:  errors.New("failed to parse configuration"),
			want: "invalid URL format",
		},
		{
			name: "unknown service at beginning",
			err:  errors.New("unknown service xyz"),
			want: "service not recognized",
		},
		{
			name: "unknown service in middle",
			err:  errors.New("error: unknown service xyz not found"),
			want: "service not recognized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := sanitizeError(tt.err)
			assert.Equal(t, tt.want, got, "sanitizeError() output mismatch")
		})
	}
}

// TestCmd_Initialization tests that the Cmd is properly initialized.
func TestCmd_Initialization(t *testing.T) {
	t.Parallel()

	t.Run("verify command is properly initialized", func(t *testing.T) {
		t.Parallel()
		assert.NotNil(t, Cmd, "Cmd should be initialized")
		assert.Equal(t, "verify", Cmd.Use, "Command use should be 'verify'")
		assert.NotEmpty(t, Cmd.Short, "Command should have a short description")
		assert.NotNil(t, Cmd.Run, "Command should have a Run function")
		assert.NotNil(t, Cmd.PreRunE, "Command should have a PreRunE function")

		// Check that url flag exists
		urlFlag := Cmd.Flags().Lookup("url")
		assert.NotNil(t, urlFlag, "URL flag should exist")
		assert.Equal(t, "u", urlFlag.Shorthand, "URL flag should have shorthand 'u'")
	})
}

// TestRun_WithLoggerSchemeSucceeds tests verification with logger scheme succeeds.
func TestRun_WithLoggerSchemeSucceeds(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		cmd.Flags().StringArrayP("url", "u", []string{}, "")

		_ = cmd.Flags().Set("url", "logger://")

		Run(cmd, []string{})

		return
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_WithLoggerSchemeSucceeds")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	err := cmd.Run()
	require.NoError(t, err, "Expected success for valid scheme")
}

// TestRun_WithUnknownSchemeFails tests verification with unknown scheme fails.
func TestRun_WithUnknownSchemeFails(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		cmd := &cobra.Command{Use: "verify"}
		cmd.Flags().StringArrayP("url", "u", []string{}, "")

		_ = cmd.Flags().Set("url", "unknown123://test")

		Run(cmd, []string{})

		return
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRun_WithUnknownSchemeFails")

	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		assert.Equal(t, 1, exitErr.ExitCode(), "Expected exit code 1 for invalid scheme")
	} else {
		t.Fatalf("Expected ExitError, got: %v", err)
	}
}
