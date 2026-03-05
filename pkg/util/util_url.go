package util

import "net/url"

// URLUserPassword is a replacement/wrapper around url.UserPassword that treats empty string arguments as not specified.
// If no user or password is specified, it returns nil (which serializes in url.URL to "").
func URLUserPassword(user, password string) *url.Userinfo {
	if password != "" {
		return url.UserPassword(user, password)
	} else if user != "" {
		return url.User(user)
	}

	return nil
}
