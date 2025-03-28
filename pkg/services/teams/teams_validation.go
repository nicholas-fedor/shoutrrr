package teams

import (
	"fmt"
	"regexp"
)

// Validation constants.
const (
	UUID4Length        = 36 // Length of a UUID4 identifier
	HashLength         = 32 // Length of a hash identifier
	WebhookDomain      = ".webhook.office.com"
	ExpectedComponents = 7 // Expected number of components in webhook URL (1 match + 6 captures)
	Path               = "webhookb2"
	ProviderName       = "IncomingWebhook"

	AltIDIndex      = 2 // Index of AltID in parts array
	GroupOwnerIndex = 3 // Index of GroupOwner in parts array
)

var (
	// HostValidator ensures the host matches the Teams webhook domain pattern.
	HostValidator = regexp.MustCompile(`^[a-zA-Z0-9-]+\.webhook\.office\.com$`)
	// WebhookURLValidator ensures the full webhook URL matches the Teams pattern.
	WebhookURLValidator = regexp.MustCompile(
		`^https://[a-zA-Z0-9-]+\.webhook\.office\.com/webhookb2/[0-9a-f-]{36}@[0-9a-f-]{36}/IncomingWebhook/[0-9a-f]{32}/[0-9a-f-]{36}/[^/]+$`,
	)
)

// ValidateWebhookURL ensures the webhook URL is valid before use.
func ValidateWebhookURL(url string) error {
	if !WebhookURLValidator.MatchString(url) {
		return fmt.Errorf("%w: %q", ErrInvalidWebhookURL, url)
	}

	return nil
}

// ParseAndVerifyWebhookURL extracts and validates webhook components from a URL.
func ParseAndVerifyWebhookURL(webhookURL string) ([5]string, error) {
	pattern := regexp.MustCompile(
		`([a-zA-Z0-9-\.]+)` + WebhookDomain + `/` + Path + `/([0-9a-f-]{36})@([0-9a-f-]{36})/` + ProviderName + `/([0-9a-f]{32})/([0-9a-f-]{36})/([^/]+)`,
	)

	groups := pattern.FindStringSubmatch(webhookURL)
	if len(groups) != ExpectedComponents {
		return [5]string{}, fmt.Errorf(
			"%w: expected %d components, got %d",
			ErrInvalidWebhookComponents,
			ExpectedComponents,
			len(groups),
		)
	}

	parts := [5]string{groups[2], groups[3], groups[4], groups[5], groups[6]}
	if err := verifyWebhookParts(parts); err != nil {
		return [5]string{}, err
	}

	return parts, nil
}

// verifyWebhookParts ensures webhook components meet format requirements.
func verifyWebhookParts(parts [5]string) error {
	type partSpec struct {
		name     string
		length   int
		index    int
		optional bool
	}

	specs := []partSpec{
		{name: "group ID", length: UUID4Length, index: 0, optional: true},
		{name: "tenant ID", length: UUID4Length, index: 1, optional: true},
		{name: "altID", length: HashLength, index: AltIDIndex, optional: true},
		{name: "groupOwner", length: UUID4Length, index: GroupOwnerIndex, optional: true},
	}

	for _, spec := range specs {
		if len(parts[spec.index]) != spec.length && parts[spec.index] != "" {
			return fmt.Errorf(
				"%w: %s must be %d characters, got %d",
				ErrInvalidPartLength,
				spec.name,
				spec.length,
				len(parts[spec.index]),
			)
		}
	}

	if parts[4] == "" {
		return ErrMissingExtraID
	}

	return nil
}

// BuildWebhookURL constructs a Teams webhook URL from components.
func BuildWebhookURL(host, group, tenant, altID, groupOwner, extraID string) string {
	// Host validation moved here for clarity
	if !HostValidator.MatchString(host) {
		return "" // Will trigger ErrInvalidHostFormat in caller
	}

	return fmt.Sprintf("https://%s/%s/%s@%s/%s/%s/%s/%s",
		host, Path, group, tenant, ProviderName, altID, groupOwner, extraID)
}
