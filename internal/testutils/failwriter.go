package testutils

import (
	"errors"
	"fmt"
	"io"
)

type failWriter struct {
	writeLimit int
	writeCount int
}

var ErrWriteLimitReached = errors.New("reached write limit")

// Close is just a dummy function to implement io.Closer.
func (fw *failWriter) Close() error {
	return nil
}

// Write returns an error if the write limit has been reached.
func (fw *failWriter) Write(data []byte) (int, error) {
	fw.writeCount++
	if fw.writeCount > fw.writeLimit {
		return 0, fmt.Errorf("%w: %d", ErrWriteLimitReached, fw.writeLimit)
	}

	return len(data), nil
}

// CreateFailWriter returns a io.WriteCloser that returns an error after the amount of writes indicated by writeLimit.
func CreateFailWriter(writeLimit int) io.WriteCloser {
	return &failWriter{
		writeLimit: writeLimit,
	}
}
