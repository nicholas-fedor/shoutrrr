// Package jsonclient provides a JSON HTTP client for making HTTP requests
// that automatically marshal and unmarshal JSON payloads.
//
// Request bodies are encoded with sorted map keys, and a nil slice or map
// encodes as an empty array or object. Responses are decoded with member names
// that match field names exactly. A repeated member name fails the decode.
// Invalid UTF-8 is replaced with U+FFFD in both directions.
//
// A response is decoded only when its status is from 200 to 299. Any other
// status fails the request with an [Error] that wraps [ErrUnexpectedStatus]
// and holds the status and the body. That includes a redirect, which reaches
// the client only when the HTTP client did not follow it.
package jsonclient
