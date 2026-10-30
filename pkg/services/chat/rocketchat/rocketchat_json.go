package rocketchat

import (
	"encoding/json/v2"
	"fmt"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// JSON represents the payload structure for the Rocket.Chat service.
type JSON struct {
	// Text is the message text.
	Text string `json:"text"`
	// UserName is the name the message is posted under. Empty omits it.
	UserName string `json:"username,omitempty"`
	// Channel is the channel or user that receives the message. Empty omits
	// it.
	Channel string `json:"channel,omitempty"`
}

// CreateJSONPayload generates a JSON payload compatible with the Rocket.Chat webhook API.
//
// Parameters:
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
		if value, found := (*params)[userNameParam]; found {
			payload.UserName = value
		}

		if value, found := (*params)[channelParam]; found {
			payload.Channel = value
		}
	}

	payloadBytes, err := json.Marshal(payload, jsonopts.Default)
	if err != nil {
		return nil, fmt.Errorf("marshaling Rocket.Chat payload to JSON: %w", err)
	}

	return payloadBytes, nil
}
