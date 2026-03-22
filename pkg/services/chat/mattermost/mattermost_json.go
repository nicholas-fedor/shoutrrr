package mattermost

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// JSON represents the payload structure for Mattermost notifications.
type JSON struct {
	Text      string `json:"text"`
	UserName  string `json:"username,omitempty"`
	Channel   string `json:"channel,omitempty"`
	IconEmoji string `json:"icon_emoji,omitempty"`
	IconURL   string `json:"icon_url,omitempty"`
}

// SetIcon sets the appropriate icon field in the payload based on whether the input is a URL or not.
func (j *JSON) SetIcon(icon string) {
	j.IconURL = ""
	j.IconEmoji = ""

	if icon != "" {
		lower := strings.ToLower(icon)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			j.IconURL = icon
		} else {
			j.IconEmoji = icon
		}
	}
}

// CreateJSONPayload generates a JSON payload for the Mattermost service.
func CreateJSONPayload(config *Config, message string, params *types.Params) ([]byte, error) {
	// Extract title from params if provided, before building the text
	title := config.Title

	if params != nil {
		if value, found := (*params)["title"]; found {
			title = value
		}
	}

	// Prepend title to message when present
	text := message
	if title != "" {
		text = title + "\n" + message
	}

	payload := JSON{
		Text:      text,
		UserName:  config.UserName,
		Channel:   config.Channel,
		IconEmoji: "",
		IconURL:   "",
	}

	icon := config.Icon

	if params != nil {
		if value, found := (*params)["username"]; found {
			payload.UserName = value
		}

		if value, found := (*params)["channel"]; found {
			payload.Channel = value
		}

		if value, found := (*params)["icon"]; found {
			icon = value
		}
	}

	payload.SetIcon(icon)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling Mattermost payload to JSON: %w", err)
	}

	return payloadBytes, nil
}
