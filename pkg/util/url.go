package util

import "net/url"

// URLUserPassword creates a url.Userinfo from user and password strings.
//
// Unlike url.UserPassword, this function treats empty strings as "not specified":
//   - If password is non-empty, returns url.UserPassword(user, password).
//   - If password is empty but user is non-empty, returns url.User(user).
//   - If both are empty, returns nil (which serializes to "" in url.URL).
//
// Parameters:
//   - user: The username, or empty string if no authentication.
//   - password: The password, or empty string if no password.
//
// Returns:
//   - A *url.Userinfo with the appropriate authentication info, or nil if neither user nor password is provided.
func URLUserPassword(user, password string) *url.Userinfo {
	if password != "" {
		return url.UserPassword(user, password)
	} else if user != "" {
		return url.User(user)
	}

	return nil
}
