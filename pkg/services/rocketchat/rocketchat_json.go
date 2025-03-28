package rocketchat

import (
	"encoding/json"
	"fmt"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// JSON represents the payload structure for the Rocket.Chat service.
type JSON struct {
	Text     string `json:"text"`
	UserName string `json:"username,omitempty"`
	Channel  string `json:"channel,omitempty"`
}

// CreateJSONPayload generates a JSON payload compatible with the Rocket.Chat webhook API.
func CreateJSONPayload(config *Config, message string, params *types.Params) ([]byte, error) {
	payload := JSON{
		Text:     message,
		UserName: config.UserName,
		Channel:  config.Channel,
	}

	if params != nil {
		if value, found := (*params)["username"]; found {
			payload.UserName = value
		}

		if value, found := (*params)["channel"]; found {
			payload.Channel = value
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling Rocket.Chat payload to JSON: %w", err)
	}

	return payloadBytes, nil
}
