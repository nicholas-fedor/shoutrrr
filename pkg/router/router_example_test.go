package router

import (
	"log"
	"os"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// ExampleNewWithOptions sends a message through a router with default options.
func ExampleNewWithOptions() {
	logger := log.New(os.Stdout, "", 0)

	sr, err := NewWithOptions(logger, types.SenderOptions{}, "logger://")
	if err != nil {
		log.Fatalf("could not create router: %s", err)
	}

	sr.Send("hello", nil)
	// Output: hello
}

// ExampleServiceRouter_Enqueue queues messages and sends them when the deferred
// Flush runs.
func ExampleServiceRouter_Enqueue() {
	logger := log.New(os.Stdout, "", 0)

	sr, err := NewWithOptions(logger, types.SenderOptions{}, "logger://")
	if err != nil {
		log.Fatalf("could not create router: %s", err)
	}

	defer sr.Flush(nil)

	sr.Enqueue("hello")
	sr.Enqueue("world")
	// Output:
	// hello
	// world
}
