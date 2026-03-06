package matrix

type (
	messageType    string
	flowType       string
	identifierType string
)

type apiResLoginFlows struct {
	Flows []flow `json:"flows"`
}

type apiReqLogin struct {
	Type       flowType    `json:"type"`
	Identifier *identifier `json:"identifier"`
	//nolint:gosec // Password is an API struct field for Matrix protocol authentication
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

type apiResLogin struct {
	//nolint:gosec // AccessToken is an API struct field for Matrix protocol authentication
	AccessToken string `json:"access_token"`
	HomeServer  string `json:"home_server"`
	UserID      string `json:"user_id"`
	DeviceID    string `json:"device_id"`
}

type apiReqSend struct {
	MsgType messageType `json:"msgtype"`
	Body    string      `json:"body"`
}

type apiResRoom struct {
	RoomID string `json:"room_id"`
}

type apiResJoinedRooms struct {
	Rooms []string `json:"joined_rooms"`
}

type apiResEvent struct {
	EventID string `json:"event_id"`
}

type apiResError struct {
	Message string `json:"error"`
	Code    string `json:"errcode"`
}

type flow struct {
	Type flowType `json:"type"`
}

type identifier struct {
	Type identifierType `json:"type"`
	User string         `json:"user,omitempty"`
}

const (
	apiLogin       = "/_matrix/client/r0/login"
	apiRoomJoin    = "/_matrix/client/r0/join/%s"
	apiSendMessage = "/_matrix/client/r0/rooms/%s/send/m.room.message"
	apiJoinedRooms = "/_matrix/client/r0/joined_rooms"

	contentType = "application/json"

	accessTokenKey = "access_token"

	msgTypeText       messageType    = "m.text"
	flowLoginPassword flowType       = "m.login.password"
	idTypeUser        identifierType = "m.id.user"
)

func (e *apiResError) Error() string {
	return e.Message
}

func newUserIdentifier(user string) *identifier {
	return &identifier{
		Type: idTypeUser,
		User: user,
	}
}
