package mattermost

import (
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// JSON represents the payload structure for Mattermost notifications.
type JSON struct {
	// Text is the message, with the title on its own line before it.
	Text string `json:"text"`
	// UserName overrides the webhook's user name, left out when empty.
	UserName string `json:"username,omitempty"`
	// Channel overrides the webhook's channel, left out when empty.
	Channel string `json:"channel,omitempty"`
	// IconEmoji overrides the webhook's icon with an emoji, left out when
	// empty.
	IconEmoji string `json:"icon_emoji,omitempty"`
	// IconURL overrides the webhook's icon with an image URL, left out when
	// empty.
	IconURL string `json:"icon_url,omitempty"`
}

// SetIcon sets the appropriate icon field in the payload based on whether the input is a URL or not.
//
// Parameters:
//   - icon: an http or https URL for [JSON.IconURL], an emoji name for
//     [JSON.IconEmoji], or empty to clear both.
func (j *JSON) SetIcon(icon string) {
	if hasPrefixFold(icon, "http://") || hasPrefixFold(icon, "https://") {
		j.IconURL, j.IconEmoji = icon, ""

		return
	}

	j.IconURL, j.IconEmoji = "", icon
}

// CreateJSONPayload generates a JSON payload for the Mattermost service.
//
// Parameters:
//   - config: the configuration with the message options.
//   - message: the message to send.
//   - params: optional username and channel overrides. The send applies
//     every other param to config first.
//
// Returns:
//   - []byte: the JSON payload.
//   - error: [ErrMessageMissing] when the title and message are both empty, or
//     the encoding error.
func CreateJSONPayload(config *Config, message string, params *types.Params) ([]byte, error) {
	text := util.TitledMessage(config.Title, message)
	if text == "" {
		return nil, ErrMessageMissing
	}

	payload := JSON{
		Text:      text,
		UserName:  config.UserName,
		Channel:   config.Channel,
		IconEmoji: "",
		IconURL:   "",
	}

	if params != nil {
		if value, found := (*params)[userNameParam]; found {
			payload.UserName = value
		}

		if value, found := (*params)[channelParam]; found {
			payload.Channel = value
		}
	}

	payload.SetIcon(config.Icon)

	payloadBytes, err := json.Marshal(payload, jsonopts.Default)
	if err != nil {
		return nil, fmt.Errorf("marshaling Mattermost payload to JSON: %w", err)
	}

	return payloadBytes, nil
}

// hasPrefixFold reports whether s begins with prefix, ignoring letter case
// under Unicode simple case folding.
//
// Parameters:
//   - s: the string to check.
//   - prefix: the prefix.
//
// Returns:
//   - bool: true when s begins with prefix in any case.
func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}
