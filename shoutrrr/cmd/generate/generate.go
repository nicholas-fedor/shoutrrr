package generate

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/shoutrrr/pkg/color"
	"github.com/nicholas-fedor/shoutrrr/pkg/generators"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// MaximumNArgs defines the maximum number of positional arguments allowed.
const MaximumNArgs = 2

// ErrNoServiceSpecified indicates that no service was provided for URL generation.
var (
	ErrNoServiceSpecified = errors.New("no service specified")
)

// serviceRouter manages the creation of notification services.
var serviceRouter router.ServiceRouter

// Cmd generates a notification service URL from user input.
var Cmd = &cobra.Command{
	Use:    "generate",
	Short:  "Generates a notification service URL from user input",
	Run:    Run,
	PreRun: loadArgsFromAltSources,
	Args:   cobra.MaximumNArgs(MaximumNArgs),
}

// loadArgsFromAltSources populates command flags from positional arguments if provided.
func loadArgsFromAltSources(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		_ = cmd.Flags().Set("service", args[0])
	}

	if len(args) > 1 {
		_ = cmd.Flags().Set("generator", args[1])
	}
}

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

// maskSensitiveURL masks sensitive parts of a Shoutrrr URL based on the service schema.
func maskSensitiveURL(serviceSchema, urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return urlStr // Return original URL if parsing fails
	}

	switch serviceSchema {
	case "discord", "slack", "teams":
		maskUser(parsedURL, "REDACTED")
	case "smtp":
		maskSMTPUser(parsedURL)
	case "pushover":
		maskPushoverQuery(parsedURL)
	case "gotify":
		maskGotifyQuery(parsedURL)
	default:
		maskGeneric(parsedURL)
	}

	return parsedURL.String()
}

// maskUser redacts the username in a URL with a placeholder.
func maskUser(parsedURL *url.URL, placeholder string) {
	if parsedURL.User != nil {
		parsedURL.User = url.User(placeholder)
	}
}

// maskSMTPUser redacts the password in an SMTP URL, preserving the username.
func maskSMTPUser(parsedURL *url.URL) {
	if parsedURL.User != nil {
		parsedURL.User = url.UserPassword(parsedURL.User.Username(), "REDACTED")
	}
}

// maskPushoverQuery redacts token and user query parameters in a Pushover URL.
func maskPushoverQuery(parsedURL *url.URL) {
	queryParams := parsedURL.Query()
	if queryParams.Get("token") != "" {
		queryParams.Set("token", "REDACTED")
	}

	if queryParams.Get("user") != "" {
		queryParams.Set("user", "REDACTED")
	}

	parsedURL.RawQuery = queryParams.Encode()
}

// maskGotifyQuery redacts the token query parameter in a Gotify URL.
func maskGotifyQuery(parsedURL *url.URL) {
	queryParams := parsedURL.Query()
	if queryParams.Get("token") != "" {
		queryParams.Set("token", "REDACTED")
	}

	parsedURL.RawQuery = queryParams.Encode()
}

// maskGeneric redacts userinfo and all query parameters for unrecognized services.
func maskGeneric(parsedURL *url.URL) {
	maskUser(parsedURL, "REDACTED")

	queryParams := parsedURL.Query()
	for key := range queryParams {
		queryParams.Set(key, "REDACTED")
	}

	parsedURL.RawQuery = queryParams.Encode()
}

// Run executes the generate command, producing a notification service URL.
func Run(cmd *cobra.Command, _ []string) {
	var service types.Service

	var err error

	serviceSchema, _ := cmd.Flags().GetString("service")
	generatorName, _ := cmd.Flags().GetString("generator")
	propertyFlags, _ := cmd.Flags().GetStringArray("property")
	showSensitive, _ := cmd.Flags().GetBool("show-sensitive")

	// Parse properties into a key-value map.
	props := make(map[string]string, len(propertyFlags))

	for _, prop := range propertyFlags {
		parts := strings.Split(prop, "=")
		if len(parts) != MaximumNArgs {
			fmt.Fprint(
				color.Output,
				"Invalid property key/value pair: ",
				color.HiYellowString(prop),
				"\n",
			)

			continue
		}

		props[parts[0]] = parts[1]
	}

	if len(propertyFlags) > 0 {
		fmt.Fprint(color.Output, "\n") // Add spacing after property warnings
	}

	// Validate and create the service.
	if serviceSchema == "" {
		err = ErrNoServiceSpecified
	} else {
		service, err = serviceRouter.NewService(serviceSchema)
	}

	if err != nil {
		fmt.Fprint(os.Stdout, "Error: ", err, "\n")
	}

	if service == nil {
		services := serviceRouter.ListServices()
		serviceList := strings.Join(services, ", ")
		cmd.SetUsageTemplate(cmd.UsageTemplate() + "\nAvailable services:\n  " + serviceList + "\n")
		_ = cmd.Usage()

		os.Exit(1)
	}

	// Determine the generator to use.
	var generator types.Generator

	generatorFlag := cmd.Flags().Lookup("generator")
	if !generatorFlag.Changed {
		// Use the service-specific default generator if available and no explicit generator is set.
		generator, _ = generators.NewGenerator(serviceSchema)
	}

	if generator != nil {
		generatorName = serviceSchema
	} else {
		var genErr error

		generator, genErr = generators.NewGenerator(generatorName)
		if genErr != nil {
			fmt.Fprint(os.Stdout, "Error: ", genErr, "\n")
		}
	}

	if generator == nil {
		generatorList := strings.Join(generators.ListGenerators(), ", ")
		cmd.SetUsageTemplate(
			cmd.UsageTemplate() + "\nAvailable generators:\n  " + generatorList + "\n",
		)

		_ = cmd.Usage()

		os.Exit(1)
	}

	// Generate and display the URL.
	fmt.Fprint(color.Output, "Generating URL for ", color.HiCyanString(serviceSchema))
	fmt.Fprint(color.Output, " using ", color.HiMagentaString(generatorName), " generator\n")

	serviceConfig, err := generator.Generate(service, props, cmd.Flags().Args())
	if err != nil {
		_, _ = fmt.Fprint(os.Stdout, "Error: ", err, "\n")
		os.Exit(1)
	}

	fmt.Fprint(color.Output, "\n")

	maskedURL := maskSensitiveURL(serviceSchema, serviceConfig.GetURL().String())

	if showSensitive {
		fmt.Fprint(os.Stdout, "URL: ", serviceConfig.GetURL().String(), "\n")
	} else {
		fmt.Fprint(os.Stdout, "URL: ", maskedURL, "\n")
	}
}
