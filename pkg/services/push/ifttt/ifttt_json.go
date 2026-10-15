package ifttt

import (
	"encoding/json"
	"fmt"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// jsonPayload represents the notification payload sent to the IFTTT webhook API.
type jsonPayload struct {
	Value1 string `json:"value1"`
	Value2 string `json:"value2"`
	Value3 string `json:"value3"`
}

// ValueFieldOne represents the Value1 field in the IFTTT payload.
const (
	ValueFieldOne   = 1 // Represents Value1 field
	ValueFieldTwo   = 2 // Represents Value2 field
	ValueFieldThree = 3 // Represents Value3 field
)

// createJSONToSend generates a JSON payload for the IFTTT webhook API.
//
// The payload starts from the configured values and the value params. A
// non-empty title then replaces the field that titlevalue selects, and the
// message replaces the field that messagevalue selects.
//
// Parameters:
//   - config: The config for this send, with params already applied.
//   - message: The notification message.
//   - params: The send params, or nil for none.
//
// Returns:
//   - []byte: The JSON payload.
//   - error: The marshaling error.
func createJSONToSend(config *Config, message string, params *types.Params) ([]byte, error) {
	payload := jsonPayload{
		Value1: config.Value1,
		Value2: config.Value2,
		Value3: config.Value3,
	}

	if params != nil {
		if value, found := (*params)["value1"]; found {
			payload.Value1 = value
		}

		if value, found := (*params)["value2"]; found {
			payload.Value2 = value
		}

		if value, found := (*params)["value3"]; found {
			payload.Value3 = value
		}
	}

	if config.Title != "" {
		payload.setValue(config.UseTitleAsValue, config.Title)
	}

	payload.setValue(config.UseMessageAsValue, message)

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling IFTTT payload to JSON: %w", err)
	}

	return jsonBytes, nil
}

// setValue writes value to the payload field that field selects.
//
// Parameters:
//   - field: The value field number, 1 to 3. Any other number writes nothing.
//   - value: The value to write.
func (p *jsonPayload) setValue(field uint8, value string) {
	switch field {
	case ValueFieldOne:
		p.Value1 = value
	case ValueFieldTwo:
		p.Value2 = value
	case ValueFieldThree:
		p.Value3 = value
	default:
	}
}
