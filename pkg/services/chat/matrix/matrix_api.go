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
	Flows []flow `json:"flows"`
}

// apiReqLogin represents a login request to the Matrix API.
type apiReqLogin struct {
	Type       flowType    `json:"type"`
	Identifier *identifier `json:"identifier"`

	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

// apiResLogin represents the response from a successful Matrix login request.
type apiResLogin struct {
	AccessToken string `json:"access_token"`
	HomeServer  string `json:"home_server"`
	UserID      string `json:"user_id"`
	DeviceID    string `json:"device_id"`
}

// apiReqSend represents a request to send a message to a Matrix room.
type apiReqSend struct {
	MsgType messageType `json:"msgtype"`
	Body    string      `json:"body"`
}

// apiResRoom represents the response from joining a Matrix room.
type apiResRoom struct {
	RoomID string `json:"room_id"`
}

// apiResJoinedRooms represents the response containing the list of joined rooms.
type apiResJoinedRooms struct {
	Rooms []string `json:"joined_rooms"`
}

// apiResEvent represents the response from sending a message event.
type apiResEvent struct {
	EventID string `json:"event_id"`
}

// apiResError represents an error response from the Matrix API.
type apiResError struct {
	Message string `json:"error"`
	Code    string `json:"errcode"`
}

// flow represents a Matrix login flow type.
type flow struct {
	Type flowType `json:"type"`
}

// identifier represents a Matrix user identifier for authentication.
type identifier struct {
	Type identifierType `json:"type"`
	User string         `json:"user,omitempty"`
}

const (
	// Matrix API endpoint paths.
	apiLogin       = "/_matrix/client/v3/login"
	apiRoomJoin    = "/_matrix/client/v3/join/%s"
	apiSendMessage = "/_matrix/client/v3/rooms/%s/send/m.room.message/%s"
	apiJoinedRooms = "/_matrix/client/v3/joined_rooms"

	contentType = "application/json"

	accessTokenKey = "access_token"

	// msgTypeText is the Matrix message type for plain text messages.
	msgTypeText messageType = "m.text"
	// flowLoginPassword is the Matrix login flow type for password authentication.
	flowLoginPassword flowType = "m.login.password"
	//nolint:gosec // This is a Matrix API constant, not a hardcoded credential
	flowLoginToken flowType = "m.login.token"
	// idTypeUser is the Matrix identifier type for username-based authentication.
	idTypeUser identifierType = "m.id.user"

	// Matrix error code for rate limiting.
	errCodeLimitExceeded = "M_LIMIT_EXCEEDED"
)

// Error returns the error message from the Matrix API error response.
func (e *apiResError) Error() string {
	return e.Message
}

// IsRateLimited checks if the error is a rate limiting error (M_LIMIT_EXCEEDED).
func (e *apiResError) IsRateLimited() bool {
	return e.Code == errCodeLimitExceeded
}

// newUserIdentifier creates a new Matrix user identifier for authentication.
func newUserIdentifier(user string) *identifier {
	return &identifier{
		Type: idTypeUser,
		User: user,
	}
}
