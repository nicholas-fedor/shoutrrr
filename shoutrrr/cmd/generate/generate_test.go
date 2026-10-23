package generate

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/pushover"
)

// Test_applySources verifies that positional arguments fill the service and then
// the generator flag, skip flags set on the command line, and leave the remaining
// arguments for the generator.
func Test_applySources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		flags       map[string]string
		args        []string
		wantService string
		wantGen     string
		wantRest    []string
	}{
		{
			name:        "service only",
			args:        []string{"discord"},
			wantService: "discord",
			wantGen:     "basic",
		},
		{
			name:        "service and generator",
			args:        []string{"smtp", "oauth2"},
			wantService: "smtp",
			wantGen:     "oauth2",
		},
		{
			name:        "generator arguments after the generator",
			args:        []string{"smtp", "oauth2", "credentials.json"},
			wantService: "smtp",
			wantGen:     "oauth2",
			wantRest:    []string{"credentials.json"},
		},
		{
			name:        "flags set on the command line keep their values",
			flags:       map[string]string{"service": "smtp", "generator": "oauth2"},
			args:        []string{"credentials.json"},
			wantService: "smtp",
			wantGen:     "oauth2",
			wantRest:    []string{"credentials.json"},
		},
		{
			name:        "an argument fills the next unset flag",
			flags:       map[string]string{"service": "smtp"},
			args:        []string{"oauth2"},
			wantService: "smtp",
			wantGen:     "oauth2",
		},
		{
			name:    "no arguments",
			wantGen: "basic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := newTestCommand()

			for name, value := range tt.flags {
				require.NoError(t, cmd.Flags().Set(name, value))
			}

			rest, err := applySources(cmd, tt.args)
			require.NoError(t, err)

			service, err := cmd.Flags().GetString("service")
			require.NoError(t, err)

			generator, err := cmd.Flags().GetString("generator")
			require.NoError(t, err)

			assert.Equal(t, tt.wantService, service)
			assert.Equal(t, tt.wantGen, generator)

			if len(tt.wantRest) == 0 {
				assert.Empty(t, rest)
			} else {
				assert.Equal(t, tt.wantRest, rest)
			}
		})
	}
}

// Test_applySourcesReadsEnvironment verifies that environment variables fill the
// flags that neither the command line nor a positional argument set.
func Test_applySourcesReadsEnvironment(t *testing.T) {
	t.Setenv("SHOUTRRR_SERVICE", "telegram")
	t.Setenv("SHOUTRRR_GENERATOR", "basic")
	t.Setenv("SHOUTRRR_SHOW_SENSITIVE", "true")

	cmd := newTestCommand()

	rest, err := applySources(cmd, []string{"discord"})
	require.NoError(t, err)
	assert.Empty(t, rest)

	service, err := cmd.Flags().GetString("service")
	require.NoError(t, err)
	assert.Equal(t, "discord", service, "a positional argument takes precedence over the environment")

	showSensitive, err := cmd.Flags().GetBool("show-sensitive")
	require.NoError(t, err)
	assert.True(t, showSensitive)
}

// newTestCommand returns a command with the generate command's flags, so tests do
// not share the flag state of the package-level command.
//
// Returns:
//   - *cobra.Command: a fresh command with the service, generator, property and
//     show-sensitive flags.
func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "generate"}
	cmd.Flags().StringP("service", "s", "", "Notification service")
	cmd.Flags().StringP("generator", "g", "basic", "Generator to use")
	cmd.Flags().StringArrayP("property", "p", []string{}, "Configuration property")
	cmd.Flags().BoolP("show-sensitive", "x", false, "Show sensitive data")

	return cmd
}

// Test_displayURL verifies that a generated URL masks each secret, here the
// Pushover token, user key, and encryption key, unless secrets are shown.
func Test_displayURL(t *testing.T) {
	t.Parallel()

	cfg := &pushover.Config{
		Token:         "secret-api-token",
		User:          "secret-user-key",
		EncryptionKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	secrets := []string{cfg.Token, cfg.User, cfg.EncryptionKey}

	masked := displayURL(cfg, false)
	for _, secret := range secrets {
		assert.NotContains(t, masked, secret)
	}

	assert.Contains(t, masked, format.MaskedValue)

	shown := displayURL(cfg, true)
	for _, secret := range secrets {
		assert.Contains(t, shown, secret)
	}
}
