// Package redact removes credentials from URLs and URL-bearing errors.
//
// Shoutrrr service URLs and the request URLs built from them often carry tokens,
// passwords and API keys in their userinfo, path or query. Errors from net/http
// and net/url embed those URLs, so anything that is returned to callers or logged
// passes through this package first. Only the scheme and host are kept.
package redact
