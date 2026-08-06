package smtp

import (
	"net/smtp"
)

type loginAuth struct {
	username, password string
}

// LoginAuth returns an Auth that implements the SASL LOGIN authentication
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {

		responseString := string(fromServer)

		if responseString == "Username:" {
			return []byte(a.username), nil
		}

		if responseString == "Password:" {
			return []byte(a.password), nil
		}

		return []byte("failed to perform AUTH LOGIN"), nil
	}
	return nil, nil
}

func (a *loginAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}
