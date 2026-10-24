package zulip

import (
	"encoding/json/v2"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
)

// CreatePayload builds a form-encoded payload for the Zulip messages API.
// A direct message lists its recipients as a JSON list of user IDs when every
// recipient is a number, or of e-mail addresses otherwise.
//
// Parameters:
//   - config: the configuration for this send.
//   - message: the message content.
//
// Returns:
//   - url.Values: the form fields.
func CreatePayload(config *Config, message string) url.Values {
	form := url.Values{}

	msgType := config.Type
	if msgType == "" {
		msgType = MessageTypeChannel
	}

	form.Set("type", string(msgType))

	if msgType == MessageTypeDirect {
		recipients := directRecipients(config)
		recipientList := parseRecipients(recipients)

		var (
			b   []byte
			err error
		)

		if ids, ok := userIDs(recipientList); ok {
			b, err = json.Marshal(ids, jsonopts.Default)
		} else {
			b, err = json.Marshal(recipientList, jsonopts.Default)
		}

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

// directRecipients returns the recipient list of a direct message: To, or
// Stream when To is empty.
//
// Parameters:
//   - config: the configuration for this send.
//
// Returns:
//   - string: the comma-separated recipients.
func directRecipients(config *Config) string {
	if config.To == "" {
		return config.Stream
	}

	return config.To
}

// userIDs returns the recipients as user IDs when every one is a positive
// integer.
//
// Parameters:
//   - recipients: the parsed recipients.
//
// Returns:
//   - []int64: the user IDs.
//   - bool: whether the list is non-empty and every recipient is a user ID.
func userIDs(recipients []string) ([]int64, bool) {
	ids := make([]int64, 0, len(recipients))

	for _, recipient := range recipients {
		id, ok := parseUserID(recipient)
		if !ok {
			return nil, false
		}

		ids = append(ids, id)
	}

	return ids, len(ids) > 0
}

// parseUserID reads a recipient as a Zulip user ID.
//
// Parameters:
//   - recipient: the recipient.
//
// Returns:
//   - int64: the user ID.
//   - bool: whether the recipient is a positive integer.
func parseUserID(recipient string) (int64, bool) {
	id, err := strconv.ParseInt(recipient, 10, 64)

	return id, err == nil && id > 0
}

// validateDirectRecipients checks that a direct message has recipients and
// that they are all user IDs or all e-mail addresses.
//
// Parameters:
//   - recipients: the parsed recipients.
//
// Returns:
//   - error: [ErrMissingRecipient], [ErrMixedRecipients], or nil.
func validateDirectRecipients(recipients []string) error {
	if len(recipients) == 0 {
		return ErrMissingRecipient
	}

	ids := 0

	for _, recipient := range recipients {
		if _, ok := parseUserID(recipient); ok {
			ids++
		}
	}

	if ids > 0 && ids < len(recipients) {
		return ErrMixedRecipients
	}

	return nil
}

// parseRecipients splits a comma-separated list of user IDs or email addresses.
//
// Parameters:
//   - s: the comma-separated recipients.
//
// Returns:
//   - []string: the recipients with surrounding spaces trimmed. Empty entries
//     are dropped.
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
//
// Parameters:
//   - t: the message type. Empty selects a channel message.
//
// Returns:
//   - error: [ErrInvalidMessageType] naming the type, or nil.
func ValidateMessageType(t MessageType) error {
	switch t {
	case MessageTypeChannel, MessageTypeDirect, "":
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidMessageType, t)
	}
}
