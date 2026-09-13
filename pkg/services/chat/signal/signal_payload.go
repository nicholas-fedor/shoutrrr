package signal

import "strings"

// createPayload builds the JSON payload for POST /v2/send.
//
// Parameters:
//   - message: the message text
//   - config: the service configuration
//
// Returns:
//   - sendMessagePayload: the payload struct to be sent
func createPayload(message string, config *Config) sendMessagePayload {
	payload := sendMessagePayload{
		Message:           composeMessage(config.Title, message, config.TextMode == TextModeStyled),
		Number:            config.Source,
		Recipients:        apiRecipients(config.Recipients),
		Base64Attachments: parseAttachments(config.Attachments),
		TextMode:          config.TextMode.payloadValue(),
	}

	if !config.NotifySelf {
		payload.NotifySelf = new(false)
	}

	return payload
}

// composeMessage prepends an optional title to the message body.
//
// Parameters:
//   - title: the optional title
//   - message: the message body
//   - styled: whether styled markup should wrap the title
//
// Returns:
//   - string: the composed message
func composeMessage(title, message string, styled bool) string {
	if title == "" {
		return message
	}

	heading := title
	if styled {
		heading = "**" + title + "**"
	}

	if message == "" {
		return heading
	}

	return heading + "\n" + message
}

// parseAttachments splits a config attachments value into API attachment strings.
//
// Parameters:
//   - raw: comma-separated raw base64, or a single data URI
//
// Returns:
//   - []string: attachment entries, or nil when empty
//
// apiRecipients maps URL recipients to REST API recipient strings.
// Username values keep the u: prefix for URL parsing and drop it in the payload.
//
// Parameters:
//   - recipients: parsed URL recipients
//
// Returns:
//   - []string: recipients suitable for POST /v2/send
func apiRecipients(recipients []string) []string {
	out := make([]string, len(recipients))

	for i, recipient := range recipients {
		if rest, ok := strings.CutPrefix(recipient, usernamePrefix); ok && rest != "" {
			out[i] = rest

			continue
		}

		out[i] = recipient
	}

	return out
}

func parseAttachments(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	if strings.Contains(raw, "data:") {
		return []string{raw}
	}

	parts := strings.Split(raw, ",")
	attachments := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			attachments = append(attachments, part)
		}
	}

	if len(attachments) == 0 {
		return nil
	}

	return attachments
}
