package util

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFlagsFromAltSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           []string
		envURL         string
		initialMessage string
		wantErr        bool
		wantURL        string
		wantMessage    string
		errContains    string
	}{
		{
			name:        "positional_args_url_only",
			args:        []string{"https://example.com/notify"},
			wantErr:     false,
			wantURL:     "https://example.com/notify",
			wantMessage: "",
		},
		{
			name:        "positional_args_url_and_message",
			args:        []string{"https://example.com/notify", "test message"},
			wantErr:     false,
			wantURL:     "https://example.com/notify",
			wantMessage: "test message",
		},
		{
			name:        "env_var_set_no_args",
			args:        []string{},
			envURL:      "https://env.example.com/notify",
			wantErr:     false,
			wantURL:     "https://env.example.com/notify",
			wantMessage: "-",
		},
		{
			name:           "env_var_set_with_existing_message_flag",
			args:           []string{},
			envURL:         "https://env.example.com/notify",
			initialMessage: "existing message",
			wantErr:        false,
			wantURL:        "https://env.example.com/notify",
			wantMessage:    "existing message",
		},
		{
			name:        "no_args_no_env",
			args:        []string{},
			envURL:      "",
			wantErr:     false,
			wantURL:     "",
			wantMessage: "",
		},
		{
			name:        "empty_args_slice",
			args:        nil,
			envURL:      "",
			wantErr:     false,
			wantURL:     "",
			wantMessage: "",
		},
		{
			name:    "multiple_positional_args_uses_first_two",
			args:    []string{"https://example.com/notify", "msg1", "extra"},
			wantErr: false,
			wantURL: "https://example.com/notify",
			// Only first two args are used
			wantMessage: "msg1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Reset viper state before each test
			viper.Reset()

			// Setup command
			cmd := setupTestCommand()

			// Set environment variable if specified
			if tt.envURL != "" {
				viper.Set("SHOUTRRR_URL", tt.envURL)
			}

			// Set initial message flag if specified
			if tt.initialMessage != "" {
				err := cmd.Flags().Set("message", tt.initialMessage)
				require.NoError(t, err)
			}

			// Execute the function
			err := LoadFlagsFromAltSources(cmd, tt.args)

			// Check error expectation
			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				return
			}

			require.NoError(t, err)

			// Verify flag values
			url, err := cmd.Flags().GetString("url")
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, url, "URL flag mismatch")

			message, err := cmd.Flags().GetString("message")
			require.NoError(t, err)
			assert.Equal(t, tt.wantMessage, message, "Message flag mismatch")
		})
	}
}

func TestLoadFlagsFromAltSources_URLFlagAlreadySet(t *testing.T) {
	t.Parallel()

	// When URL flag is already set, positional args should still override
	viper.Reset()

	cmd := setupTestCommand()

	// Pre-set the URL flag
	err := cmd.Flags().Set("url", "https://preset.example.com")
	require.NoError(t, err)

	// Call with positional arg
	err = LoadFlagsFromAltSources(cmd, []string{"https://positional.example.com"})
	require.NoError(t, err)

	// Positional arg should override
	url, err := cmd.Flags().GetString("url")
	require.NoError(t, err)
	assert.Equal(t, "https://positional.example.com", url)
}

func TestLoadFlagsFromAltSources_EnvVarOverridesEmptyFlag(t *testing.T) {
	t.Parallel()

	// When URL flag is empty but env var is set, env var should be used
	viper.Reset()

	cmd := setupTestCommand()

	viper.Set("SHOUTRRR_URL", "https://env.example.com")

	err := LoadFlagsFromAltSources(cmd, []string{})
	require.NoError(t, err)

	url, err := cmd.Flags().GetString("url")
	require.NoError(t, err)
	assert.Equal(t, "https://env.example.com", url)
}

func TestLoadFlagsFromAltSources_ErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupCmd    func() *cobra.Command
		args        []string
		envURL      string
		wantErr     bool
		errContains string
	}{
		{
			name: "error_url_flag_not_exists",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "test"}
				// Only add message flag, not url flag
				cmd.Flags().String("message", "", "The notification message")

				return cmd
			},
			args:        []string{},
			envURL:      "https://example.com",
			wantErr:     true,
			errContains: "checking url flag and env",
		},
		{
			name: "error_message_flag_not_exists_for_env_path",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "test"}
				// Only add url flag, not message flag
				cmd.Flags().String("url", "", "The notification URL")

				return cmd
			},
			args:        []string{},
			envURL:      "https://example.com",
			wantErr:     true,
			errContains: "getting message flag value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			viper.Reset()

			cmd := tt.setupCmd()

			if tt.envURL != "" {
				viper.Set("SHOUTRRR_URL", tt.envURL)
			}

			err := LoadFlagsFromAltSources(cmd, tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func Test_hasURLInEnvButNotFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		flagURL    string
		envURL     string
		want       bool
		wantErr    bool
		errContain string
	}{
		{
			name:    "flag_empty_env_set",
			flagURL: "",
			envURL:  "https://example.com",
			want:    true,
			wantErr: false,
		},
		{
			name:    "flag_set_env_empty",
			flagURL: "https://example.com",
			envURL:  "",
			want:    false,
			wantErr: false,
		},
		{
			name:    "both_empty",
			flagURL: "",
			envURL:  "",
			want:    false,
			wantErr: false,
		},
		{
			name:    "both_set",
			flagURL: "https://flag.example.com",
			envURL:  "https://env.example.com",
			want:    false,
			wantErr: false,
		},
		{
			name:    "flag_set_env_set_different",
			flagURL: "https://example.com",
			envURL:  "https://other.example.com",
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Reset viper state
			viper.Reset()

			// Setup command with flag value
			cmd := setupTestCommand()

			if tt.flagURL != "" {
				err := cmd.Flags().Set("url", tt.flagURL)
				require.NoError(t, err)
			}

			if tt.envURL != "" {
				viper.Set("SHOUTRRR_URL", tt.envURL)
			}

			got, err := hasURLInEnvButNotFlag(cmd)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_hasURLInEnvButNotFlag_MissingFlag(t *testing.T) {
	t.Parallel()

	// Test when the url flag doesn't exist on the command
	viper.Reset()

	cmd := &cobra.Command{
		Use: "test",
	}

	// Only add message flag, not url flag
	cmd.Flags().String("message", "", "The notification message")

	viper.Set("SHOUTRRR_URL", "https://example.com")

	got, err := hasURLInEnvButNotFlag(cmd)
	// This should return an error because the flag doesn't exist
	require.Error(t, err)
	assert.False(t, got)
	assert.Contains(t, err.Error(), "getting url flag value")
}

// setupTestCommand creates a cobra command with url and message flags for testing.
func setupTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "test",
	}

	cmd.Flags().String("url", "", "The notification URL")
	cmd.Flags().String("message", "", "The notification message")

	return cmd
}
