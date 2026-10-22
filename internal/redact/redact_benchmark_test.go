package redact

import (
	"errors"
	"fmt"
	"net/url"
	"testing"
)

// benchRequestURL is a request URL with credentials in its userinfo, path, and
// query.
const benchRequestURL = "https://user:" + testSecret + "@api.example.com/bot" + testSecret +
	"/sendMessage?token=" + testSecret

// errBenchCause is the cause the redaction benchmarks wrap.
var errBenchCause = errors.New("connection refused")

// BenchmarkRequestURL measures reducing a request URL to its scheme and host.
func BenchmarkRequestURL(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = RequestURL(benchRequestURL)
	}
}

// BenchmarkURLError measures redacting the errors an HTTP client returns: a
// *url.Error whose cause quotes no URL, one whose cause quotes the URL again,
// and another error that quotes the URL in its message.
func BenchmarkURLError(b *testing.B) {
	// errs lists the errors to redact by name.
	errs := []struct {
		err  error
		name string
	}{
		{name: "url error", err: &url.Error{Op: "Post", URL: benchRequestURL, Err: errBenchCause}},
		{
			name: "url error quoting the url",
			err: &url.Error{
				Op:  "Get",
				URL: benchRequestURL,
				Err: fmt.Errorf("redirect to %q: %w", benchRequestURL, errBenchCause),
			},
		},
		{name: "other error", err: fmt.Errorf("posting to %s: %w", benchRequestURL, errBenchCause)},
	}

	for _, tt := range errs {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				_ = URLError(tt.err)
			}
		})
	}
}
