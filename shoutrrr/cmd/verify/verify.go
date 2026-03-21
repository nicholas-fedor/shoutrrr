// Package verify provides the CLI command for verifying notification service URLs.
package verify

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	internalUtil "github.com/nicholas-fedor/shoutrrr/internal/util"
	"github.com/nicholas-fedor/shoutrrr/pkg/color"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
)

var (
	// Cmd is the cobra command for verifying notification service URLs.
	// It validates that a URL is properly formatted and the service can be located.
	Cmd = &cobra.Command{
		Use:     "verify",
		Short:   "Verify the validity of a notification service URL",
		PreRunE: internalUtil.LoadFlagsFromAltSources,
		Run:     Run,
		Args:    cobra.MaximumNArgs(1),
	}

	// serviceRouter manages service lookup and initialization.
	serviceRouter router.ServiceRouter
)

// init initializes the command flags for the verify command.
func init() {
	Cmd.Flags().StringArrayP("url", "u", []string{}, "The notification URL(s) to verify")

	if err := Cmd.MarkFlagRequired("url"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking URL flag as required: %v\n", err)
	}
}

// Run executes the verify command, validating the specified notification URL.
// It locates the service, initializes it, and displays the parsed configuration.
//
// Parameters:
//   - cmd: The cobra command containing the parsed flags.
//   - _: Unused positional arguments.
func Run(cmd *cobra.Command, _ []string) {
	// Retrieve the URL flag.
	urls, err := cmd.Flags().GetStringArray("url")

	URL := ""
	if len(urls) > 0 {
		URL = urls[0]
	}

	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, "Error getting URL flags: ", err, "\n")

		os.Exit(1)
	}

	// Initialize the service router with default timeout (0 = no timeout).
	serviceRouter = router.ServiceRouter{
		Timeout: 0,
	}

	// Locate the service for the provided URL.
	service, err := serviceRouter.Locate(URL)
	if err != nil {
		wrappedErr := fmt.Errorf("locating service for URL: %w", err)
		_, _ = fmt.Fprint(os.Stdout, "error verifying URL: ", sanitizeError(wrappedErr), "\n")

		os.Exit(1)
	}

	// Retrieve and display the service configuration.
	config := format.GetServiceConfig(service)
	configNode := format.GetConfigFormat(config)

	cfg := color.DefaultConfig()
	_, _ = fmt.Fprint(cfg.Output, format.ColorFormatTree(configNode, true))
}

// sanitizeError removes sensitive details from an error message.
// It replaces specific error patterns with generic messages to avoid leaking URL details.
//
// Parameters:
//   - err: The error to sanitize.
//
// Returns:
//   - string: A sanitized error message safe for display.
func sanitizeError(err error) string {
	errStr := err.Error()

	// Check for common error patterns without exposing URL details.
	if strings.Contains(errStr, "unknown service") {
		return "service not recognized"
	}

	if strings.Contains(errStr, "parse") || strings.Contains(errStr, "invalid") {
		return "invalid URL format"
	}

	// Fallback for other errors.
	return "unable to process URL"
}
