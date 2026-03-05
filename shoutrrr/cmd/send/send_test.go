package send

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cli "github.com/nicholas-fedor/shoutrrr/shoutrrr/cmd"
)

// TestRun tests the Run function with successful scenarios only.
// Note: Error cases that trigger os.Exit are tested via the internal run() function.
func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setupCmd func() *cobra.Command
		wantErr  bool
	}{
		{
			name: "successful send with logger URL and message",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", false, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("url", "logger://")
				_ = cmd.Flags().Set("message", "test message")

				return cmd
			},
			wantErr: false,
		},
		{
			name: "successful send with title parameter",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", false, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("url", "logger://")
				_ = cmd.Flags().Set("message", "test message")
				_ = cmd.Flags().Set("title", "Test Title")

				return cmd
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := tt.setupCmd()
			err := Run(cmd, []string{})

			if tt.wantErr {
				require.Error(t, err, "Run() should return an error")
			} else {
				assert.NoError(t, err, "Run() should not return an error")
			}
		})
	}
}

// Test_logf tests the logf function with various format strings.
//
//nolint:paralleltest // Test manipulates global os.Stderr, cannot run in parallel
func Test_logf(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		args       []any
		wantOutput string
	}{
		{
			name:       "simple message outputs to stderr",
			format:     "hello",
			args:       nil,
			wantOutput: "hello\n",
		},
		{
			name:       "formatted message with integer",
			format:     "count: %d",
			args:       []any{5},
			wantOutput: "count: 5\n",
		},
		{
			name:       "formatted message with string",
			format:     "name: %s",
			args:       []any{"test"},
			wantOutput: "name: test\n",
		},
		{
			name:       "empty format outputs newline",
			format:     "",
			args:       nil,
			wantOutput: "\n",
		},
		{
			name:       "multiple format arguments",
			format:     "%s: %d",
			args:       []any{"value", 42},
			wantOutput: "value: 42\n",
		},
		{
			name:       "format with percent sign",
			format:     "progress: %d%%",
			args:       []any{75},
			wantOutput: "progress: 75%\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stderr
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			require.NoError(t, err, "Failed to create pipe")

			os.Stderr = w

			// Call logf
			logf(tt.format, tt.args...)

			// Restore stderr and close write end
			w.Close()

			os.Stderr = oldStderr

			// Read captured output
			var buf bytes.Buffer

			_, err = io.Copy(&buf, r)
			require.NoError(t, err, "Failed to read captured output")
			r.Close()

			assert.Equal(t, tt.wantOutput, buf.String(), "logf() output mismatch")
		})
	}
}

// Test_run tests the internal run function with comprehensive scenarios.
// This tests both success and error cases since it doesn't trigger os.Exit.
//
//nolint:paralleltest // Test manipulates global os.Stderr, cannot run in parallel
func Test_run(t *testing.T) {
	tests := []struct {
		name           string
		setupCmd       func() *cobra.Command
		setupStdin     func() func()
		wantErr        bool
		wantErrType    string
		wantErrMessage string
		verifyOutput   func(t *testing.T, output string)
	}{
		{
			name: "basic send with logger URL returns success",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", false, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("url", "logger://")
				_ = cmd.Flags().Set("message", "test message")

				return cmd
			},
			wantErr: false,
		},
		{
			name: "verbose flag logs URLs and message",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", false, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("verbose", "true")
				_ = cmd.Flags().Set("url", "logger://")
				_ = cmd.Flags().Set("message", "verbose test message")

				return cmd
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "URLs:", "Verbose output should contain URLs")
				assert.Contains(t, output, "Message:", "Verbose output should contain Message")
				assert.Contains(t, output, "logger://", "Verbose output should contain the URL")
			},
		},
		{
			name: "stdin message reads from piped input",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", false, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("url", "logger://")
				_ = cmd.Flags().Set("message", "-")

				return cmd
			},
			setupStdin: func() func() {
				oldStdin := os.Stdin
				r, w, _ := os.Pipe()
				os.Stdin = r

				// Write test input
				go func() {
					_, _ = w.WriteString("stdin test message")
					w.Close()
				}()

				return func() {
					os.Stdin = oldStdin

					r.Close()
				}
			},
			wantErr: false,
		},
		{
			name: "URL deduplication removes duplicate URLs",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", true, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				// Set same URL twice
				_ = cmd.Flags().Set("url", "logger://")
				_ = cmd.Flags().Set("url", "logger://")
				_ = cmd.Flags().Set("message", "dedupe test")

				return cmd
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, output string) {
				t.Helper()
				// Should only log the URL once due to deduplication
				count := 0

				for i := range len(output) - len("logger://") {
					if output[i:i+len("logger://")] == "logger://" {
						count++
					}
				}

				assert.Equal(t, 1, count, "URL should appear exactly once after deduplication")
			},
		},
		{
			name: "flag retrieval error for verbose",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				// Create a flag that will cause type mismatch error
				cmd.Flags().StringP("verbose", "v", "not-a-bool", "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("url", "logger://")
				_ = cmd.Flags().Set("message", "test")

				return cmd
			},
			wantErr:        true,
			wantErrMessage: "failed to get verbose flag",
		},
		{
			name: "flag retrieval error for url",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", false, "")
				// Create url flag with wrong type
				cmd.Flags().BoolP("url", "u", false, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("message", "test")

				return cmd
			},
			wantErr:        true,
			wantErrMessage: "failed to get url flag",
		},
		{
			name: "flag retrieval error for message",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", false, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				// Create message flag with wrong type
				cmd.Flags().BoolP("message", "m", false, "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("url", "logger://")

				return cmd
			},
			wantErr:        true,
			wantErrMessage: "failed to get message flag",
		},
		{
			name: "flag retrieval error for title",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", false, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				// Create title flag with wrong type
				cmd.Flags().BoolP("title", "t", false, "")

				_ = cmd.Flags().Set("url", "logger://")
				_ = cmd.Flags().Set("message", "test")

				return cmd
			},
			wantErr:        true,
			wantErrMessage: "failed to get title flag",
		},
		{
			name: "configuration error with invalid URL",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", false, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("url", "://invalid")
				_ = cmd.Flags().Set("message", "test")

				return cmd
			},
			wantErr:        true,
			wantErrType:    "ConfigurationError",
			wantErrMessage: "error invoking send",
		},
		{
			name: "unknown service scheme returns configuration error",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", false, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("url", "unknownservice://test")
				_ = cmd.Flags().Set("message", "test")

				return cmd
			},
			wantErr:        true,
			wantErrType:    "ConfigurationError",
			wantErrMessage: "error invoking send",
		},
		{
			name: "multiple different URLs work correctly",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", true, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("url", "logger://first")
				_ = cmd.Flags().Set("url", "logger://second")
				_ = cmd.Flags().Set("message", "multi url test")

				return cmd
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "logger://first", "Output should contain first URL")
				assert.Contains(t, output, "logger://second", "Output should contain second URL")
			},
		},
		{
			name: "verbose with title logs title",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", true, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("url", "logger://")
				_ = cmd.Flags().Set("message", "title test")
				_ = cmd.Flags().Set("title", "My Test Title")

				return cmd
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "Title:", "Verbose output should contain Title")
				assert.Contains(t, output, "My Test Title", "Verbose output should contain the title value")
			},
		},
		{
			name: "long message is truncated in verbose output",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "send"}
				cmd.Flags().BoolP("verbose", "v", true, "")
				cmd.Flags().StringArrayP("url", "u", []string{}, "")
				cmd.Flags().StringP("message", "m", "", "")
				cmd.Flags().StringP("title", "t", "", "")

				_ = cmd.Flags().Set("url", "logger://")
				// Message longer than MaxMessageLength (100)
				longMsg := make([]byte, 150)
				for i := range longMsg {
					longMsg[i] = 'A'
				}

				_ = cmd.Flags().Set("message", string(longMsg))

				return cmd
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "Message:", "Verbose output should contain Message")
				assert.Contains(t, output, "[...]", "Long message should be truncated with ellipsis")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup stdin mock if needed
			var cleanupStdin func()
			if tt.setupStdin != nil {
				cleanupStdin = tt.setupStdin()
				defer cleanupStdin()
			}

			// Capture stderr for output verification
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			require.NoError(t, err, "Failed to create pipe")

			os.Stderr = w

			cmd := tt.setupCmd()
			runErr := run(cmd)

			// Restore stderr and read output
			w.Close()

			os.Stderr = oldStderr

			var outputBuf bytes.Buffer

			_, _ = io.Copy(&outputBuf, r)
			r.Close()

			output := outputBuf.String()

			if tt.wantErr {
				require.Error(t, runErr, "run() should return an error")

				if tt.wantErrMessage != "" {
					assert.Contains(t, runErr.Error(), tt.wantErrMessage, "Error message mismatch")
				}

				if tt.wantErrType != "" {
					var exitErr cli.ExitError
					if errors.As(runErr, &exitErr) {
						switch tt.wantErrType {
						case "ConfigurationError":
							assert.Equal(t, cli.ExConfig, exitErr.ExitCode, "Exit code should be ExConfig")
						case "TaskUnavailable":
							assert.Equal(t, cli.ExUnavailable, exitErr.ExitCode, "Exit code should be ExUnavailable")
						}
					} else {
						t.Errorf("Expected ExitError, got %T: %v", runErr, runErr)
					}
				}
			} else {
				require.NoError(t, runErr, "run() should not return an error")
				// Verify "Notification sent" message was logged
				assert.Contains(t, output, "Notification sent", "Success should log 'Notification sent'")

				if tt.verifyOutput != nil {
					tt.verifyOutput(t, output)
				}
			}
		})
	}
}

// Test_run_stdinError tests stdin reading error scenarios.
//
//nolint:paralleltest // Test manipulates global os.Stdin, cannot run in parallel
func Test_run_stdinError(t *testing.T) {
	// Test stdin read error by using a pipe that gets closed
	t.Run("stdin read error returns wrapped error", func(t *testing.T) {
		oldStdin := os.Stdin
		r, w, err := os.Pipe()
		require.NoError(t, err)

		os.Stdin = r

		// Close the write end immediately to cause read error
		w.Close()
		// Also close the read end to ensure read fails
		r.Close()

		defer func() {
			os.Stdin = oldStdin
		}()

		cmd := &cobra.Command{Use: "send"}
		cmd.Flags().BoolP("verbose", "v", false, "")
		cmd.Flags().StringArrayP("url", "u", []string{}, "")
		cmd.Flags().StringP("message", "m", "", "")
		cmd.Flags().StringP("title", "t", "", "")

		_ = cmd.Flags().Set("url", "logger://")
		_ = cmd.Flags().Set("message", "-")

		runErr := run(cmd)
		require.Error(t, runErr, "run() should return an error for stdin read failure")
		assert.Contains(t, runErr.Error(), "failed to read message from stdin", "Error should indicate stdin read failure")
	})
}
