// Package util provides utility functions for the shoutrrr CLI application.
package util

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// LoadFlagsFromAltSources is a workaround to make cobra count env vars and
// positional arguments when checking required flags.
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

		// If the URL has been set in ENV, default the message to read from stdin.
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

func hasURLInEnvButNotFlag(cmd *cobra.Command) (bool, error) {
	s, err := cmd.Flags().GetString("url")
	if err != nil {
		return false, fmt.Errorf("getting url flag value: %w", err)
	}

	return s == "" && viper.GetViper().GetString("SHOUTRRR_URL") != "", nil
}
