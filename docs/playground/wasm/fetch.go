//go:build js && wasm

package main

import "github.com/nicholas-fedor/shoutrrr"

// sendString sends a message to a service URL through [shoutrrr.Send].
//
// In the browser, net/http makes its requests with the fetch API, which the
// frontend wraps to strip the User-Agent header.
//
// Parameters:
//   - rawURL: the Shoutrrr service URL.
//   - message: the message to send.
//
// Returns:
//   - string: {"success":true} when the send succeeds, or an errorResult as
//     JSON.
func sendString(rawURL, message string) string {
	if err := shoutrrr.Send(rawURL, message); err != nil {
		return marshalError(err)
	}

	return `{"success":true}`
}
