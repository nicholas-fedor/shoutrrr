package httpclient

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryAfter returns the wait that a response's Retry-After header asks for.
// RFC 9110 defines the header as delay-seconds, which are digits only, or an
// HTTP date. A date in the past is a wait of zero, and a wait too long for a
// [time.Duration] becomes the longest one.
//
// Parameters:
//   - header: the response headers.
//
// Returns:
//   - time.Duration: the wait, zero or more.
//   - bool: false when the header is missing or holds neither form, such as a
//     signed or fractional number.
func RetryAfter(header http.Header) (time.Duration, bool) {
	value := header.Get("Retry-After")
	if value == "" {
		return 0, false
	}

	// For digits, ParseInt fails only with a range error, returning the
	// largest int64. Parsing as int64 keeps the value in the type of a
	// duration, so no conversion can overflow.
	if strings.Trim(value, "0123456789") == "" {
		seconds, _ := strconv.ParseInt(value, 10, 64) //nolint:errcheck // Digits fail only with a range error, which returns the largest int64.

		return time.Duration(min(seconds, int64(math.MaxInt64/time.Second))) * time.Second, true
	}

	if date, err := http.ParseTime(value); err == nil {
		return max(time.Until(date), 0), true
	}

	return 0, false
}
