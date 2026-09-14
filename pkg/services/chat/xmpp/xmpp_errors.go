package xmpp

import "errors"

var (
	// ErrMissingHost is returned when the configuration URL has no hostname.
	ErrMissingHost = errors.New("host is required")

	// ErrMissingUser is returned when the configuration URL has no auth user.
	ErrMissingUser = errors.New("user is required")

	// ErrMissingPassword is returned when the configuration URL has no password.
	ErrMissingPassword = errors.New("password is required")

	// ErrMissingTargets is returned when neither to nor rooms is set.
	ErrMissingTargets = errors.New("at least one to or rooms target is required")

	// ErrDisableTLSOnXMPPS is returned when disabletls is set on an xmpps:// URL.
	ErrDisableTLSOnXMPPS = errors.New("disabletls is not allowed with xmpps://")

	// ErrUnsupportedScheme is returned when the URL scheme is not xmpp or xmpps.
	ErrUnsupportedScheme = errors.New("unsupported URL scheme")

	// ErrInvalidUserJID is returned when the auth user is not a valid JID.
	ErrInvalidUserJID = errors.New("user is not a valid JID")

	// ErrInvalidRecipientJID is returned when a to= value is not a valid JID.
	ErrInvalidRecipientJID = errors.New("recipient is not a valid JID")

	// ErrInvalidRoomJID is returned when a rooms= value is not a valid JID.
	ErrInvalidRoomJID = errors.New("room is not a valid JID")

	// ErrAuthenticationFailed is returned when SASL authentication is rejected.
	ErrAuthenticationFailed = errors.New("authentication failed")
)
