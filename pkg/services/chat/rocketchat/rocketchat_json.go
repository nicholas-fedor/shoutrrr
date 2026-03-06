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
//
// Params:
//   - config: The Rocket.Chat configuration containing user and channel settings
//   - message: The message text to include in the payload
//   - params: Optional parameters that can override config values (username, channel)
//
// Returns:
//   - []byte: The JSON payload as a byte slice
//   - error: An error if JSON marshaling fails
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
