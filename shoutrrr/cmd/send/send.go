// Package send provides the CLI command for sending notifications to various notification services.
package send

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/shoutrrr/internal/dedupe"
	internalUtil "github.com/nicholas-fedor/shoutrrr/internal/util"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
	cli "github.com/nicholas-fedor/shoutrrr/shoutrrr/cmd"
)

// Command constants define the maximum number of arguments and message length limits.
const (
	MaximumNArgs     = 2
	MaxMessageLength = 100
)

// Cmd is the cobra command for sending notifications.
// It sends messages to one or more notification services using configured URLs.
var Cmd = &cobra.Command{
	Use:     "send",
	Short:   "Send a notification using a service url",
	Args:    cobra.MaximumNArgs(MaximumNArgs),
	PreRunE: internalUtil.LoadFlagsFromAltSources,
	RunE:    Run,
}

// init initializes the command flags for the send command.
func init() {
	Cmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")
	Cmd.Flags().StringArrayP("url", "u", []string{}, "The notification URL(s) to send to")

	if err := Cmd.MarkFlagRequired("url"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking URL flag as required: %v\n", err)
	}

	Cmd.Flags().
		StringP("message", "m", "", "The message to send to the notification url, or - to read message from stdin")

	if err := Cmd.MarkFlagRequired("message"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking message flag as required: %v\n", err)
	}

	Cmd.Flags().StringP("title", "t", "", "The title used for services that support it")
}

// Run executes the send command and handles its result.
// It processes errors appropriately, exiting with the correct exit code for non-usage errors.
//
// Parameters:
//   - cmd: The cobra command containing the parsed flags.
//   - _: Unused positional arguments.
//
// Returns:
//   - error: An error if the command fails, or nil on success.
func Run(cmd *cobra.Command, _ []string) error {
	err := run(cmd)
	if err != nil {
		var result cli.ExitError
		if errors.As(err, &result) && result.ExitCode != cli.ExUsage {
			// If the error is not related to CLI usage, report error and exit to avoid cobra error output.
			_, _ = fmt.Fprintln(os.Stderr, err.Error())

			os.Exit(result.ExitCode)
		}
	}

	return err
}

// logf prints a formatted message to stderr.
//
// Parameters:
//   - format: The format string.
//   - a: Variadic arguments for the format string.
func logf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

// run executes the core send logic.
// It retrieves flags, reads input, and sends notifications to configured services.
//
// Parameters:
//   - cmd: The cobra command containing the parsed flags.
//
// Returns:
//   - error: An error if the send operation fails, or nil on success.
func run(cmd *cobra.Command) error {
	flags := cmd.Flags()

	// Retrieve verbose flag.
	verbose, err := flags.GetBool("verbose")
	if err != nil {
		return fmt.Errorf("failed to get verbose flag: %w", err)
	}

	// Retrieve and deduplicate URLs.
	urls, err := flags.GetStringArray("url")
	if err != nil {
		return fmt.Errorf("failed to get url flag: %w", err)
	}

	urls = dedupe.RemoveDuplicates(urls)

	// Retrieve message flag.
	message, err := flags.GetString("message")
	if err != nil {
		return fmt.Errorf("failed to get message flag: %w", err)
	}

	// Retrieve title flag.
	title, err := flags.GetString("title")
	if err != nil {
		return fmt.Errorf("failed to get title flag: %w", err)
	}

	// Read message from stdin if requested.
	if message == "-" {
		logf("Reading from STDIN...")

		stringBuilder := strings.Builder{}

		count, err := io.Copy(&stringBuilder, os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read message from stdin: %w", err)
		}

		logf("Read %d byte(s)", count)

		message = stringBuilder.String()
	}

	// Setup logger based on verbose flag.
	var logger *log.Logger

	if verbose {
		urlsPrefix := "URLs:"
		for i, url := range urls {
			logf("%s %s", urlsPrefix, url)

			if i == 0 {
				// Only display "URLs:" prefix for first line, replace with indentation for subsequent lines.
				urlsPrefix = strings.Repeat(" ", len(urlsPrefix))
			}
		}

		logf("Message: %s", util.Ellipsis(message, MaxMessageLength))

		if title != "" {
			logf("Title: %v", title)
		}

		logger = log.New(os.Stderr, "SHOUTRRR ", log.LstdFlags)
	} else {
		logger = util.DiscardLogger
	}

	// Create service router and send notification.
	serviceRouter, err := router.New(logger, urls...)
	if err != nil {
		return cli.ConfigurationError(fmt.Sprintf("error invoking send: %s", err))
	}

	params := make(types.Params)
	if title != "" {
		params["title"] = title
	}

	errs := serviceRouter.SendAsync(message, &params)
	for err := range errs {
		if err != nil {
			return cli.TaskUnavailable(err.Error())
		}

		logf("Notification sent")
	}

	return nil
}
