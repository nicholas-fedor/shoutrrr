// Package docs provides the "docs" CLI command for generating documentation
// for Shoutrrr notification services.
package docs

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/shoutrrr/cmd"
)

var (
	// serviceRouter manages the creation and initialization of notification services.
	serviceRouter router.ServiceRouter
	// services contains the list of available service schemes.
	services = serviceRouter.ListServices()

	// Cmd is the cobra command for generating service documentation.
	// It displays configuration options and documentation for specified services.
	Cmd = &cobra.Command{
		Use:   "docs",
		Short: "Print documentation for services",
		Run:   Run,
		Args: func(cmd *cobra.Command, args []string) error {
			serviceList := strings.Join(services, ", ")
			cmd.SetUsageTemplate(
				cmd.UsageTemplate() + "\nAvailable services: \n  " + serviceList + "\n",
			)

			return cobra.MinimumNArgs(1)(cmd, args)
		},
		ValidArgs: services,
	}
)

// init initializes the command flags for the docs command.
func init() {
	Cmd.Flags().StringP("format", "f", "console", "Output format (console or markdown)")
}

// Run executes the docs command, generating and displaying service documentation.
//
// Parameters:
//   - rootCmd: The cobra command containing the parsed flags.
//   - args: The list of service schemes to document.
func Run(rootCmd *cobra.Command, args []string) {
	res := runDocs(rootCmd, args)
	if res.ExitCode != 0 {
		fmt.Fprintf(os.Stderr, "%s", res.Message)
	}

	os.Exit(res.ExitCode)
}

// runDocs contains the core logic for the docs command.
// It is extracted from Run to enable unit testing without calling os.Exit.
//
// Parameters:
//   - rootCmd: The cobra command containing the parsed flags.
//   - args: The list of service schemes to document.
//
// Returns:
//   - cmd.ExitError: An ExitError indicating success or failure with an appropriate exit code.
func runDocs(rootCmd *cobra.Command, args []string) cmd.ExitError {
	formatType, err := rootCmd.Flags().GetString("format")
	if err != nil {
		return cmd.ExitError{
			ExitCode: 1,
			Message:  fmt.Sprintf("Error getting format flag: %v\n", err),
		}
	}

	return printDocs(formatType, args)
}

// printDocs generates documentation for the specified services in the requested format.
//
// Parameters:
//   - docFormat: The output format ("console" or "markdown").
//   - services: The list of service schemes to document.
//
// Returns:
//   - cmd.ExitError: An ExitError indicating success or failure with an appropriate exit code.
func printDocs(docFormat string, services []string) cmd.ExitError {
	var renderer format.TreeRenderer

	// Select the appropriate renderer based on the requested format.
	switch docFormat {
	case "console":
		renderer = format.ConsoleTreeRenderer{WithValues: false}
	case "markdown":
		renderer = format.MarkdownTreeRenderer{
			HeaderPrefix:      "### ",
			PropsDescription:  "Props can be either supplied using the params argument, or through the URL using\n`?key=value&key=value` etc.\n",
			PropsEmptyMessage: "*The services does not support any query/param props*\n",
		}
	default:
		return cmd.InvalidUsage("invalid format")
	}

	// Create a logger for service initialization.
	logger := log.New(os.Stderr, "", 0)

	// Generate documentation for each requested service.
	for _, scheme := range services {
		service, err := serviceRouter.NewService(scheme)
		if err != nil {
			return cmd.InvalidUsage("failed to init service: " + err.Error())
		}

		// Initialize the service to populate Config.
		dummyURL, err := url.Parse(scheme + "://dummy@dummy.com")
		if err != nil {
			return cmd.InvalidUsage("failed to parse URL: " + err.Error())
		}

		if err := service.Initialize(dummyURL, logger); err != nil {
			return cmd.InvalidUsage(
				fmt.Sprintf("failed to initialize service %q: %v\n", scheme, err),
			)
		}

		config := format.GetServiceConfig(service)
		configNode := format.GetConfigFormat(config)
		_, _ = fmt.Fprint(os.Stdout, renderer.RenderTree(configNode, scheme))
	}

	return cmd.ErrNil
}
