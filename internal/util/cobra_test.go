package util

import (
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// viperMu protects viper state during parallel test execution.
var viperMu sync.Mutex

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

			// Setup command before acquiring mutex (no viper dependency)
			cmd := setupTestCommand()

			// Set initial message flag if specified (no viper dependency)
			if tt.initialMessage != "" {
				err := cmd.Flags().Set("message", tt.initialMessage)
				require.NoError(t, err)
			}

			// Serialize all viper operations to prevent races between
			// parallel subtests that share viper's global state.
			viperMu.Lock()

			viper.Reset()

			if tt.envURL != "" {
				viper.Set("SHOUTRRR_URL", tt.envURL)
			}

			// Execute the function while holding the lock so viper state
			// (Reset + Set + read inside LoadFlagsFromAltSources) is atomic
			err := LoadFlagsFromAltSources(cmd, tt.args)

			viperMu.Unlock()

			// Check error expectation
			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				return
			}

			require.NoError(t, err)

			// Verify flag values (no viper dependency, safe outside lock)
			urls, err := cmd.Flags().GetStringArray("url")
			require.NoError(t, err)

			if tt.wantURL == "" {
				assert.Empty(t, urls, "URL flag mismatch")
			} else {
				require.Len(t, urls, 1, "URL flag mismatch")
				assert.Equal(t, tt.wantURL, urls[0], "URL flag mismatch")
			}

			message, err := cmd.Flags().GetString("message")
			require.NoError(t, err)
			assert.Equal(t, tt.wantMessage, message, "Message flag mismatch")
		})
	}
}

func TestLoadFlagsFromAltSources_URLFlagAlreadySet(t *testing.T) {
	t.Parallel()

	// When URL flag is already set, positional args should still override
	cmd := setupTestCommand()

	// Pre-set the URL flag (no viper dependency)
	err := cmd.Flags().Set("url", "https://preset.example.com")
	require.NoError(t, err)

	// Serialize viper access
	viperMu.Lock()
	viper.Reset()

	err = LoadFlagsFromAltSources(cmd, []string{"https://positional.example.com"})
	viperMu.Unlock()

	require.NoError(t, err)

	// Positional arg should be appended (StringArray.Set appends)
	urls, err := cmd.Flags().GetStringArray("url")
	require.NoError(t, err)
	assert.Contains(t, urls, "https://positional.example.com")
}

func TestLoadFlagsFromAltSources_EnvVarOverridesEmptyFlag(t *testing.T) {
	t.Parallel()

	// When URL flag is empty but env var is set, env var should be used
	cmd := setupTestCommand()

	// Serialize viper access
	viperMu.Lock()
	viper.Reset()
	viper.Set("SHOUTRRR_URL", "https://env.example.com")

	err := LoadFlagsFromAltSources(cmd, []string{})
	viperMu.Unlock()

	require.NoError(t, err)

	urls, err := cmd.Flags().GetStringArray("url")
	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.Equal(t, "https://env.example.com", urls[0])
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
				cmd.Flags().StringArray("url", []string{}, "The notification URL")

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

			cmd := tt.setupCmd()

			// Serialize viper access
			viperMu.Lock()
			viper.Reset()

			if tt.envURL != "" {
				viper.Set("SHOUTRRR_URL", tt.envURL)
			}

			err := LoadFlagsFromAltSources(cmd, tt.args)
			viperMu.Unlock()

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

			// Setup command with flag value (no viper dependency)
			cmd := setupTestCommand()

			if tt.flagURL != "" {
				err := cmd.Flags().Set("url", tt.flagURL)
				require.NoError(t, err)
			}

			// Serialize viper access
			viperMu.Lock()
			viper.Reset()

			if tt.envURL != "" {
				viper.Set("SHOUTRRR_URL", tt.envURL)
			}

			got, err := hasURLInEnvButNotFlag(cmd)
			viperMu.Unlock()

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
	cmd := &cobra.Command{
		Use: "test",
	}

	// Only add message flag, not url flag
	cmd.Flags().String("message", "", "The notification message")

	// Serialize viper access
	viperMu.Lock()
	viper.Reset()
	viper.Set("SHOUTRRR_URL", "https://example.com")

	got, err := hasURLInEnvButNotFlag(cmd)
	viperMu.Unlock()

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

	cmd.Flags().StringArray("url", []string{}, "The notification URL")
	cmd.Flags().String("message", "", "The notification message")

	return cmd
}
