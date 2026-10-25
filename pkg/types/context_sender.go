package types

import "context"

// ContextSender is the interface for services that support context-aware sending.
// Services that implement this interface will receive the caller's context,
// enabling cancellation and deadline propagation.
type ContextSender interface {
	// SendContext sends a message, honoring the cancellation and deadline of
	// ctx.
	//
	// The router passes a context that ends when the caller's context ends or
	// the service's send budget runs out.
	//
	// Parameters:
	//   - ctx: cancellation and deadline for the send.
	//   - message: the message to send.
	//   - params: per-send parameters such as the title, or nil.
	//
	// Returns:
	//   - error: the send failure, or nil on success.
	SendContext(ctx context.Context, message string, params *Params) error
}

// ContextAttachmentSender is the interface for services that support context-aware
// rich sending with structured message items.
type ContextAttachmentSender interface {
	// SendItemsContext sends structured message items, honoring the
	// cancellation and deadline of ctx.
	//
	// When the router sends message items, it prefers this method over the
	// service's other send methods.
	//
	// Parameters:
	//   - ctx: cancellation and deadline for the send.
	//   - items: the message items to send.
	//   - params: per-send parameters such as the title.
	//
	// Returns:
	//   - error: the send failure, or nil on success.
	SendItemsContext(ctx context.Context, items []MessageItem, params Params) error
}
