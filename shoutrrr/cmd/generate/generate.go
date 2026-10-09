// Package generate provides the CLI command for generating notification service URLs.
package generate

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	internalUtil "github.com/nicholas-fedor/shoutrrr/internal/util"
	"github.com/nicholas-fedor/shoutrrr/pkg/color"
	"github.com/nicholas-fedor/shoutrrr/pkg/generators"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/pushover"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// MaximumNArgs was the maximum number of positional arguments generate accepted.
//
// Deprecated: generate accepts the service, the generator, and any arguments for
// the generator, so it has no maximum.
const MaximumNArgs = 2

// redactedStr represents the placeholder string for masked text values.
const redactedStr = "REDACTED"

var (
	// ErrNoServiceSpecified indicates that no service was provided for URL generation.
	ErrNoServiceSpecified = errors.New("no service specified")

	// serviceRouter manages the creation of notification services.
	serviceRouter router.ServiceRouter

	// Cmd is the cobra command for generating notification service URLs.
	// It creates a URL from user-provided properties and configuration.
	Cmd = &cobra.Command{
		Use:   "generate [service] [generator] [generator args...]",
		Short: "Generates a notification service URL from user input",
		Run:   Run,
		Args:  cobra.ArbitraryArgs,
	}
)

// init initializes the command flags for the generate command.
func init() {
	serviceRouter = router.ServiceRouter{}

	Cmd.Flags().
		StringP("service", "s", "", "Notification service to generate a URL for (e.g., discord, smtp)")
	Cmd.Flags().
		StringP("generator", "g", "basic", "Generator to use (e.g., basic, or service-specific)")
	Cmd.Flags().
		StringArrayP("property", "p", []string{}, "Configuration property in key=value format (e.g., token=abc123)")
	Cmd.Flags().
		BoolP("show-sensitive", "x", false, "Show sensitive data in the generated URL (default: masked)")
}

// Run executes the generate command, producing a notification service URL.
//
// Positional arguments fill the service and then the generator flag, skipping
// flags set on the command line, and the remaining arguments go to the generator.
// Environment variables then fill any flag that is still unset.
//
// Parameters:
//   - cmd: The cobra command containing the parsed flags.
//   - args: The positional arguments.
func Run(cmd *cobra.Command, args []string) {
	var service types.Service

	generatorArgs, err := applySources(cmd, args)
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, "Error: ", err, "\n")

		os.Exit(1)
	}

	// Retrieve command flags.
	serviceSchema, err := cmd.Flags().GetString("service")
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, "Error getting service flag: ", err, "\n")

		os.Exit(1)
	}

	generatorName, err := cmd.Flags().GetString("generator")
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, "Error getting generator flag: ", err, "\n")

		os.Exit(1)
	}

	propertyFlags, err := cmd.Flags().GetStringArray("property")
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, "Error getting property flag: ", err, "\n")

		os.Exit(1)
	}

	showSensitive, err := cmd.Flags().GetBool("show-sensitive")
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, "Error getting show-sensitive flag: ", err, "\n")

		os.Exit(1)
	}

	// Parse properties into a key-value map.
	props := make(map[string]string, len(propertyFlags))

	cfg := color.DefaultConfig()

	for _, prop := range propertyFlags {
		key, value, found := strings.Cut(prop, "=")
		if !found || key == "" {
			_, _ = fmt.Fprint(
				cfg.Output,
				"Invalid property key/value pair: ",
				color.HiYellowString(prop),
				"\n",
			)

			continue
		}

		props[key] = value
	}

	if len(propertyFlags) > 0 {
		// Add spacing after property warnings.
		_, _ = fmt.Fprint(cfg.Output, "\n")
	}

	// Validate and create the service.
	if serviceSchema == "" {
		err = ErrNoServiceSpecified
	} else {
		service, err = serviceRouter.NewService(serviceSchema)
	}

	if err != nil {
		_, _ = fmt.Fprint(os.Stdout, "Error: ", err, "\n")
	}

	if service == nil {
		services := serviceRouter.ListServices()
		serviceList := strings.Join(services, ", ")
		cmd.SetUsageTemplate(cmd.UsageTemplate() + "\nAvailable services:\n  " + serviceList + "\n")

		if err := cmd.Usage(); err != nil {
			_, _ = fmt.Fprint(os.Stderr, "Error displaying usage: ", err, "\n")
		}

		os.Exit(1)
	}

	// Determine the generator to use.
	var generator types.Generator

	generatorFlag := cmd.Flags().Lookup("generator")
	if !generatorFlag.Changed {
		// Use the service-specific default generator if available and no explicit generator is set.
		generator, err = generators.NewGenerator(serviceSchema)
		if err != nil {
			// Service-specific generator not found, will try basic generator later.
			generator = nil
		}
	}

	if generator != nil {
		generatorName = serviceSchema
	} else {
		var genErr error

		generator, genErr = generators.NewGenerator(generatorName)
		if genErr != nil {
			_, _ = fmt.Fprint(os.Stdout, "Error: ", genErr, "\n")
		}
	}

	if generator == nil {
		generatorList := strings.Join(generators.ListGenerators(), ", ")
		cmd.SetUsageTemplate(
			cmd.UsageTemplate() + "\nAvailable generators:\n  " + generatorList + "\n",
		)

		if err := cmd.Usage(); err != nil {
			_, _ = fmt.Fprint(os.Stderr, "Error displaying usage: ", err, "\n")
		}

		os.Exit(1)
	}

	// Generate and display the URL.
	_, _ = fmt.Fprint(cfg.Output, "Generating URL for ", color.HiCyanString(serviceSchema))
	_, _ = fmt.Fprint(cfg.Output, " using ", color.HiMagentaString(generatorName), " generator\n")

	serviceConfig, err := generator.Generate(service, props, generatorArgs)
	if err != nil {
		_, _ = fmt.Fprint(os.Stdout, "Error: ", err, "\n")

		os.Exit(1)
	}

	_, _ = fmt.Fprint(cfg.Output, "\n")

	maskedURL := maskSensitiveURL(serviceSchema, serviceConfig.GetURL().String())

	if showSensitive {
		_, _ = fmt.Fprint(os.Stdout, "URL: ", serviceConfig.GetURL().String(), "\n")
	} else {
		_, _ = fmt.Fprint(os.Stdout, "URL: ", maskedURL, "\n")
	}
}

// applySources fills the service and generator flags from positional arguments
// and then fills any unset flag from its environment variable.
//
// Parameters:
//   - cmd: the generate command.
//   - args: the positional arguments.
//
// Returns:
//   - []string: the arguments for the generator, left over after the service and
//     generator flags are filled.
//   - error: an error naming the flag or environment variable that could not be applied.
func applySources(cmd *cobra.Command, args []string) ([]string, error) {
	generatorArgs, err := internalUtil.ApplyArgs(cmd, args, "service", "generator")
	if err != nil {
		return nil, fmt.Errorf("applying positional arguments: %w", err)
	}

	if err := internalUtil.ApplyEnv(cmd); err != nil {
		return nil, fmt.Errorf("applying environment variables: %w", err)
	}

	return generatorArgs, nil
}

// maskSensitiveURL masks sensitive parts of a Shoutrrr URL based on the service schema.
//
// Parameters:
//   - serviceSchema: The service type (e.g., "discord", "smtp", "pushover").
//   - urlStr: The URL string to mask.
//
// Returns:
//   - string: The URL with sensitive information masked.
func maskSensitiveURL(serviceSchema, urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// Return original URL if parsing fails.
		return urlStr
	}

	switch serviceSchema {
	case "discord", "slack", "teams":
		maskUser(parsedURL, redactedStr)
	case "smtp":
		maskSMTPUser(parsedURL)
	case "pushover":
		maskPushoverQuery(parsedURL)
	case "gotify":
		maskGotifyQuery(parsedURL)
	case "signalgrid":
		maskSignalgrid(parsedURL)
	default:
		maskGeneric(parsedURL)
	}

	return parsedURL.String()
}

// maskUser redacts the username in a URL with a placeholder.
//
// Parameters:
//   - parsedURL: The URL to modify.
//   - placeholder: The replacement string for the username.
func maskUser(parsedURL *url.URL, placeholder string) {
	if parsedURL.User != nil {
		parsedURL.User = url.User(placeholder)
	}
}

// maskSMTPUser redacts the password in an SMTP URL, preserving the username.
//
// Parameters:
//   - parsedURL: The SMTP URL to modify.
func maskSMTPUser(parsedURL *url.URL) {
	if parsedURL.User != nil {
		parsedURL.User = url.UserPassword(parsedURL.User.Username(), redactedStr)
	}
}

// maskPushoverQuery redacts the API token in URL userinfo and sensitive query
// parameters in a Pushover URL.
//
// Parameters:
//   - parsedURL: The Pushover URL to modify.
func maskPushoverQuery(parsedURL *url.URL) {
	pushover.MaskURL(parsedURL)
}

// maskGotifyQuery redacts the token query parameter in a Gotify URL.
//
// Parameters:
//   - parsedURL: The Gotify URL to modify.
func maskGotifyQuery(parsedURL *url.URL) {
	queryParams := parsedURL.Query()
	if queryParams.Get("token") != "" {
		queryParams.Set("token", redactedStr)
	}

	parsedURL.RawQuery = queryParams.Encode()
}

// maskSignalgrid redacts the client key in userinfo and the channel token in the host.
//
// Parameters:
//   - parsedURL: The Signalgrid URL to modify.
func maskSignalgrid(parsedURL *url.URL) {
	maskUser(parsedURL, redactedStr)

	if parsedURL.Host != "" {
		parsedURL.Host = redactedStr
	}
}

// maskGeneric redacts userinfo and all query parameters for unrecognized services.
//
// Parameters:
//   - parsedURL: The URL to modify.
func maskGeneric(parsedURL *url.URL) {
	maskUser(parsedURL, redactedStr)

	queryParams := parsedURL.Query()
	for key := range queryParams {
		queryParams.Set(key, redactedStr)
	}

	parsedURL.RawQuery = queryParams.Encode()
}
