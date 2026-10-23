package httpbody

import (
	"errors"
	"fmt"
	"io"
)

// MaxSize is the most of a response body that services read, 1 MiB.
//
// It is larger than any response the services expect. The largest are Signal's
// per-recipient results for a send to a full group and the Telegram
// generator's batch of updates.
const MaxSize = 1 << 20

// ErrTooLarge reports a response body larger than the read limit.
var ErrTooLarge = errors.New("response body is too large")

// Read reads r until EOF or until it holds more than limit bytes.
//
// It reads at most limit+1 bytes, so an oversized body is detected without
// buffering it. The bytes read before an error are returned with it, so callers
// that only show the body, such as in an error message, can still use the
// first limit bytes of an oversized one.
//
// Parameters:
//   - r: the response body.
//   - limit: the most bytes to return. It must not be negative.
//
// Returns:
//   - []byte: the body, or its first limit bytes when it is larger. It never
//     holds more than limit bytes, also when returned with a read error.
//   - error: an error wrapping [ErrTooLarge] when the body holds more than
//     limit bytes, or the read error.
func Read(r io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		// A reader can return its last bytes with the error, so the bytes read
		// can already be one past the limit.
		//nolint:wrapcheck // Callers wrap read errors with their own context.
		return body[:min(int64(len(body)), limit)], err
	}

	if int64(len(body)) > limit {
		return body[:limit], fmt.Errorf("%w: more than %d bytes", ErrTooLarge, limit)
	}

	return body, nil
}
