package types

import (
	"strings"
	"time"
)

// MessageLevel is used to denote the urgency of a message item.
type MessageLevel uint8

// File represents a file attachment for a message.
type File struct {
	// Name is the file name of the attachment.
	Name string
	// Data is the content of the file.
	Data []byte
}

// MessageItem is an entry in a notification being sent by a service.
type MessageItem struct {
	// Timestamp is the time the item refers to. The zero time means none.
	Timestamp time.Time
	// File is an attachment for the item, or nil for none.
	File *File
	// Text is the body of the item.
	Text string
	// Fields holds extra key/value data for the item.
	Fields []Field
	// Level is the urgency of the item. [Unknown] means none.
	Level MessageLevel
}

// Message levels. Each level after [Unknown] is more urgent than the one
// before it.
const (
	// Unknown is the default message level.
	Unknown MessageLevel = iota
	// Debug is the lowest kind of known message level.
	Debug
	// Info is generally used as the "normal" message level.
	Info
	// Warning is generally used to denote messages that might be OK, but can cause problems.
	Warning
	// Error is generally used for messages about things that did not go as planned.
	Error
	// messageLevelCount is the number of message levels.
	messageLevelCount
)

// MessageLevelCount is used to create arrays that maps levels to other values.
const MessageLevelCount = int(messageLevelCount)

// messageLevelStrings holds the name of each message level, indexed by
// level.
var messageLevelStrings = [MessageLevelCount]string{
	"Unknown",
	"Debug",
	"Info",
	"Warning",
	"Error",
}

// String returns the string representation of the message level.
//
// Returns:
//   - string: the level name, or "Unknown" for a level out of range.
func (lvl MessageLevel) String() string {
	if lvl >= messageLevelCount {
		return messageLevelStrings[0]
	}

	return messageLevelStrings[lvl]
}

// ParseMessageLevel parses a string into a MessageLevel.
//
// The match is case-insensitive. If the input does not match a known level
// name, then Unknown is returned with false.
//
// Parameters:
//   - s: the string to parse.
//
// Returns:
//   - MessageLevel: the parsed level.
//   - bool: true if the parse succeeded.
func ParseMessageLevel(s string) (MessageLevel, bool) {
	for i, name := range messageLevelStrings {
		if strings.EqualFold(name, s) {
			return MessageLevel(i), true
		}
	}

	return Unknown, false
}

// WithField appends the key/value pair to the message items fields.
//
// Parameters:
//   - key: the field key.
//   - value: the field value.
//
// Returns:
//   - *MessageItem: the receiver for chaining.
func (mi *MessageItem) WithField(key, value string) *MessageItem {
	mi.Fields = append(mi.Fields, Field{
		Key:   key,
		Value: value,
	})

	return mi
}

// ItemsToPlain joins together the MessageItems' Text using newlines.
//
// It is used by ServiceRouter.SendItems as the fallback for services that do
// not implement RichSender; only Text is preserved, while Level, Timestamp,
// Fields, and File are discarded.
//
// Parameters:
//   - items: the message items to convert.
//
// Returns:
//   - string: the concatenated plain text.
func ItemsToPlain(items []MessageItem) string {
	builder := strings.Builder{}
	for _, item := range items {
		builder.WriteString(item.Text)
		builder.WriteRune('\n')
	}

	return builder.String()
}
