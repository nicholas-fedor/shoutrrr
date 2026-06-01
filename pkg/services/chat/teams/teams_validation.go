package teams

import (
	"fmt"
	"regexp"
)

var workflowURLValidator = regexp.MustCompile(
	`^https://[a-zA-Z0-9][a-zA-Z0-9.-]*\.logic\.azure(?:\.[a-z]{2,})?(:\d+)?/(?:powerautomate/automations/direct/)?workflows/|` +
		`^https://[a-zA-Z0-9][a-zA-Z0-9.-]*\.environment\.api\.powerplatform\.com(:\d+)?/powerautomate/automations/direct/workflows/`,
)

// ValidateWebhookURL ensures the webhook URL matches the Power Automate workflow pattern.
func ValidateWebhookURL(url string) error {
	if !workflowURLValidator.MatchString(url) {
		return fmt.Errorf("%w", ErrInvalidWebhookURL)
	}

	return nil
}
