package gotify

import (
	"encoding/json"
	"fmt"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// PayloadBuilder handles request payload construction and extras parsing.
type PayloadBuilder interface {
	PrepareRequest(
		message string,
		config *Config,
		extras map[string]any,
		date string,
	) *MessageRequest
	ParseExtras(params *types.Params, config *Config) (map[string]any, error)
}

// DefaultPayloadBuilder provides the default implementation of PayloadBuilder.
type DefaultPayloadBuilder struct{}

// PrepareRequest builds the request payload.
// This function constructs the JSON payload that will be sent to the Gotify API,
// combining the message content with configuration settings and any additional extras.
// Parameters:
//   - message: The main notification message text
//   - config: Configuration containing title, priority, and other settings
//   - extras: Additional key-value pairs to include in the notification
//   - date: Optional custom timestamp in ISO 8601 format
//
// Returns: *MessageRequest containing all data to be sent to the API.
func (b *DefaultPayloadBuilder) PrepareRequest(
	message string,
	config *Config,
	extras map[string]any,
	date string,
) *MessageRequest {
	var datePtr *string
	if date != "" {
		datePtr = &date
	}

	return &MessageRequest{
		Message:  message,         // The notification message content
		Title:    config.Title,    // Notification title from configuration
		Priority: config.Priority, // Priority level for the notification
		Date:     datePtr,         // Optional custom timestamp
		Extras:   extras,          // Additional metadata or custom fields
	}
}

// ParseExtras handles extras parsing from params.
// This function processes the 'extras' parameter which contains additional JSON data
// to be sent with the notification. It attempts to parse JSON from parameters first,
// falling back to configuration extras if parsing fails or no parameter extras exist.
// Parameters:
//   - params: Request parameters that may contain 'extras' JSON string
//   - config: Configuration that may contain default extras
//
// Returns: map of extra data to include in the notification payload, or error if parsing fails.
func (b *DefaultPayloadBuilder) ParseExtras(
	params *types.Params,
	config *Config,
) (map[string]any, error) {
	// Initialize variable to hold parsed extras
	var requestExtras map[string]any

	// Check if parameters exist and contain extras
	if params != nil {
		// Look for 'extras' key in parameters
		if extrasStr, exists := (*params)["extras"]; exists && extrasStr != "" {
			// Attempt to parse the JSON string into a map
			if err := json.Unmarshal([]byte(extrasStr), &requestExtras); err != nil {
				// Return error for parsing failure
				return nil, fmt.Errorf("%w", ErrExtrasUnmarshalFailed)
			}
		}
	}

	// Fall back to configuration extras if no valid parameter extras were found
	if requestExtras == nil {
		requestExtras = config.Extras
	}

	// Return the resolved extras (either from params or config)
	return requestExtras, nil
}
