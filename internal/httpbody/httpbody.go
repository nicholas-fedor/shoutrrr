package httpbody

import (
	"errors"
	"fmt"
	"io"
)

// limitedBody is a response body that [Limit] caps and whose Close reads it to
// its limit first.
type limitedBody struct {
	// body is the response body.
	body io.ReadCloser
	// left is the number of bytes left to read before the limit, or 0 once
	// the body has reached its end.
	left int64
	// drain is true when Close reads the rest of the body first.
	drain bool
}

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

// Limit returns a response body that reads at most [MaxSize]+1 bytes of body,
// the most any read of a response needs to detect an oversized one.
//
// Its Close reads what is left of a body whose length the response declares,
// up to [MaxSize], then closes body. The body is then read to its end, which
// lets its connection carry the next request at once, where a body closed
// early leaves the transport to read the rest in the background. A body of
// unknown or larger length is closed without reading on, so a server that
// streams or stalls cannot hold up the send, and the transport reads what it
// can in the background.
//
// Parameters:
//   - body: the response body.
//   - length: the body length the response declares, or -1 when it is unknown.
//
// Returns:
//   - io.ReadCloser: the limited body.
func Limit(body io.ReadCloser, length int64) io.ReadCloser {
	left := int64(MaxSize) + 1
	drain := length >= 0 && length <= MaxSize

	return &limitedBody{body: body, left: left, drain: drain}
}

// Close reads what is left of the limited body when its length is known and
// within the limit, then closes the response body. A body read to its end is
// closed at once.
//
// Returns:
//   - error: the error from closing the response body.
func (b *limitedBody) Close() error {
	if b.drain && b.left > 0 {
		_, _ = io.Copy(io.Discard, b) //nolint:errcheck // A failed read only keeps the connection from being reused.
	}

	return b.body.Close() //nolint:wrapcheck // The body's own close error, as from any response body.
}

// Read reads from the body, up to its limit.
//
// Parameters:
//   - buf: the buffer to read into.
//
// Returns:
//   - int: the number of bytes read.
//   - error: [io.EOF] at the end of the body or its limit, or the body's read
//     error.
func (b *limitedBody) Read(buf []byte) (int, error) {
	if b.left <= 0 {
		return 0, io.EOF
	}

	if int64(len(buf)) > b.left {
		buf = buf[:b.left]
	}

	read, err := b.body.Read(buf)
	b.left -= int64(read)

	if errors.Is(err, io.EOF) {
		b.left = 0
	}

	return read, err //nolint:wrapcheck // The body's own read error, as from any response body.
}
