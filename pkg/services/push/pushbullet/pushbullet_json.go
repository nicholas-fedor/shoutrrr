package pushbullet

import (
	"regexp"
)

type PushRequest struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Body  string `json:"body"`

	Email      string `json:"email"`
	ChannelTag string `json:"channel_tag"`
	DeviceIden string `json:"device_iden"`
}

type PushResponse struct {
	Active                  bool    `json:"active"`
	Body                    string  `json:"body"`
	Created                 float64 `json:"created"`
	Direction               string  `json:"direction"`
	Dismissed               bool    `json:"dismissed"`
	Iden                    string  `json:"iden"`
	Modified                float64 `json:"modified"`
	ReceiverEmail           string  `json:"receiver_email"`
	ReceiverEmailNormalized string  `json:"receiver_email_normalized"`
	ReceiverIden            string  `json:"receiver_iden"`
	SenderEmail             string  `json:"sender_email"`
	SenderEmailNormalized   string  `json:"sender_email_normalized"`
	SenderIden              string  `json:"sender_iden"`
	SenderName              string  `json:"sender_name"`
	Title                   string  `json:"title"`
	Type                    string  `json:"type"`
}

type ResponseError struct {
	ErrorData struct {
		Cat     string `json:"cat"`
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

var emailPattern = regexp.MustCompile(`.*@.*\..*`)

func (err *ResponseError) Error() string {
	return err.ErrorData.Message
}

// NewNotePush creates a new push request.
//
//nolint:exhaustruct // PushRequest targeting fields (Email, ChannelTag, DeviceIden) are set by SetTarget method
func NewNotePush(message, title string) *PushRequest {
	return &PushRequest{
		Type:  "note",
		Title: title,
		Body:  message,
	}
}

func (p *PushRequest) SetTarget(target string) {
	if emailPattern.MatchString(target) {
		p.Email = target

		return
	}

	if target != "" && string(target[0]) == "#" {
		p.ChannelTag = target[1:]

		return
	}

	p.DeviceIden = target
}
