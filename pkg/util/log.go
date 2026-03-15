package util

import (
	"io"
	"log"
)

// DiscardLogger is a logger that discards all output written to it.
// Use this when you need a logger but want to suppress all log output.
var DiscardLogger = log.New(io.Discard, "", 0)
