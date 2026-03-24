// Package main provides the WASM module for the Shoutrrr Playground.
// It exposes service configuration, URL generation, parsing, and sending
// requests to the browser via syscall/js.
//
// HTTP requests use Go's net/http, which in WASM calls the browser's
// fetch API. The frontend patches fetch to strip the User-Agent header,
// which some CORS preflight responses don't allow.
package main
