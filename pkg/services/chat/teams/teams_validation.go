package teams

import (
	"fmt"
	"regexp"
)

// workflowURLValidator matches Power Automate workflow webhook URLs.
// Newer URLs issued by Microsoft can include routing segments between
// "direct/" and "workflows/" (e.g. ".../automations/direct/cu/04/workflows/..."),
// so optional intermediate path segments are allowed before "workflows/".
var workflowURLValidator = regexp.MustCompile(
	`^https://[a-zA-Z0-9][a-zA-Z0-9.-]*\.logic\.azure(?:\.[a-z]{2,})?(:\d+)?/(?:powerautomate/automations/direct/(?:[A-Za-z0-9-]+/)*)?workflows/|` +
		`^https://[a-zA-Z0-9][a-zA-Z0-9.-]*\.environment\.api\.powerplatform\.com(:\d+)?/powerautomate/automations/direct/(?:[A-Za-z0-9-]+/)*workflows/`,
)

// ValidateWebhookURL ensures the webhook URL matches the Power Automate workflow pattern.
func ValidateWebhookURL(url string) error {
	if !workflowURLValidator.MatchString(url) {
		return fmt.Errorf("%w", ErrInvalidWebhookURL)
	}

	return nil
}
