package zulip

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// CreatePayload builds a form-encoded payload for the Zulip messages API.
func CreatePayload(config *Config, message string) url.Values {
	form := url.Values{}

	msgType := config.Type
	if msgType == "" {
		msgType = MessageTypeChannel
	}

	form.Set("type", string(msgType))

	if msgType == MessageTypeDirect {
		recipients := config.To
		if recipients == "" && config.Stream != "" {
			recipients = config.Stream
		}

		recipientList := parseRecipients(recipients)

		b, err := json.Marshal(recipientList)
		if err != nil {
			form.Set("to", recipients)
		} else {
			form.Set("to", string(b))
		}
	} else {
		form.Set("to", config.Stream)
	}

	form.Set("content", message)

	if config.Topic != "" && msgType != MessageTypeDirect {
		form.Set("topic", config.Topic)
	}

	if config.ReadBySender {
		form.Set("read_by_sender", "true")
	}

	return form
}

// parseRecipients splits a comma-separated list of user IDs or email addresses.
func parseRecipients(s string) []string {
	parts := strings.Split(s, ",")

	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// ValidateMessageType returns an error if the message type is not supported.
func ValidateMessageType(t MessageType) error {
	switch t {
	case MessageTypeChannel, MessageTypeDirect, "":
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidMessageType, t)
	}
}
