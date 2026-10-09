package util

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadFlagsFromAltSourcesPositionalArgs verifies that positional arguments
// fill the url and then the message flag, skip flags set on the command line, and
// are rejected when no flag is left to fill.
func TestLoadFlagsFromAltSourcesPositionalArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		urlFlag     string
		args        []string
		wantURL     []string
		wantMessage string
		wantErr     error
	}{
		{
			name:    "url only",
			args:    []string{"logger://"},
			wantURL: []string{"logger://"},
		},
		{
			name:        "url and message",
			args:        []string{"logger://", "hello"},
			wantURL:     []string{"logger://"},
			wantMessage: "hello",
		},
		{
			name:        "an argument after a url flag is the message",
			urlFlag:     "logger://flag",
			args:        []string{"hello"},
			wantURL:     []string{"logger://flag"},
			wantMessage: "hello",
		},
		{
			name:    "an argument with no flag left to fill",
			urlFlag: "logger://flag",
			args:    []string{"hello", "extra"},
			wantErr: ErrTooManyArgs,
		},
		{
			name:    "no arguments",
			wantURL: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := newSendCommand()
			if tt.urlFlag != "" {
				require.NoError(t, cmd.Flags().Set("url", tt.urlFlag))
			}

			err := LoadFlagsFromAltSources(cmd, tt.args)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assertFlags(t, cmd, tt.wantURL, tt.wantMessage)
		})
	}
}

// TestLoadFlagsFromAltSourcesEnvironment verifies how SHOUTRRR_URL and
// SHOUTRRR_MESSAGE fill the send command's flags, including the stdin default
// for a message when the URL comes from the environment.
func TestLoadFlagsFromAltSourcesEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		urlFlag     string
		args        []string
		wantURL     []string
		wantMessage string
	}{
		{
			name:        "url from the environment reads the message from stdin",
			env:         map[string]string{"SHOUTRRR_URL": "logger://env"},
			wantURL:     []string{"logger://env"},
			wantMessage: "-",
		},
		{
			name:        "message from the environment",
			env:         map[string]string{"SHOUTRRR_URL": "logger://env", "SHOUTRRR_MESSAGE": "from env"},
			wantURL:     []string{"logger://env"},
			wantMessage: "from env",
		},
		{
			name:        "a positional message takes precedence over the environment",
			env:         map[string]string{"SHOUTRRR_URL": "logger://env", "SHOUTRRR_MESSAGE": "from env"},
			args:        []string{"logger://arg", "from args"},
			wantURL:     []string{"logger://arg"},
			wantMessage: "from args",
		},
		{
			name:    "a url flag takes precedence over the environment",
			env:     map[string]string{"SHOUTRRR_URL": "logger://env"},
			urlFlag: "logger://flag",
			wantURL: []string{"logger://flag"},
		},
		{
			name:    "an empty variable counts as unset",
			env:     map[string]string{"SHOUTRRR_URL": ""},
			wantURL: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			cmd := newSendCommand()
			if tt.urlFlag != "" {
				require.NoError(t, cmd.Flags().Set("url", tt.urlFlag))
			}

			require.NoError(t, LoadFlagsFromAltSources(cmd, tt.args))
			assertFlags(t, cmd, tt.wantURL, tt.wantMessage)
		})
	}
}

// TestLoadFlagsFromAltSourcesWithoutMessageFlag verifies that a command without a
// message flag, such as verify, accepts a URL from the environment.
func TestLoadFlagsFromAltSourcesWithoutMessageFlag(t *testing.T) {
	t.Setenv("SHOUTRRR_URL", "logger://env")

	cmd := &cobra.Command{Use: "verify"}
	cmd.Flags().StringArrayP("url", "u", []string{}, "The notification URL(s) to verify")

	require.NoError(t, LoadFlagsFromAltSources(cmd, nil))

	urls, err := cmd.Flags().GetStringArray("url")
	require.NoError(t, err)
	assert.Equal(t, []string{"logger://env"}, urls)
}

// TestApplyEnv verifies that ApplyEnv maps flag names to SHOUTRRR_ variables,
// parses typed flags, ignores the help flag, and names the variable in errors.
func TestApplyEnv(t *testing.T) {
	t.Run("typed and dashed flags", func(t *testing.T) {
		t.Setenv("SHOUTRRR_VERBOSE", "true")
		t.Setenv("SHOUTRRR_SHOW_SENSITIVE", "true")
		t.Setenv("SHOUTRRR_HELP", "true")

		cmd := newSendCommand()
		cmd.Flags().Bool("show-sensitive", false, "Show sensitive data")
		cmd.Flags().Bool("help", false, "Help")

		require.NoError(t, ApplyEnv(cmd))

		for name, want := range map[string]bool{"verbose": true, "show-sensitive": true, "help": false} {
			got, err := cmd.Flags().GetBool(name)
			require.NoError(t, err)
			assert.Equal(t, want, got, name)
		}
	})

	t.Run("a value the flag rejects", func(t *testing.T) {
		t.Setenv("SHOUTRRR_VERBOSE", "loud")

		err := ApplyEnv(newSendCommand())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "SHOUTRRR_VERBOSE")
	})
}

// TestApplyArgs verifies that ApplyArgs fills the named flags in order, skips set
// and undefined flags, and returns the arguments left over.
func TestApplyArgs(t *testing.T) {
	t.Parallel()

	cmd := newSendCommand()
	require.NoError(t, cmd.Flags().Set("url", "logger://flag"))

	rest, err := ApplyArgs(cmd, []string{"hello", "extra"}, "missing", "url", "message")
	require.NoError(t, err)

	assert.Equal(t, []string{"extra"}, rest)
	assertFlags(t, cmd, []string{"logger://flag"}, "hello")
}

// TestEnvName verifies the environment variable name for a flag.
func TestEnvName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "SHOUTRRR_URL", EnvName("url"))
	assert.Equal(t, "SHOUTRRR_SHOW_SENSITIVE", EnvName("show-sensitive"))
}

// newSendCommand returns a command with the send command's url, message and
// verbose flags, so tests do not share flag state.
//
// Returns:
//   - *cobra.Command: a fresh command.
func newSendCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "send"}
	cmd.Flags().StringArrayP("url", "u", []string{}, "The notification URL(s) to send to")
	cmd.Flags().StringP("message", "m", "", "The message to send")
	cmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")

	return cmd
}

// assertFlags checks the url and message flags of cmd.
//
// Parameters:
//   - t: the test.
//   - cmd: the command to check.
//   - wantURL: the expected url flag values.
//   - wantMessage: the expected message flag value.
func assertFlags(t *testing.T, cmd *cobra.Command, wantURL []string, wantMessage string) {
	t.Helper()

	urls, err := cmd.Flags().GetStringArray("url")
	require.NoError(t, err)
	assert.Equal(t, wantURL, urls)

	message, err := cmd.Flags().GetString("message")
	require.NoError(t, err)
	assert.Equal(t, wantMessage, message)
}
