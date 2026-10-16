package signal

import (
	"encoding/json/v2"
	"fmt"
	"strconv"
)

// sendMessagePayload is the JSON body posted to POST /v2/send.
type sendMessagePayload struct {
	Message           string   `json:"message"`
	Number            string   `json:"number"`
	Recipients        []string `json:"recipients"`
	Base64Attachments []string `json:"base64_attachments,omitempty"`
	TextMode          string   `json:"text_mode,omitempty"`
	NotifySelf        *bool    `json:"notify_self,omitempty"`
}

// sendErrorResponse is the error body from a failed send.
type sendErrorResponse struct {
	Error           string   `json:"error"`
	ChallengeTokens []string `json:"challenge_tokens,omitempty"`
	Account         string   `json:"account,omitempty"`
}

// sendResult is one signal-cli send in a successful /v2/send response.
type sendResult struct {
	// Timestamp is the time the message was sent, or nil when the response has none.
	Timestamp *timestamp `json:"timestamp"`
	// Errors lists the recipients the send could not reach.
	Errors *sendResultErrors `json:"errors,omitempty"`
}

// sendResultErrors holds the failures of a send result.
type sendResultErrors struct {
	// Recipients lists each recipient the send could not reach.
	Recipients []recipientFailure `json:"recipients,omitempty"`
}

// recipientFailure is a recipient a send could not reach, identified by
// number, username, or UUID.
type recipientFailure struct {
	// Username is the recipient's Signal username, without the u: prefix.
	Username string `json:"username,omitempty"`
	// Number is the recipient's phone number.
	Number string `json:"number,omitempty"`
	// UUID is the recipient's Signal account ID.
	UUID string `json:"uuid,omitempty"`
	// Reason is the signal-cli result type, such as UNREGISTERED_FAILURE.
	Reason string `json:"reason,omitempty"`
}

// timestamp is a send time in milliseconds since the Unix epoch, which the
// API writes as a quoted number.
type timestamp int64

// UnmarshalJSON reads a timestamp from a quoted or plain JSON number.
//
// Parameters:
//   - data: the JSON value.
//
// Returns:
//   - error: when the value is not an integer.
func (ts *timestamp) UnmarshalJSON(data []byte) error {
	text := string(data)

	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return fmt.Errorf("decoding timestamp: %w", err)
		}
	}

	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return fmt.Errorf("parsing timestamp %q: %w", text, err)
	}

	*ts = timestamp(n)

	return nil
}

// describe names the recipient by number, username, or UUID, with the
// failure reason.
//
// Returns:
//   - string: the recipient and reason, such as "+15551234567 (UNREGISTERED_FAILURE)".
func (f recipientFailure) describe() string {
	recipient := f.Number
	if recipient == "" {
		recipient = f.Username
	}

	if recipient == "" {
		recipient = f.UUID
	}

	return recipient + " (" + f.Reason + ")"
}
