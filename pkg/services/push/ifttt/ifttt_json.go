package ifttt

import (
	"encoding/json/v2"
	"fmt"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
)

// jsonPayload represents the notification payload sent to the IFTTT webhook API.
type jsonPayload struct {
	// Value1 is the event's first value.
	Value1 string `json:"value1"`
	// Value2 is the event's second value.
	Value2 string `json:"value2"`
	// Value3 is the event's third value.
	Value3 string `json:"value3"`
}

// Value field numbers, as messagevalue and titlevalue select them.
const (
	// ValueFieldOne represents the Value1 field in the IFTTT payload.
	ValueFieldOne = 1
	// ValueFieldTwo represents the Value2 field in the IFTTT payload.
	ValueFieldTwo = 2
	// ValueFieldThree represents the Value3 field in the IFTTT payload.
	ValueFieldThree = 3
)

// createJSONToSend generates a JSON payload for the IFTTT webhook API.
//
// The payload starts from the values of the send's config, which hold the
// value params. A non-empty title then replaces the field that titlevalue
// selects, and the message replaces the field that messagevalue selects.
//
// Parameters:
//   - config: The config for this send, with params already applied.
//   - message: The notification message.
//
// Returns:
//   - []byte: The JSON payload.
//   - error: The marshaling error.
func createJSONToSend(config *Config, message string) ([]byte, error) {
	payload := jsonPayload{
		Value1: config.Value1,
		Value2: config.Value2,
		Value3: config.Value3,
	}

	if config.Title != "" {
		payload.setValue(config.UseTitleAsValue, config.Title)
	}

	payload.setValue(config.UseMessageAsValue, message)

	jsonBytes, err := json.Marshal(payload, jsonopts.Default)
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
