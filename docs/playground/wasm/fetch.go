//go:build js && wasm

package main

import "github.com/nicholas-fedor/shoutrrr"

// sendString sends a notification via rawURL with the given message.
// It calls shoutrrr.Send() which handles URL conversion and body
// formatting internally. The browser's fetch API (used by net/http
// in WASM) is patched by the frontend to strip the User-Agent header.
func sendString(rawURL, message string) string {
	if err := shoutrrr.Send(rawURL, message); err != nil {
		return marshalError(err)
	}

	return `{"success":true}`
}
