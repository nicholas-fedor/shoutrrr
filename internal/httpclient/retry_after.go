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
	// largest int64, which Duration turns into the longest wait.
	if strings.Trim(value, "0123456789") == "" {
		seconds, _ := strconv.ParseInt(value, 10, 64) //nolint:errcheck // Digits fail only with a range error, which returns the largest int64.

		return Duration(float64(seconds), time.Second)
	}

	if date, err := http.ParseTime(value); err == nil {
		return max(time.Until(date), 0), true
	}

	return 0, false
}

// Duration returns count units as a [time.Duration], such as a wait that a
// response body gives in seconds with a fraction. A count too large for a
// duration becomes the longest one, because converting an out-of-range float
// to an integer has no defined result.
//
// Parameters:
//   - count: the number of units, with a fraction.
//   - unit: the length of one unit, such as [time.Second]. It must be
//     positive.
//
// Returns:
//   - time.Duration: the duration, zero or more.
//   - bool: false when count is negative or NaN.
func Duration(count float64, unit time.Duration) (time.Duration, bool) {
	// A NaN count fails every comparison, so this rejects it too.
	if !(count >= 0) {
		return 0, false
	}

	if count >= float64(math.MaxInt64/unit) {
		return math.MaxInt64, true
	}

	return time.Duration(count * float64(unit)), true
}
