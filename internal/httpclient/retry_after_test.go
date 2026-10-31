package httpclient

import (
	"math"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRetryAfter verifies the wait read from a Retry-After header. Digits are
// seconds, and an HTTP date is the time until that date, zero for a date in
// the past. A value too large for a duration, or for an int64, becomes the
// longest wait. A missing header and any other value, including a signed or
// fractional number, give no wait.
func TestRetryAfter(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		now := time.Now()

		var longest time.Duration = math.MaxInt64

		// tests lists a Retry-After value, empty for no header, with the wait
		// it should give, or false when it gives none.
		tests := []struct {
			name   string
			header string
			want   time.Duration
			wantOK bool
		}{
			{name: "seconds", header: "7", want: 7 * time.Second, wantOK: true},
			{name: "zero seconds", header: "0", want: 0, wantOK: true},
			{name: "future date", header: now.Add(4 * time.Second).UTC().Format(http.TimeFormat), want: 4 * time.Second, wantOK: true},
			{name: "past date", header: now.Add(-time.Hour).UTC().Format(http.TimeFormat), want: 0, wantOK: true},
			{name: "too large for a duration", header: "99999999999999999", want: longest, wantOK: true},
			{name: "too large for an int64", header: "99999999999999999999", want: longest, wantOK: true},
			{name: "no header"},
			{name: "negative", header: "-5"},
			{name: "plus sign", header: "+5"},
			{name: "negative zero", header: "-0"},
			{name: "fraction", header: "1.5"},
			{name: "neither form", header: "later"},
		}

		for _, tt := range tests {
			header := http.Header{}
			if tt.header != "" {
				header.Set("Retry-After", tt.header)
			}

			wait, ok := RetryAfter(header)
			assert.Equal(t, tt.wantOK, ok, tt.name)
			assert.Equal(t, tt.want, wait, tt.name)
		}
	})
}

// TestDuration verifies that a count of a unit becomes that duration. A count
// too large for a duration becomes the longest one in any unit. A negative or
// NaN count gives none.
func TestDuration(t *testing.T) {
	t.Parallel()

	// tests lists a count and unit with the duration each should give, or
	// false when it gives none.
	tests := []struct {
		name   string
		count  float64
		unit   time.Duration
		want   time.Duration
		wantOK bool
	}{
		{name: "whole seconds", count: 7, unit: time.Second, want: 7 * time.Second, wantOK: true},
		{name: "fractional seconds", count: 2.5, unit: time.Second, want: 2500 * time.Millisecond, wantOK: true},
		{name: "milliseconds", count: 1500, unit: time.Millisecond, want: 1500 * time.Millisecond, wantOK: true},
		{name: "zero", count: 0, unit: time.Second, want: 0, wantOK: true},
		{name: "too large in seconds", count: 1e30, unit: time.Second, want: math.MaxInt64, wantOK: true},
		{name: "too large in nanoseconds", count: math.MaxInt64, unit: time.Nanosecond, want: math.MaxInt64, wantOK: true},
		{name: "infinite", count: math.Inf(1), unit: time.Second, want: math.MaxInt64, wantOK: true},
		{name: "negative", count: -1, unit: time.Second},
		{name: "negative zero", count: math.Copysign(0, -1), unit: time.Second, want: 0, wantOK: true},
		{name: "NaN", count: math.NaN(), unit: time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := Duration(tt.count, tt.unit)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
