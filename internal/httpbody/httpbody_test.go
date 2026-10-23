package httpbody_test

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
)

// countingReader counts the bytes read through it.
type countingReader struct {
	// r is the wrapped reader.
	r io.Reader
	// n is the number of bytes read so far.
	n int64
}

// lastChunkReader returns all of its data with an error in a single read,
// as an [io.Reader] may.
type lastChunkReader struct {
	// data is the content returned by the first read.
	data string
	// err is the error returned with the data and by every later read.
	err error
	// done records that the data has been returned.
	done bool
}

// errRead is the failure returned by the readers in TestReadError.
var errRead = errors.New("read failed")

// Read reads from the wrapped reader and counts the bytes.
//
// Parameters:
//   - p: the buffer to read into.
//
// Returns:
//   - int: the number of bytes read.
//   - error: the wrapped reader's error.
func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)

	return n, err
}

// TestRead verifies that a body within the limit is returned whole, and that a
// larger one is cut to the limit and reported as [httpbody.ErrTooLarge] after
// reading one byte past the limit.
func TestRead(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		body     string
		limit    int64
		want     string
		wantRead int64
		tooLarge bool
	}{
		"empty body": {
			body:     "",
			limit:    4,
			want:     "",
			wantRead: 0,
			tooLarge: false,
		},
		"under the limit": {
			body:     "abc",
			limit:    4,
			want:     "abc",
			wantRead: 3,
			tooLarge: false,
		},
		"at the limit": {
			body:     "abcd",
			limit:    4,
			want:     "abcd",
			wantRead: 4,
			tooLarge: false,
		},
		"one byte over the limit": {
			body:     "abcde",
			limit:    4,
			want:     "abcd",
			wantRead: 5,
			tooLarge: true,
		},
		"far over the limit": {
			body:     strings.Repeat("x", 1000),
			limit:    4,
			want:     "xxxx",
			wantRead: 5,
			tooLarge: true,
		},
		"zero limit with a body": {
			body:     "a",
			limit:    0,
			want:     "",
			wantRead: 1,
			tooLarge: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			reader := &countingReader{r: strings.NewReader(tc.body), n: 0}

			got, err := httpbody.Read(reader, tc.limit)

			if tc.tooLarge {
				require.ErrorIs(t, err, httpbody.ErrTooLarge)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.want, string(got))
			assert.Equal(t, tc.wantRead, reader.n)
		})
	}
}

// TestReadError verifies that a read error is returned as is, with the bytes
// read before it, cut to the limit when the error comes with the byte past it.
func TestReadError(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		reader io.Reader
		limit  int64
		want   string
	}{
		"after a partial body": {
			reader: io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(errRead)),
			limit:  httpbody.MaxSize,
			want:   "partial",
		},
		"with the byte past the limit": {
			reader: &lastChunkReader{data: "abcde", err: errRead, done: false},
			limit:  4,
			want:   "abcd",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := httpbody.Read(tc.reader, tc.limit)

			require.ErrorIs(t, err, errRead)
			require.NotErrorIs(t, err, httpbody.ErrTooLarge)
			assert.Equal(t, tc.want, string(got))
		})
	}
}

// Read returns all of the data with the reader's error on the first call, and
// only the error after that.
//
// Parameters:
//   - p: the buffer to read into.
//
// Returns:
//   - int: the number of bytes read.
//   - error: the reader's error.
func (l *lastChunkReader) Read(p []byte) (int, error) {
	if l.done {
		return 0, l.err
	}

	l.done = true

	return copy(p, l.data), l.err
}
