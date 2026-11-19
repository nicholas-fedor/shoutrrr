package gotify

import (
	"encoding/json/v2"
	"fmt"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// DefaultPayloadBuilder provides the default implementation of PayloadBuilder.
type DefaultPayloadBuilder struct{}

// PayloadBuilder handles request payload construction and extras parsing.
type PayloadBuilder interface {
	// PrepareRequest builds the request body for message with config's title
	// and priority and the given extras.
	PrepareRequest(
		message string,
		config *Config,
		extras map[string]any,
		date string,
	) *MessageRequest
	// ParseExtras returns the extras param parsed as a JSON object, or the
	// configured extras when the params set none.
	ParseExtras(params *types.Params, config *Config) (map[string]any, error)
}

// ParseExtras returns the extras param parsed as a JSON object, or the
// configured extras when the params set none.
//
// Parameters:
//   - params: the send params, which may hold the extras param as JSON.
//   - config: the config with the configured extras.
//
// Returns:
//   - map[string]any: the extras for this send.
//   - error: [ErrExtrasUnmarshalFailed] when the extras param is not a JSON
//     object.
func (b *DefaultPayloadBuilder) ParseExtras(
	params *types.Params,
	config *Config,
) (map[string]any, error) {
	if params != nil {
		if extrasStr, exists := (*params)[extrasKey]; exists && extrasStr != "" {
			var requestExtras map[string]any
			if err := json.Unmarshal([]byte(extrasStr), &requestExtras, jsonopts.UserInput); err != nil {
				return nil, fmt.Errorf("%w", ErrExtrasUnmarshalFailed)
			}

			// A param of null sets no extras, so the configured ones apply.
			if requestExtras != nil {
				return requestExtras, nil
			}
		}
	}

	return config.Extras, nil
}

// PrepareRequest builds the request for a message.
//
// Parameters:
//   - message: the message text.
//   - config: the config for this send, with the title and priority.
//   - extras: the custom key-value pairs to send with the message.
//   - date: a timestamp kept on the request, which is never sent.
//
// Returns:
//   - *MessageRequest: the request, without a priority when the config has
//     none of its own.
func (b *DefaultPayloadBuilder) PrepareRequest(
	message string,
	config *Config,
	extras map[string]any,
	date string,
) *MessageRequest {
	request := &MessageRequest{Extras: extras, Date: nil, Priority: nil, Message: message, Title: config.Title}

	// The date and priority are copied in their branches, so a request
	// without them allocates nothing for them.
	if date != "" {
		request.Date = new(date)
	}

	if config.Priority != 0 || config.prioritySet {
		request.Priority = new(config.Priority)
	}

	return request
}
