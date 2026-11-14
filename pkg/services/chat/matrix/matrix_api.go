package matrix

import "errors"

// Type definitions for Matrix API message types.
type (
	// messageType represents the type of a Matrix message.
	messageType string
	// flowType represents the type of a Matrix login flow.
	flowType string
	// identifierType represents the type of Matrix user identifier.
	identifierType string
)

// apiResLoginFlows represents the response from the Matrix login flows endpoint.
type apiResLoginFlows struct {
	// Flows lists the login flows the homeserver supports.
	Flows []flow `json:"flows"`
}

// apiReqLogin represents a login request to the Matrix API.
type apiReqLogin struct {
	// Type is the login flow.
	Type flowType `json:"type"`
	// Identifier identifies the user.
	Identifier identifier `json:"identifier"`

	// Password is the user's password.
	Password string `json:"password"`
	// DeviceID reuses this device ID, so repeated logins do not add devices,
	// and is left out when empty.
	DeviceID string `json:"device_id,omitempty"`
}

// apiResLogin represents the response from a successful Matrix login request.
type apiResLogin struct {
	// AccessToken authenticates the later requests.
	AccessToken string `json:"access_token"`
	// HomeServer is the homeserver's server name.
	HomeServer string `json:"home_server"`
	// UserID is the logged-in user's full Matrix ID.
	UserID string `json:"user_id"`
	// DeviceID is the ID of the device the login created or reused.
	DeviceID string `json:"device_id"`
}

// apiReqSend represents a request to send a message to a Matrix room.
type apiReqSend struct {
	// MsgType is the message type, m.text for plain text.
	MsgType messageType `json:"msgtype"`
	// Body is the message text.
	Body string `json:"body"`
}

// apiResRoom represents the response from joining a Matrix room.
type apiResRoom struct {
	// RoomID is the ID of the joined room.
	RoomID string `json:"room_id"`
}

// apiResJoinedRooms represents the response containing the list of joined rooms.
type apiResJoinedRooms struct {
	// Rooms lists the IDs of the rooms the user has joined.
	Rooms []string `json:"joined_rooms"`
}

// apiResVersions represents the response listing the specification versions
// the homeserver supports.
type apiResVersions struct {
	// Versions lists the supported versions, such as v1.15.
	Versions []string `json:"versions"`
}

// apiResEvent represents the response from sending a message event.
type apiResEvent struct {
	// EventID is the ID of the sent message event.
	EventID string `json:"event_id"`
}

// apiResError represents an error response from the Matrix API.
type apiResError struct {
	// Message describes the error.
	Message string `json:"error"`
	// Code is the Matrix error code, such as M_FORBIDDEN.
	Code string `json:"errcode"`
	// RetryAfterMS is how many milliseconds a rate-limited client waits before
	// it tries again. The spec deprecates it in favor of the Retry-After
	// header.
	RetryAfterMS int64 `json:"retry_after_ms"`
}

// flow represents a Matrix login flow type.
type flow struct {
	// Type is the login flow type, such as m.login.password.
	Type flowType `json:"type"`
}

// identifier represents a Matrix user identifier for authentication.
type identifier struct {
	// Type is the identifier type, m.id.user for a user name.
	Type identifierType `json:"type"`
	// User is the user name or full Matrix ID.
	User string `json:"user"`
}

// Matrix client-server API paths, request settings, and protocol values.
const (
	// apiLogin is the login endpoint path.
	apiLogin = "/_matrix/client/v3/login"
	// apiRoomJoin is the join endpoint, /_matrix/client/v3/join/{roomIdOrAlias},
	// without the escaped room.
	apiRoomJoin = "/_matrix/client/v3/join/"
	// apiRooms starts the message send endpoint,
	// /_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId}, before the
	// escaped room ID.
	apiRooms = "/_matrix/client/v3/rooms/"
	// apiSendMessage is the part of the message send endpoint between the room
	// ID and the transaction ID.
	apiSendMessage = "/send/m.room.message/"
	// apiJoinedRooms is the joined rooms endpoint path.
	apiJoinedRooms = "/_matrix/client/v3/joined_rooms"
	// apiVersions is the endpoint that lists the specification versions the
	// homeserver supports. It takes no access token.
	apiVersions = "/_matrix/client/versions"

	// contentType is the JSON encoding the API takes.
	contentType = "application/json"

	// msgTypeText is the Matrix message type for plain text messages.
	msgTypeText messageType = "m.text"
	// flowLoginPassword is the Matrix login flow type for password authentication.
	flowLoginPassword flowType = "m.login.password"
	// idTypeUser is the Matrix identifier type for username-based authentication.
	idTypeUser identifierType = "m.id.user"

	// errCodeUnknownToken is the error code of a request whose access token
	// the homeserver does not recognize. The homeserver answers so for a token
	// that expired or was revoked, and for one that another login to the same
	// device replaced.
	errCodeUnknownToken = "M_UNKNOWN_TOKEN"
	// errCodeUnrecognized is the error code of a request for an endpoint the
	// homeserver does not support. A homeserver that offers only the OAuth 2.0
	// login API answers the login flows request so.
	errCodeUnrecognized = "M_UNRECOGNIZED"

	// roomIDSigil starts every Matrix room ID.
	roomIDSigil = "!"
	// aliasSigil starts every Matrix room alias.
	aliasSigil = "#"
)

// Error returns the error message from the Matrix API error response.
//
// Returns:
//   - string: the message.
func (e *apiResError) Error() string {
	return e.Message
}

// hasErrorCode reports whether err is the homeserver's error with the Matrix
// error code code.
//
// Parameters:
//   - err: the error of a request.
//   - code: the Matrix error code, such as M_UNKNOWN_TOKEN.
//
// Returns:
//   - bool: whether err is a Matrix error with that code.
func hasErrorCode(err error, code string) bool {
	resError, ok := errors.AsType[*apiResError](err)

	return ok && resError.Code == code
}
