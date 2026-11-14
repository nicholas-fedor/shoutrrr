package matrix

import "errors"

// Error definitions for the Matrix service.
//
// These errors are returned by various methods in the Matrix service package
// to indicate specific failure conditions during configuration, authentication,
// and message sending operations.
var (
	// ErrMissingHost is returned when the Matrix server host is not provided
	// in the service configuration URL.
	ErrMissingHost = errors.New("host is required")

	// ErrMissingCredentials is returned when neither a password nor an access
	// token is provided in the service configuration URL.
	ErrMissingCredentials = errors.New("password or access token is required")

	// ErrClientNotInitialized is returned when a send finds no homeserver to
	// send to, because Initialize failed or the Config was set without it.
	ErrClientNotInitialized = errors.New("client not initialized; cannot send message")

	// ErrInvalidRoomID is returned when a room ID in the homeserver's reply to
	// a join or a joined rooms lookup does not start with the ! sigil that
	// every Matrix room ID starts with.
	ErrInvalidRoomID = errors.New("room ID lacks the ! sigil")

	// ErrLoginReplyWithoutToken is returned when the homeserver's reply to a
	// login holds no access token, which the Matrix API requires in every
	// successful login reply.
	ErrLoginReplyWithoutToken = errors.New("login reply has no access token")

	// ErrUnsupportedLoginFlows is returned when a send with a user and
	// password finds that the homeserver offers no password login, such as one
	// that signs users in only through single sign-on or only through the
	// OAuth 2.0 login API. A service URL with an access token in place of the
	// password sends without logging in.
	ErrUnsupportedLoginFlows = errors.New("homeserver offers no password login, so use an access token")

	// ErrUnexpectedStatus is returned when the Matrix API returns an HTTP
	// status code that was not expected by the client.
	ErrUnexpectedStatus = errors.New("unexpected HTTP status")

	// ErrNoJoinedRooms is returned when a send without configured rooms finds
	// that the account has joined no room, so the message would reach no one.
	ErrNoJoinedRooms = errors.New("no joined rooms to send to")

	// ErrEmptyRoom is returned when a room value is empty or contains only whitespace.
	ErrEmptyRoom = errors.New("room value is empty")
)
