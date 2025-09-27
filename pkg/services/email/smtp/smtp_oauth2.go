package smtp

import (
	"net/smtp"
)

type oauth2Auth struct {
	username, accessToken string
}

// OAuth2Auth returns an Auth that implements the SASL XOAUTH2 authentication
// as per https://developers.google.com/gmail/imap/xoauth2-protocol.
// It assumes the provided password is a valid OAuth2 access token.
// Token refresh or complex challenge-response flows are not supported.
func OAuth2Auth(username, accessToken string) smtp.Auth {
	return &oauth2Auth{username, accessToken}
}

func (a *oauth2Auth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	resp := []byte("user=" + a.username + "\x01auth=Bearer " + a.accessToken + "\x01\x01")

	return "XOAUTH2", resp, nil
}

func (a *oauth2Auth) Next(_ []byte, _ bool) ([]byte, error) {
	return nil, nil
}
