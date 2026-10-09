// Package util provides utility functions for the shoutrrr CLI application.
package util

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// EnvPrefix is the prefix of the environment variables that set CLI flags. A flag
// named show-sensitive is read from SHOUTRRR_SHOW_SENSITIVE.
const EnvPrefix = "SHOUTRRR"

// stdinMessage is the message flag value that reads the message from stdin.
const stdinMessage = "-"

// ErrTooManyArgs indicates more positional arguments than flags left to fill.
var ErrTooManyArgs = errors.New("too many positional arguments")

// LoadFlagsFromAltSources fills the url and message flags of send and verify from
// positional arguments and environment variables, before cobra checks required
// flags.
//
// Positional arguments fill the url and then the message flag, skipping flags set
// on the command line. Environment variables then fill any flag that is still
// unset, as described in [ApplyEnv]. When the URL comes from the environment and
// the command has a message flag that is still empty, the message is read from
// stdin.
//
// Parameters:
//   - cmd: the command whose flags are filled.
//   - args: the positional arguments.
//
// Returns:
//   - error: [ErrTooManyArgs] when an argument has no flag left to fill, or an
//     error naming the flag or environment variable that could not be applied.
func LoadFlagsFromAltSources(cmd *cobra.Command, args []string) error {
	rest, err := ApplyArgs(cmd, args, "url", "message")
	if err != nil {
		return err
	}

	if len(rest) > 0 {
		return fmt.Errorf("%w: %q", ErrTooManyArgs, rest)
	}

	urlFromEnv := !cmd.Flags().Changed("url")

	if err := ApplyEnv(cmd); err != nil {
		return err
	}

	urlFromEnv = urlFromEnv && cmd.Flags().Changed("url")

	message := cmd.Flags().Lookup("message")
	if urlFromEnv && message != nil && !message.Changed {
		if err := cmd.Flags().Set("message", stdinMessage); err != nil {
			return fmt.Errorf("setting message flag to read stdin: %w", err)
		}
	}

	return nil
}

// ApplyArgs fills flags from positional arguments, in the order of names. A flag
// set on the command line, or one the command does not define, is skipped, so an
// argument fills the next flag that is still unset.
//
// Parameters:
//   - cmd: the command whose flags are filled.
//   - args: the positional arguments.
//   - names: the flags that positional arguments fill, in order.
//
// Returns:
//   - []string: the arguments left over after every named flag is filled.
//   - error: an error naming the flag that could not be set.
func ApplyArgs(cmd *cobra.Command, args []string, names ...string) ([]string, error) {
	flags := cmd.Flags()

	for _, name := range names {
		if len(args) == 0 {
			break
		}

		flag := flags.Lookup(name)
		if flag == nil || flag.Changed {
			continue
		}

		if err := flags.Set(name, args[0]); err != nil {
			return nil, fmt.Errorf("setting %s flag from positional argument: %w", name, err)
		}

		args = args[1:]
	}

	return args, nil
}

// ApplyEnv fills every flag that is still unset from its environment variable.
//
// The variable name is [EnvPrefix], an underscore, and the flag name in upper
// case with dashes replaced by underscores, so --url is read from SHOUTRRR_URL. A
// flag set on the command line or by a positional argument keeps its value, and
// an empty variable counts as unset. An array flag takes a single value from its
// variable. The help flag is never read from the environment.
//
// Parameters:
//   - cmd: the command whose flags are filled.
//
// Returns:
//   - error: an error naming the environment variable whose value the flag rejected.
func ApplyEnv(cmd *cobra.Command) error {
	env := viper.New()
	env.SetEnvPrefix(EnvPrefix)
	env.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	env.AutomaticEnv()

	var applyErr error

	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if applyErr != nil || flag.Changed || flag.Name == "help" || !env.IsSet(flag.Name) {
			return
		}

		if err := cmd.Flags().Set(flag.Name, env.GetString(flag.Name)); err != nil {
			applyErr = fmt.Errorf("applying %s to the %s flag: %w", EnvName(flag.Name), flag.Name, err)
		}
	})

	return applyErr
}

// EnvName returns the environment variable that sets a flag.
//
// Parameters:
//   - flagName: the flag name, such as show-sensitive.
//
// Returns:
//   - string: the variable name, such as SHOUTRRR_SHOW_SENSITIVE.
func EnvName(flagName string) string {
	return EnvPrefix + "_" + strings.ToUpper(strings.ReplaceAll(flagName, "-", "_"))
}
