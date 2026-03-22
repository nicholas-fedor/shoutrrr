// Package util provides utility functions for the shoutrrr CLI application.
package util

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// LoadFlagsFromAltSources is a workaround to make cobra count env vars and
// positional arguments when checking required flags.
//
// It resolves the url and message flags from positional arguments (if provided)
// or from the SHOUTRRR_URL environment variable. When the URL is sourced from
// the environment, the message defaults to stdin ("-") unless already set.
//
// Parameters:
//   - cmd: the cobra.Command whose flags will be populated.
//   - args: positional arguments passed to the command.
//
// Returns:
//   - error: if any flag operation fails; otherwise nil.
func LoadFlagsFromAltSources(cmd *cobra.Command, args []string) error {
	flags := cmd.Flags()

	if len(args) > 0 {
		if err := flags.Set("url", args[0]); err != nil {
			return fmt.Errorf("setting url flag from positional arg: %w", err)
		}

		if len(args) > 1 {
			if err := flags.Set("message", args[1]); err != nil {
				return fmt.Errorf("setting message flag from positional arg: %w", err)
			}
		}

		return nil
	}

	hasURL, err := hasURLInEnvButNotFlag(cmd)
	if err != nil {
		return fmt.Errorf("checking url flag and env: %w", err)
	}

	if hasURL {
		if err := flags.Set("url", viper.GetViper().GetString("SHOUTRRR_URL")); err != nil {
			return fmt.Errorf("setting url flag from env var: %w", err)
		}

		// Default the message to read from stdin when the URL is sourced from env.
		msg, err := flags.GetString("message")
		if err != nil {
			return fmt.Errorf("getting message flag value: %w", err)
		}

		if msg == "" {
			if err := flags.Set("message", "-"); err != nil {
				return fmt.Errorf("setting message flag to default stdin: %w", err)
			}
		}
	}

	return nil
}

// hasURLInEnvButNotFlag checks whether the SHOUTRRR_URL environment variable is
// set while no url flag has been explicitly provided on the command.
//
// Cobra StringArray flags default to [""] rather than a truly empty slice, so
// both cases are treated as "no URL provided via flag".
//
// Parameters:
//   - cmd: the cobra.Command to inspect.
//
// Returns:
//   - bool: true when the env var is set and the flag is empty.
//   - error: if the url flag cannot be read.
func hasURLInEnvButNotFlag(cmd *cobra.Command) (bool, error) {
	urls, err := cmd.Flags().GetStringArray("url")
	if err != nil {
		return false, fmt.Errorf("getting url flag value: %w", err)
	}

	// Treat an empty slice or a single-element slice containing "" as "no URL provided".
	flagEmpty := len(urls) == 0 || (len(urls) == 1 && urls[0] == "")

	return flagEmpty && viper.GetViper().GetString("SHOUTRRR_URL") != "", nil
}
