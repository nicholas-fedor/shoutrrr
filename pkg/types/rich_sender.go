package types

// RichSender is the interface for services that accept structured message items.
// When ServiceRouter.SendItems is called, it type-asserts each service to this
// interface. Services that implement RichSender receive the full []MessageItem
// slice, preserving fields, timestamps, and file attachments. Services that do
// not implement it fall back to plain text via ItemsToPlain.
type RichSender interface {
	SendItems(items []MessageItem, params Params) error
}
