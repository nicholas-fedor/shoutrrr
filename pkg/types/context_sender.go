package types

import "context"

// ContextSender is the interface for services that support context-aware sending.
// Services that implement this interface will receive the caller's context,
// enabling cancellation and deadline propagation.
type ContextSender interface {
	SendContext(ctx context.Context, message string, params *Params) error
}

// ContextAttachmentSender is the interface for services that support context-aware
// rich sending with structured message items.
type ContextAttachmentSender interface {
	SendItemsContext(ctx context.Context, items []MessageItem, params Params) error
}
