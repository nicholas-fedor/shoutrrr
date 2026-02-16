package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// schemeHTTPPrefixLength is the length of "http" in "https", used to strip TLS suffix.
const (
	schemeHTTPPrefixLength = 4
	tokenHintLength        = 3
	httpClientErrorStatus  = 400
	defaultHTTPTimeout     = 10 * time.Second // defaultHTTPTimeout is the timeout for HTTP requests.
)

// ErrUnsupportedLoginFlows indicates that none of the server login flows are supported.
var (
	ErrUnsupportedLoginFlows = errors.New("none of the server login flows are supported")
	ErrUnexpectedStatus      = errors.New("unexpected HTTP status")
)

// client manages interactions with the Matrix API.
type client struct {
	apiURL      url.URL
	AccessToken string
	logger      types.StdLogger
	httpClient  *http.Client
}

// newClient creates a new Matrix client with the specified host and TLS settings.
func newClient(host string, disableTLS bool, logger types.StdLogger) *client {
	client := &client{
		logger: logger,
		apiURL: url.URL{
			Host:   host,
			Scheme: "https",
		},
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}

	if client.logger == nil {
		client.logger = util.DiscardLogger
	}

	if disableTLS {
		client.apiURL.Scheme = client.apiURL.Scheme[:schemeHTTPPrefixLength] // "https" -> "http"
	}

	client.logger.Printf("Using server: %v\n", client.apiURL.String())

	return client
}

// login authenticates the client using a username and password.
func (c *client) login(user string, password string) error {
	c.apiURL.RawQuery = ""

	resLogin := apiResLoginFlows{}
	if err := c.apiReq(apiLogin, nil, &resLogin, http.MethodGet); err != nil {
		return fmt.Errorf("failed to get login flows: %w", err)
	}

	flows := make([]string, 0, len(resLogin.Flows))
	for _, flow := range resLogin.Flows {
		flows = append(flows, string(flow.Type))

		if flow.Type == flowLoginPassword {
			c.logf("Using login flow '%v'", flow.Type)

			return c.loginPassword(user, password)
		}
	}

	return fmt.Errorf("%w: %v", ErrUnsupportedLoginFlows, strings.Join(flows, ", "))
}

// loginPassword performs a password-based login to the Matrix server.
func (c *client) loginPassword(user string, password string) error {
	response := apiResLogin{}
	if err := c.apiReq(apiLogin, apiReqLogin{
		Type:       flowLoginPassword,
		Password:   password,
		Identifier: newUserIdentifier(user),
	}, &response, http.MethodPost); err != nil {
		return fmt.Errorf("failed to log in: %w", err)
	}

	c.AccessToken = response.AccessToken

	tokenHint := ""
	if len(response.AccessToken) > tokenHintLength {
		tokenHint = response.AccessToken[:tokenHintLength]
	}

	c.logf("AccessToken: %v...\n", tokenHint)
	c.logf("HomeServer: %v\n", response.HomeServer)
	c.logf("User: %v\n", response.UserID)
	return nil
}

// sendMessage sends a message to the specified rooms or all joined rooms if none are specified.
func (c *client) sendMessage(message string, rooms []string) []error {
	if len(rooms) > 0 {
		return c.sendToExplicitRooms(rooms, message)
	}

	return c.sendToJoinedRooms(message)
}

// sendToExplicitRooms sends a message to explicitly specified rooms and collects any errors.
func (c *client) sendToExplicitRooms(rooms []string, message string) []error {
	var errors []error

	for _, room := range rooms {
		c.logf("Sending message to '%v'...\n", room)

		roomID, err := c.joinRoom(room)
		if err != nil {
			errors = append(errors, fmt.Errorf("error joining room %v: %w", roomID, err))

			continue
		}

		if room != roomID {
			c.logf("Resolved room alias '%v' to ID '%v'", room, roomID)
		}

		if err := c.sendMessageToRoom(message, roomID); err != nil {
			errors = append(
				errors,
				fmt.Errorf("failed to send message to room '%v': %w", roomID, err),
			)
		}
	}

	return errors
}

// sendToJoinedRooms sends a message to all joined rooms and collects any errors.
func (c *client) sendToJoinedRooms(message string) []error {
	var errors []error

	joinedRooms, err := c.getJoinedRooms()
	if err != nil {
		return append(errors, fmt.Errorf("failed to get joined rooms: %w", err))
	}

	for _, roomID := range joinedRooms {
		c.logf("Sending message to '%v'...\n", roomID)

		if err := c.sendMessageToRoom(message, roomID); err != nil {
			errors = append(
				errors,
				fmt.Errorf("failed to send message to room '%v': %w", roomID, err),
			)
		}
	}

	return errors
}

// joinRoom joins a specified room and returns its ID.
func (c *client) joinRoom(room string) (string, error) {
	resRoom := apiResRoom{}
	if err := c.apiReq(fmt.Sprintf(apiRoomJoin, room), nil, &resRoom, http.MethodPost); err != nil {
		return "", err
	}

	return resRoom.RoomID, nil
}

var txCounter = 0

// sendMessageToRoom sends a message to a specific room.
func (c *client) sendMessageToRoom(message string, roomID string) error {
	resEvent := apiResEvent{}

	txnID := strconv.Itoa(os.Getpid()) + strconv.Itoa(int(time.Now().Unix())) + strconv.Itoa(txCounter)
	url := fmt.Sprintf(apiSendMessage, roomID, txnID)
	txCounter += 1

	return c.apiReq(url, apiReqSend{
		MsgType: msgTypeText,
		Body:    message,
	}, &resEvent, http.MethodPut)
}

// apiPut performs a Put or Post request to the Matrix API.
func (c *client) apiReq(path string, request any, response any, method string) error {
	c.apiURL.Path = path

	var body []byte
	var err error
	if request != nil {
		body, err = json.Marshal(request)
	}
	if err != nil {
		return fmt.Errorf("marshaling %w request: %w", method, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		c.apiURL.String(),
		bytes.NewReader(body),
	)

	if err != nil {
		return fmt.Errorf("creating %w request: %w", method, err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("accept", contentType)
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing %w request: %w", method, err)
	}

	defer func() { _ = res.Body.Close() }()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading %w response body: %w", method, err)
	}

	if res.StatusCode >= httpClientErrorStatus {
		resError := &apiResError{}
		if err = json.Unmarshal(body, resError); err == nil {
			return resError
		}

		return fmt.Errorf("%w: %v (unmarshal error: %w)", method, ErrUnexpectedStatus, res.Status, err)
	}

	if err = json.Unmarshal(body, response); err != nil {
		return fmt.Errorf("unmarshaling %w response: %w", method, err)
	}

	return nil
}

// logf logs a formatted message using the client's logger.
func (c *client) logf(format string, v ...any) {
	c.logger.Printf(format, v...)
}

// getJoinedRooms retrieves the list of rooms the client has joined.
func (c *client) getJoinedRooms() ([]string, error) {
	response := apiResJoinedRooms{}
	if err := c.apiReq(apiJoinedRooms, nil, &response, http.MethodGet); err != nil {
		return []string{}, err
	}

	return response.Rooms, nil
}
