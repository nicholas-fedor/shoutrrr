package teams

import (
	"fmt"
	"regexp"
)

// workflowURLValidator matches the Power Automate workflow webhook URL formats
// documented by Microsoft for the "When a Teams webhook request is received"
// trigger.
//
// Two host patterns are accepted:
//   - Legacy Logic Apps: https://*.logic.azure.com/workflows/...
//   - Current Power Platform: https://*.environment.api.powerplatform.com/powerautomate/automations/direct/.../workflows/...
//
// The Power Platform path may contain optional routing segments between
// "direct/" and "workflows/" (e.g. ".../direct/cu/04/workflows/..."), which
// Microsoft inserts when routing through the Power Platform ingress gateway.
//
// References:
//   - https://learn.microsoft.com/en-us/troubleshoot/power-platform/power-automate/flow-run-issues/triggers-troubleshoot?tabs=new-designer#changes-to-http-or-teams-webhook-trigger-flows
//   - https://learn.microsoft.com/en-us/power-automate/overview
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
