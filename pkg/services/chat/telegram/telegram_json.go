package telegram

import (
	"fmt"
	"html"
	"strconv"
	"strings"
)

// SendMessagePayload is the body of a sendMessage request.
type SendMessagePayload struct {
	// MessageThreadID is the forum topic to post in, for forum supergroups. It
	// is left out when nil.
	MessageThreadID *int `json:"message_thread_id,omitzero"`
	// ReplyMarkup holds an inline keyboard for the message. It is left out when nil.
	ReplyMarkup *replyMarkup `json:"reply_markup,omitzero"`
	// Text is the message text.
	Text string `json:"text"`
	// ID is the target chat ID or the @username of the target channel.
	ID string `json:"chat_id"`
	// ParseMode is how Telegram formats the text, such as HTML. It is left out
	// for plain text.
	ParseMode string `json:"parse_mode,omitempty"`
	// Entities lists formatting entities in the text, as an alternative to
	// ParseMode. It is left out when empty.
	Entities []entity `json:"entities,omitempty"`
	// ReplyTo is the ID of the message this one replies to, or 0 for none.
	ReplyTo int64 `json:"reply_to_message_id"`
	// MessageID identifies an existing message. sendMessage does not read it,
	// so it is left out when 0.
	MessageID int64 `json:"message_id,omitzero"`
	// DisablePreview turns off the link preview for URLs in the text.
	DisablePreview bool `json:"disable_web_page_preview"`
	// DisableNotification delivers the message without a sound.
	DisableNotification bool `json:"disable_notification"`
}

// Message represents one chat message.
type Message struct {
	// From is the sender, or nil for a message posted in a channel.
	From *User `json:"from"`
	// Chat is the conversation the message belongs to.
	Chat *Chat `json:"chat"`
	// Text is the text of a text message.
	Text string `json:"text"`
	// MessageID is the message's identifier within its chat.
	MessageID int64 `json:"message_id"`
}

// messageResponse is the response to a sendMessage request.
type messageResponse struct {
	// Result is the sent message when OK is true.
	Result *Message `json:"result"`
	// Description explains the failure when OK is false.
	Description string `json:"description"`
	// ErrorCode is the error code of the failure when OK is false.
	ErrorCode int `json:"error_code"`
	// OK reports whether the request succeeded.
	OK bool `json:"ok"`
}

// responseError is the body of a failed Bot API request.
type responseError struct {
	// Description explains the failure.
	Description string `json:"description"`
	// ErrorCode is the error code, usually the HTTP status.
	ErrorCode int `json:"error_code"`
	// OK is false for a failed request.
	OK bool `json:"ok"`
}

// userResponse is the response to a getMe request.
type userResponse struct {
	// Description explains the failure when OK is false.
	Description string `json:"description"`
	// Result is the bot's user when OK is true.
	Result User `json:"result"`
	// ErrorCode is the error code of the failure when OK is false.
	ErrorCode int `json:"error_code"`
	// OK reports whether the request succeeded.
	OK bool `json:"ok"`
}

// User contains information about a telegram user or bot.
type User struct {
	// FirstName is the user's or bot's first name.
	FirstName string `json:"first_name"`
	// LastName is the user's or bot's last name, if set.
	LastName string `json:"last_name"`
	// Username is the user's or bot's username, if set.
	Username string `json:"username"`
	// LanguageCode is the IETF language tag of the user's language, if known.
	LanguageCode string `json:"language_code"`
	// ID is the unique identifier of the user or bot.
	ID int64 `json:"id"`
	// IsBot reports whether the user is a bot.
	IsBot bool `json:"is_bot"`
	// CanJoinGroups reports whether the bot can be invited to groups. Only
	// getMe returns it.
	CanJoinGroups bool `json:"can_join_groups"`
	// CanReadAllGroupMessages reports whether privacy mode is off for the bot.
	// Only getMe returns it.
	CanReadAllGroupMessages bool `json:"can_read_all_group_messages"`
	// SupportsInlineQueries reports whether the bot supports inline queries.
	// Only getMe returns it.
	SupportsInlineQueries bool `json:"supports_inline_queries"`
}

// updatesRequest is the body of a getUpdates request.
type updatesRequest struct {
	// AllowedUpdates lists the update types to receive. Nil leaves the
	// parameter out, so the previous setting stays. An empty list asks for
	// the default types.
	AllowedUpdates []string `json:"allowed_updates,omitzero"`
	// Offset is the identifier of the first update to return. Updates before
	// it are confirmed and no longer returned.
	Offset int `json:"offset"`
	// Limit is the most updates to return, from 1 to 100.
	Limit int `json:"limit"`
	// Timeout is how many seconds to long poll for updates, or 0 for none.
	Timeout int `json:"timeout"`
}

// updatesResponse is the response to a getUpdates request.
type updatesResponse struct {
	// Description explains the failure when OK is false.
	Description string `json:"description"`
	// Result lists the updates when OK is true.
	Result []Update `json:"result"`
	// ErrorCode is the error code of the failure when OK is false.
	ErrorCode int `json:"error_code"`
	// OK reports whether the request succeeded.
	OK bool `json:"ok"`
}

// inlineQuery is an incoming inline query.
type inlineQuery struct {
	// ID is the unique identifier of the query.
	ID string `json:"id"`
	// Query is the text of the query, up to 256 characters.
	Query string `json:"query"`
	// Offset is the offset of the results to return, which the bot controls.
	Offset string `json:"offset"`
	// From is the user who sent the query.
	From User `json:"from"`
}

// chosenInlineResult is an inline query result that a user chose. The
// generator only checks for its presence, so it decodes none of its fields.
type chosenInlineResult struct{}

// Update contains state changes since the previous Update. At most one of the
// optional fields is set.
type Update struct {
	// Message is a new incoming message of any kind, such as text or a photo.
	Message *Message `json:"message"`
	// EditedMessage is a new version of a message the bot knows of.
	EditedMessage *Message `json:"edited_message"`
	// ChannelPost is a new incoming channel post of any kind.
	ChannelPost *Message `json:"channel_post"`
	// EditedChannelPost is a new version of a channel post the bot knows of.
	EditedChannelPost *Message `json:"edited_channel_post"`
	// InlineQuery is a new incoming inline query.
	InlineQuery *inlineQuery `json:"inline_query"`
	// ChosenInlineResult is an inline query result that a user chose and sent
	// to their chat partner.
	ChosenInlineResult *chosenInlineResult `json:"chosen_inline_result"`
	// CallbackQuery is a new incoming callback query.
	CallbackQuery *callbackQuery `json:"callback_query"`
	// ChatMemberUpdate is a change in the bot's own membership in a chat.
	ChatMemberUpdate *ChatMemberUpdate `json:"my_chat_member"`
	// UpdateID is the update's unique identifier. Identifiers increase
	// sequentially, so a getUpdates offset of the last ID plus one confirms
	// every update up to it.
	UpdateID int `json:"update_id"`
}

// Chat represents a telegram conversation.
type Chat struct {
	// Type is the kind of chat: private, group, supergroup, or channel.
	Type string `json:"type"`
	// Title is the title of a group, supergroup, or channel.
	Title string `json:"title"`
	// Username is the username of a private chat, supergroup, or channel, if set.
	Username string `json:"username"`
	// ID is the chat's unique identifier.
	ID int64 `json:"id"`
}

// inlineKey is one button of an inline keyboard. Exactly one of the optional
// fields sets what the button does.
type inlineKey struct {
	// Text is the label on the button.
	Text string `json:"text"`
	// URL is a link the button opens.
	URL string `json:"url"`
	// LoginURL is a URL that logs the user in through Telegram.
	LoginURL string `json:"login_url"`
	// CallbackData is sent to the bot in a callback query when the button is pressed.
	CallbackData string `json:"callback_data"`
	// SwitchInlineQuery starts an inline query with this text in a chat the
	// user picks.
	SwitchInlineQuery string `json:"switch_inline_query"`
	// SwitchInlineQueryCurrent starts an inline query with this text in the
	// current chat.
	SwitchInlineQueryCurrent string `json:"switch_inline_query_current_chat"`
}

// replyMarkup is the reply markup of a message.
type replyMarkup struct {
	// InlineKeyboard holds the rows of buttons shown under the message.
	InlineKeyboard [][]inlineKey `json:"inline_keyboard,omitempty"`
}

// entity marks a formatted span of message text.
type entity struct {
	// Type is the kind of entity, such as bold or url.
	Type string `json:"type"`
	// Offset is where the entity starts, in UTF-16 code units.
	Offset int `json:"offset"`
	// Length is the entity's length, in UTF-16 code units.
	Length int `json:"length"`
}

// callbackQuery is a press of an inline keyboard button.
type callbackQuery struct {
	// ID is the unique identifier of the query.
	ID string `json:"id"`
	// From is the user who pressed the button.
	From *User `json:"from"`
	// Message is the message the button belongs to.
	Message *Message `json:"message"`
	// Data is the button's callback data.
	Data string `json:"data"`
}

// ChatMemberUpdate represents a member update in a telegram chat.
type ChatMemberUpdate struct {
	// Chat is the chat the member belongs to.
	Chat *Chat `json:"chat"`
	// From is the user whose action caused the change.
	From *User `json:"from"`
	// OldChatMember is the member's state before the change.
	OldChatMember *ChatMember `json:"old_chat_member"`
	// NewChatMember is the member's state after the change.
	NewChatMember *ChatMember `json:"new_chat_member"`
	// Date is when the change happened, in Unix time.
	Date int `json:"date"`
}

// ChatMember represents the membership state for a user in a telegram chat.
type ChatMember struct {
	// User is the member.
	User *User `json:"user"`
	// Status is the member's status in the chat, such as administrator or kicked.
	Status string `json:"status"`
}

// Error returns the description of the failure.
//
// Returns:
//   - string: the description Telegram gave.
func (e *responseError) Error() string {
	return e.Description
}

// Name returns the name of the chat: the @username of a private chat or of a
// channel with a username, and the title otherwise.
//
// Returns:
//   - string: the chat's display name.
func (c *Chat) Name() string {
	if c.Type == "private" || c.Type == "channel" && c.Username != "" {
		return "@" + c.Username
	}

	return c.Title
}

// createSendMessagePayload builds the sendMessage body for one chat.
//
// A chat written as <chat ID>:<thread ID> posts in that forum topic. With a
// title and no parse mode, the message is escaped and sent as HTML so the title
// can be bold. Only HTML supports a title.
//
// Parameters:
//   - message: the message text.
//   - channel: the chat ID or @channel, optionally with a thread ID.
//   - config: the configuration for this send.
//
// Returns:
//   - SendMessagePayload: the request body.
func createSendMessagePayload(message, channel string, config *Config) SendMessagePayload {
	var threadID *int

	chatID, thread, ok := strings.Cut(channel, ":")
	if ok {
		if parsed, err := strconv.Atoi(thread); err == nil {
			threadID = &parsed
		}
	}

	payload := SendMessagePayload{
		Text:                message,
		ID:                  chatID,
		MessageThreadID:     threadID,
		DisableNotification: !config.Notification,
		DisablePreview:      !config.Preview,
	}

	parseMode := config.ParseMode
	if config.ParseMode == ParseModes.None && config.Title != "" {
		parseMode = ParseModes.HTML
		// Without a parse mode the message is plain text, so it is escaped to
		// send as HTML.
		message = html.EscapeString(message)
	}

	if parseMode != ParseModes.None {
		payload.ParseMode = parseMode.String()
	}

	// Only the HTML parse mode supports a title.
	if parseMode == ParseModes.HTML {
		payload.Text = fmt.Sprintf("<b>%v</b>\n%v", html.EscapeString(config.Title), message)
	}

	return payload
}
