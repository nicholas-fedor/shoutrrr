package types

// Sender is the interface needed to implement to send notifications.
type Sender interface {
	// Send sends a notification message.
	//
	// Parameters:
	//   - message: the message to send.
	//   - params: per-send parameters such as the title, or nil.
	//
	// Returns:
	//   - error: the send failure, such as [ErrServiceNotInitialized] when
	//     the service has not been initialized.
	Send(message string, params *Params) error
}
