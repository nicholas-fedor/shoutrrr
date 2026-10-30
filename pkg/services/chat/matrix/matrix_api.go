package matrix

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
	// Type is the login flow, password or token.
	Type flowType `json:"type"`
	// Identifier identifies the user for a password login.
	Identifier *identifier `json:"identifier"`

	// Password is the user's password, left out for a token login.
	Password string `json:"password,omitempty"`
	// Token is the login token, left out for a password login.
	Token string `json:"token,omitempty"`
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
	// User is the user name or full Matrix ID, left out when empty.
	User string `json:"user,omitempty"`
}

// Matrix client-server API paths, request settings, and protocol values.
const (
	// apiLogin is the login endpoint path.
	apiLogin = "/_matrix/client/v3/login"
	// apiRoomJoin is the room join endpoint path, formatted with the room ID
	// or alias.
	apiRoomJoin = "/_matrix/client/v3/join/%s"
	// apiSendMessage is the message send endpoint path, formatted with the
	// room ID and a transaction ID.
	apiSendMessage = "/_matrix/client/v3/rooms/%s/send/m.room.message/%s"
	// apiJoinedRooms is the joined rooms endpoint path.
	apiJoinedRooms = "/_matrix/client/v3/joined_rooms"

	// contentType is the JSON encoding the API takes.
	contentType = "application/json"

	// msgTypeText is the Matrix message type for plain text messages.
	msgTypeText messageType = "m.text"
	// flowLoginPassword is the Matrix login flow type for password authentication.
	flowLoginPassword flowType = "m.login.password"
	// flowLoginToken is the Matrix login flow type for token authentication.
	//nolint:gosec // This is a Matrix API constant, not a hardcoded credential
	flowLoginToken flowType = "m.login.token"
	// idTypeUser is the Matrix identifier type for username-based authentication.
	idTypeUser identifierType = "m.id.user"
)

// Error returns the error message from the Matrix API error response.
//
// Returns:
//   - string: the message.
func (e *apiResError) Error() string {
	return e.Message
}

// newUserIdentifier creates a new Matrix user identifier for authentication.
//
// Parameters:
//   - user: the user name or full Matrix ID.
//
// Returns:
//   - *identifier: the m.id.user identifier.
func newUserIdentifier(user string) *identifier {
	return &identifier{
		Type: idTypeUser,
		User: user,
	}
}
