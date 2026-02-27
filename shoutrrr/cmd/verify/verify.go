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

// Cmd verifies the validity of a service url.
var Cmd = &cobra.Command{
	Use:     "verify",
	Short:   "Verify the validity of a notification service URL",
	PreRunE: internalUtil.LoadFlagsFromAltSources,
	Run:     Run,
	Args:    cobra.MaximumNArgs(1),
}

var serviceRouter router.ServiceRouter

func init() {
	Cmd.Flags().StringP("url", "u", "", "The notification url")
	_ = Cmd.MarkFlagRequired("url")
}

// Run the verify command.
func Run(cmd *cobra.Command, _ []string) {
	URL, _ := cmd.Flags().GetString("url")
	serviceRouter = router.ServiceRouter{}

	service, err := serviceRouter.Locate(URL)
	if err != nil {
		wrappedErr := fmt.Errorf("locating service for URL: %w", err)
		_, _ = fmt.Fprint(os.Stdout, "error verifying URL: ", sanitizeError(wrappedErr), "\n")

		os.Exit(1)
	}

	config := format.GetServiceConfig(service)
	configNode := format.GetConfigFormat(config)

	cfg := color.DefaultConfig()
	_, _ = fmt.Fprint(cfg.Output, format.ColorFormatTree(configNode, true))
}

// sanitizeError removes sensitive details from an error message.
func sanitizeError(err error) string {
	errStr := err.Error()
	// Check for common error patterns without exposing URL details
	if strings.Contains(errStr, "unknown service") {
		return "service not recognized"
	}

	if strings.Contains(errStr, "parse") || strings.Contains(errStr, "invalid") {
		return "invalid URL format"
	}
	// Fallback for other errors
	return "unable to process URL"
}
