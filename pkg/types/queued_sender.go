package types

// QueuedSender is the interface for a proxied sender that queues messages before sending.
type QueuedSender interface {
	// Enqueuef adds a formatted message to the queue.
	//
	// Parameters:
	//   - format: the format string, as for [fmt.Sprintf].
	//   - v: the values to format.
	Enqueuef(format string, v ...any)
	// Enqueue adds a message to the queue.
	//
	// Parameters:
	//   - message: the message to add.
	Enqueue(message string)
	// Flush sends the queued messages.
	//
	// Parameters:
	//   - params: per-send parameters such as the title, or nil.
	Flush(params *map[string]string)
	// Service returns the service that sends the queued messages.
	//
	// Returns:
	//   - Service: the underlying service.
	Service() Service
}
