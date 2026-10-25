package matrix

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// HTTPClient defines the interface for HTTP operations.
//
// Deprecated: The service takes a [types.HTTPClient], which has the same
// method. Use [types.HTTPClient].
type HTTPClient interface {
	// Do sends req and returns its response.
	Do(req *http.Request) (*http.Response, error)
}

// client manages interactions with the Matrix API.
type client struct {
	// logger receives the client's progress messages.
	logger types.StdLogger
	// httpClient holds the service's HTTP client, which sends the API
	// requests.
	httpClient *httpclient.Holder
	// accessToken authenticates the requests after login, or is empty before.
	accessToken string
	// apiURL is the homeserver's base URL.
	apiURL url.URL
}

// DefaultHTTPClient is an HTTP client whose requests time out after
// [defaultHTTPTimeout]. The zero value is ready to use.
//
// Deprecated: The service builds its own default client and redacts the
// errors of every client. Inject an *http.Client with [Service.SetHTTPClient]
// instead.
type DefaultHTTPClient struct {
	// client sends the requests, or is nil to send through a new client with
	// [defaultHTTPTimeout] as its timeout.
	client *http.Client
}

// Client settings, protocol values, and ID generation parameters.
const (
	// schemeHTTPS is the URL scheme for HTTPS.
	schemeHTTPS = "https"

	// schemeHTTP is the URL scheme for HTTP.
	schemeHTTP = "http"

	// tokenHintLength is the length of the token hint shown in logs.
	tokenHintLength = 3

	// minSliceLength is the minimum length for slice operations.
	minSliceLength = 1

	// httpClientErrorStatus is the HTTP status code threshold for client errors.
	httpClientErrorStatus = 400

	// defaultHTTPTimeout is the part of the send budget for each request, and
	// the timeout of [DefaultHTTPClient].
	defaultHTTPTimeout = 10 * time.Second

	// transactionIDRandLen is the length of random bytes in transaction ID.
	transactionIDRandLen = 8

	// byteMask is the mask for extracting a single byte from an integer.
	byteMask = 0xFF

	// bitsPerByte is the number of bits in a byte.
	bitsPerByte = 8

	// authorizationHeader is the HTTP header name for Bearer token authentication.
	authorizationHeader = "Authorization"

	// bearerPrefix is the prefix for Bearer token authentication.
	bearerPrefix = "Bearer "

	// deviceIDLength is the length of the generated device ID.
	// The Matrix spec recommends 10 characters for sufficient uniqueness.
	deviceIDLength = 10

	// deviceIDPrefix is the prefix for generated device IDs.
	// This aids identification in Matrix admin interfaces.
	deviceIDPrefix = "SH"

	// deviceIDHashBytes is the number of hash bytes used for device ID generation.
	// 8 bytes from SHA-256 provides enough characters for the suffix.
	deviceIDHashBytes = 8

	// opaqueIDCharset is the character set allowed by the Matrix Opaque Identifier Grammar.
	// Reference: https://spec.matrix.org/v1.18/appendices/#opaque-identifiers
	opaqueIDCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	// opaqueIDCharsetLen is the number of characters in opaqueIDCharset.
	opaqueIDCharsetLen = byte(len(opaqueIDCharset))
)

// Do performs the HTTP request.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: the response.
//   - error: the client's error, with its URL redacted.
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	client := &http.Client{Timeout: defaultHTTPTimeout}
	if c != nil && c.client != nil {
		client = c.client
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %w", redact.URLError(err))
	}

	return resp, nil
}

// apiDo performs an HTTP request to the Matrix API.
//
// Parameters:
//   - ctx: bounds the request.
//   - method: the HTTP method.
//   - path: the API path.
//   - request: the value sent as the JSON request body. A nil value is sent as
//     null.
//   - response: the value the JSON response is decoded into. A nil value
//     makes decoding a successful response fail.
//
// Returns:
//   - error: the request error or the API's error, logged first when it is
//     a rate limit error.
func (c *client) apiDo(ctx context.Context, method, path string, request, response any) error {
	err := c.doSingleRequest(ctx, method, path, request, response)
	if err == nil {
		return nil
	}

	// If it's a M_LIMIT_EXCEEDED rate limited error, propagate it immediately.
	// This allows the caller to handle rate limiting as appropriate
	if isRateLimitedError(err) {
		c.logf("Rate limited, returning error to caller: %v\n", err)

		return err
	}

	// For non-retryable errors, return immediately
	return err
}

// apiGet performs a GET request to the Matrix API with the provided context.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - path: the API path to request.
//   - response: the value the JSON response is decoded into.
//
// Returns:
//   - error: the Matrix API error for an error status, [ErrUnexpectedStatus]
//     for an error status without one, or the request, read, or decoding error.
//     Reading stops at [httpbody.MaxSize]. An error status is reported from the
//     bytes read, and an oversized success body wraps [httpbody.ErrTooLarge].
func (c *client) apiGet(ctx context.Context, path string, response any) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.buildURL(path),
		http.NoBody,
	)
	if err != nil {
		return fmt.Errorf("creating GET request: %w", redact.URLError(err))
	}

	c.setAuthorizationHeader(req)

	res, err := c.httpClient.Client().Do(req)
	if err != nil {
		return fmt.Errorf("executing GET request: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	body, readErr := httpbody.Read(res.Body, httpbody.MaxSize)
	if readErr != nil && !errors.Is(readErr, httpbody.ErrTooLarge) {
		return fmt.Errorf("reading GET response body: %w", readErr)
	}

	// An error status is reported from the first bytes of an oversized body.
	if res.StatusCode >= httpClientErrorStatus {
		resError := &apiResError{}
		if err = json.Unmarshal(body, resError, jsonopts.Default); err == nil {
			return resError
		}

		return fmt.Errorf(
			"%w: %v (unmarshal error: %w)",
			ErrUnexpectedStatus,
			res.Status, err,
		)
	}

	if readErr != nil {
		return fmt.Errorf("reading GET response body: %w", readErr)
	}

	if err = json.Unmarshal(body, response, jsonopts.Default); err != nil {
		return fmt.Errorf("unmarshaling GET response: %w", err)
	}

	return nil
}

// apiPost performs a POST request to the Matrix API with the provided context.
//
// Parameters:
//   - ctx: bounds the request.
//   - path: the API path.
//   - request: the request body, encoded as JSON.
//   - response: decodes the response body.
//
// Returns:
//   - error: the request error or the API's error.
func (c *client) apiPost(ctx context.Context, path string, request, response any) error {
	return c.apiDo(ctx, http.MethodPost, path, request, response)
}

// apiPut performs a PUT request to the Matrix API with the provided context.
//
// Parameters:
//   - ctx: bounds the request.
//   - path: the API path.
//   - request: the request body, encoded as JSON.
//   - response: decodes the response body.
//
// Returns:
//   - error: the request error or the API's error.
func (c *client) apiPut(ctx context.Context, path string, request, response any) error {
	return c.apiDo(ctx, http.MethodPut, path, request, response)
}

// buildURL returns a copy of the base API URL with the specified path set.
//
// Parameters:
//   - path: the API path.
//
// Returns:
//   - string: the request URL.
func (c *client) buildURL(path string) string {
	u := c.apiURL
	u.Path = path

	return u.String()
}

// doSingleRequest performs a single HTTP request without retry logic.
//
// Parameters:
//   - ctx: cancellation for the request, further bounded by the client's timeout.
//   - method: the HTTP method.
//   - path: the API path to request.
//   - request: the value sent as the JSON request body.
//   - response: the value the JSON response is decoded into.
//
// Returns:
//   - error: the Matrix API error for an error status, [ErrUnexpectedStatus]
//     for an error status without one, or the request, read, or decoding error.
//     Reading stops at [httpbody.MaxSize]. An error status is reported from the
//     bytes read, and an oversized success body wraps [httpbody.ErrTooLarge].
func (c *client) doSingleRequest(ctx context.Context, method, path string, request, response any) error {
	body, err := json.Marshal(request, jsonopts.Default)
	if err != nil {
		return fmt.Errorf("marshaling %s request: %w", method, err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		c.buildURL(path),
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("creating %s request: %w", method, redact.URLError(err))
	}

	req.Header.Set("Content-Type", contentType)
	c.setAuthorizationHeader(req)

	res, err := c.httpClient.Client().Do(req)
	if err != nil {
		return fmt.Errorf("executing %s request: %w", method, redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	body, readErr := httpbody.Read(res.Body, httpbody.MaxSize)
	if readErr != nil && !errors.Is(readErr, httpbody.ErrTooLarge) {
		return fmt.Errorf("reading %s response body: %w", method, readErr)
	}

	// An error status is reported from the first bytes of an oversized body.
	if res.StatusCode >= httpClientErrorStatus {
		resError := &apiResError{}
		if err = json.Unmarshal(body, resError, jsonopts.Default); err == nil {
			return resError
		}

		return fmt.Errorf(
			"%w: %v (unmarshal error: %w)",
			ErrUnexpectedStatus,
			res.Status,
			err,
		)
	}

	if readErr != nil {
		return fmt.Errorf("reading %s response body: %w", method, readErr)
	}

	if err = json.Unmarshal(body, response, jsonopts.Default); err != nil {
		return fmt.Errorf("unmarshaling %s response: %w", method, err)
	}

	return nil
}

// getJoinedRooms retrieves the list of rooms the client has joined.
//
// Parameters:
//   - ctx: bounds the request.
//
// Returns:
//   - []string: the IDs of the joined rooms.
//   - error: the request error or the API's error.
func (c *client) getJoinedRooms(ctx context.Context) ([]string, error) {
	response := apiResJoinedRooms{}

	err := c.apiGet(ctx, apiJoinedRooms, &response)
	if err != nil {
		return []string{}, err
	}

	return response.Rooms, nil
}

// joinRoom joins a specified room and returns its ID.
//
// Parameters:
//   - ctx: bounds the request.
//   - room: the room alias or ID.
//
// Returns:
//   - string: the room ID.
//   - error: the request error or the API's error.
func (c *client) joinRoom(ctx context.Context, room string) (string, error) {
	resRoom := apiResRoom{}

	err := c.apiPost(
		ctx,
		fmt.Sprintf(apiRoomJoin, room),
		&struct{}{},
		&resRoom,
	)
	if err != nil {
		return "", err
	}

	return resRoom.RoomID, nil
}

// logf logs a formatted message using the client's logger.
//
// Parameters:
//   - format: the message format.
//   - v: the format arguments.
func (c *client) logf(format string, v ...any) {
	c.logger.Printf(format, v...)
}

// login authenticates the client using a username and password.
//
// It asks the homeserver for its login flows and uses the password flow when
// a user is set, or the token flow with password as the token when none is.
//
// Parameters:
//   - ctx: bounds the requests.
//   - user: the user name, or empty for a token login.
//   - password: the password, or the login token when user is empty.
//
// Returns:
//   - error: the request error, the API's error, or [ErrUnsupportedLoginFlows]
//     when the homeserver offers no flow that fits.
func (c *client) login(ctx context.Context, user, password string) error {
	c.apiURL.RawQuery = ""

	resLogin := apiResLoginFlows{}
	if err := c.apiGet(ctx, apiLogin, &resLogin); err != nil {
		return fmt.Errorf("failed to get login flows: %w", err)
	}

	flows := make([]string, 0, len(resLogin.Flows))
	for _, flow := range resLogin.Flows {
		flows = append(flows, string(flow.Type))

		// Prefer password login when a user is configured
		if flow.Type == flowLoginPassword && user != "" {
			c.logf("Using login flow '%v'", flow.Type)

			return c.loginPassword(ctx, user, password)
		}

		// Only use token login when no user is configured
		if flow.Type == flowLoginToken && user == "" {
			c.logf("Using login flow '%v'", flow.Type)

			return c.loginToken(ctx, password)
		}
	}

	return fmt.Errorf(
		"%w: %v",
		ErrUnsupportedLoginFlows,
		strings.Join(flows, ", "),
	)
}

// loginPassword performs a password-based login to the Matrix server.
//
// Parameters:
//   - ctx: bounds the request.
//   - user: the user name or full Matrix ID.
//   - password: the password.
//
// Returns:
//   - error: the request error or the API's error.
func (c *client) loginPassword(ctx context.Context, user, password string) error {
	response := apiResLogin{}

	deviceID := generateDeviceID(user, c.apiURL.Host)
	c.logf("Using device ID: %v\n", deviceID)

	//nolint:exhaustruct_v5 // Intentional zero-value initialization for apiReqLogin
	err := c.apiPost(
		ctx,
		apiLogin,
		apiReqLogin{
			Type:       flowLoginPassword,
			Password:   password,
			Identifier: newUserIdentifier(user),
			DeviceID:   deviceID,
		},
		&response,
	)
	if err != nil {
		return fmt.Errorf("failed to log in: %w", err)
	}

	c.accessToken = response.AccessToken

	tokenHint := ""
	if len(response.AccessToken) > tokenHintLength {
		tokenHint = response.AccessToken[:tokenHintLength]
	}

	c.logf("AccessToken: %v...\n", tokenHint)
	c.logf("HomeServer: %v\n", response.HomeServer)
	c.logf("User: %v\n", response.UserID)
	c.logf("DeviceID: %v\n", response.DeviceID)

	return nil
}

// loginToken performs a token-based login to the Matrix server.
//
// Parameters:
//   - ctx: bounds the request.
//   - token: the login token.
//
// Returns:
//   - error: the request error or the API's error.
func (c *client) loginToken(ctx context.Context, token string) error {
	response := apiResLogin{}

	// For token-based login, use an empty user string in the device ID hash.
	// This ensures all token-based logins to the same host reuse the same device.
	deviceID := generateDeviceID("", c.apiURL.Host)
	c.logf("Using device ID: %v\n", deviceID)

	//nolint:exhaustruct_v5 // Intentional zero-value initialization for apiReqLogin
	err := c.apiPost(
		ctx,
		apiLogin,
		apiReqLogin{
			Type:     flowLoginToken,
			Token:    token,
			DeviceID: deviceID,
		},
		&response,
	)
	if err != nil {
		return fmt.Errorf("failed to log in with token: %w", err)
	}

	c.accessToken = response.AccessToken

	tokenHint := ""
	if len(response.AccessToken) > tokenHintLength {
		tokenHint = response.AccessToken[:tokenHintLength]
	}

	c.logf("AccessToken: %v...\n", tokenHint)
	c.logf("HomeServer: %v\n", response.HomeServer)
	c.logf("User: %v\n", response.UserID)
	c.logf("DeviceID: %v\n", response.DeviceID)

	return nil
}

// sendMessage sends a message to the specified rooms or all joined rooms if none are specified.
//
// Parameters:
//   - ctx: bounds the requests.
//   - message: the message.
//   - rooms: the room aliases or IDs, or none for all joined rooms.
//
// Returns:
//   - []error: one error for each room the message could not reach, or for a
//     failed lookup of the joined rooms.
func (c *client) sendMessage(ctx context.Context, message string, rooms []string) []error {
	if len(rooms) >= minSliceLength {
		return c.sendToExplicitRooms(ctx, rooms, message)
	}

	return c.sendToJoinedRooms(ctx, message)
}

// sendMessageToRoom sends a message to a specific room using PUT method with transaction ID.
//
// Parameters:
//   - ctx: bounds the request.
//   - message: the message.
//   - roomID: the room ID.
//
// Returns:
//   - error: the request error or the API's error.
func (c *client) sendMessageToRoom(ctx context.Context, message, roomID string) error {
	resEvent := apiResEvent{}
	txnID := generateTransactionID()

	path := fmt.Sprintf(apiSendMessage, roomID, txnID)

	request := apiReqSend{
		MsgType: msgTypeText,
		Body:    message,
	}

	return c.apiPut(ctx, path, request, &resEvent)
}

// sendToExplicitRooms sends a message to explicitly specified rooms and collects any errors.
//
// A room that starts with ! is a room ID and is sent to as is. Any other room,
// such as an alias, is joined first to get its ID.
//
// Parameters:
//   - ctx: bounds the requests.
//   - rooms: the room aliases or IDs.
//   - message: the message.
//
// Returns:
//   - []error: one error for each room that is empty or that the message
//     could not reach.
func (c *client) sendToExplicitRooms(ctx context.Context, rooms []string, message string) []error {
	var errs []error

	for _, room := range rooms {
		room = strings.TrimSpace(room)
		if room == "" {
			errs = append(errs, fmt.Errorf("%w", ErrEmptyRoom))

			continue
		}

		c.logf("Sending message to '%v'...\n", room)

		if !strings.HasPrefix(room, "!") {
			roomID, err := c.joinRoom(ctx, room)
			if err != nil {
				errs = append(errs, fmt.Errorf("error joining room %v: %w", room, err))

				continue
			}

			if room != roomID {
				c.logf("Resolved room alias '%v' to ID '%v'", room, roomID)
			}

			room = roomID
		}

		if err := c.sendMessageToRoom(ctx, message, room); err != nil {
			errs = append(
				errs,
				fmt.Errorf("failed to send message to room '%v': %w", room, err),
			)
		}
	}

	return errs
}

// sendToJoinedRooms sends a message to all joined rooms and collects any errors.
//
// Parameters:
//   - ctx: bounds the requests.
//   - message: the message.
//
// Returns:
//   - []error: one error for each room the message could not reach, or one
//     for a failed lookup of the joined rooms.
func (c *client) sendToJoinedRooms(ctx context.Context, message string) []error {
	var errs []error

	joinedRooms, err := c.getJoinedRooms(ctx)
	if err != nil {
		return append(errs, fmt.Errorf("failed to get joined rooms: %w", err))
	}

	for _, roomID := range joinedRooms {
		c.logf("Sending message to '%v'...\n", roomID)

		if err := c.sendMessageToRoom(ctx, message, roomID); err != nil {
			errs = append(
				errs,
				fmt.Errorf("failed to send message to room '%v': %w", roomID, err),
			)
		}
	}

	return errs
}

// setAuthorizationHeader sets the Authorization header with Bearer token if access token is available.
//
// Parameters:
//   - req: the request.
func (c *client) setAuthorizationHeader(req *http.Request) {
	if c.accessToken != "" {
		req.Header.Set(authorizationHeader, bearerPrefix+c.accessToken)
	}
}

// useToken sets the access token for the client.
//
// Parameters:
//   - token: the access token.
func (c *client) useToken(token string) {
	c.accessToken = token
}

// isRateLimitedError checks if the error is a rate limiting error.
//
// Parameters:
//   - err: the error, or nil.
//
// Returns:
//   - bool: true when err wraps a Matrix M_LIMIT_EXCEEDED error.
func isRateLimitedError(err error) bool {
	if err == nil {
		return false
	}

	if apiErr, ok := errors.AsType[*apiResError](err); ok {
		return apiErr.IsRateLimited()
	}

	return false
}

// generateTransactionID generates a unique transaction ID using a timestamp
// and random bytes to ensure uniqueness across requests.
//
// Returns:
//   - string: the transaction ID.
func generateTransactionID() string {
	now := time.Now().UnixNano()
	randBytes := make([]byte, transactionIDRandLen)

	if _, err := rand.Read(randBytes); err != nil {
		// Fallback: use timestamp-based bytes if crypto/rand fails
		for i := range randBytes {
			randBytes[i] = byte((now >> (bitsPerByte * i)) & byteMask)
		}
	}

	return fmt.Sprintf("%d-%s", now, hex.EncodeToString(randBytes))
}

// generateDeviceID generates a deterministic device ID based on the user and
// host. This ensures the same logical client reuses the same device across
// restarts, preventing device accumulation on the Matrix server.
//
// The device ID follows the Matrix spec's Opaque Identifier Grammar:
// characters from [0-9], [A-Z], [a-z], "-", ".", "_", and "~".
// The format is "SH" + 8 characters derived from SHA-256(user:host),
// totaling 10 characters as recommended by the Matrix spec.
//
// The host is normalized to exclude default ports (443 for HTTPS, 80 for HTTP)
// to prevent device churn when ports are omitted or explicitly included.
//
// Parameters:
//   - user: the user name, or empty for a token login.
//   - host: the homeserver host, with an optional port.
//
// Returns:
//   - string: the device ID.
func generateDeviceID(user, host string) string {
	// Normalize host by removing default ports to prevent device churn.
	// url.Host may include a port (e.g., "matrix.example.com:443"), but we want
	// consistent device IDs regardless of whether the default port is explicit.
	normalizedHost := normalizeHostForDeviceID(host)

	// Create a deterministic hash from user and host.
	// Using user + ":" + host ensures different users on the same host
	// get different device IDs, and the same user on different hosts
	// also gets different device IDs.
	hash := sha256.Sum256([]byte(user + ":" + normalizedHost))

	// Encode hash bytes to a device ID using the Opaque Identifier Grammar.
	// We use base32-like encoding with the allowed character set.
	// The hash provides sufficient entropy for uniqueness per user+host.
	deviceID := deviceIDPrefix
	for i := 0; i < deviceIDHashBytes && len(deviceID) < deviceIDLength; i++ {
		// Map each byte to a character in the allowed set.
		// Using modulo with a character set size ensures we stay within bounds.
		deviceID += string(opaqueIDCharset[hash[i]%opaqueIDCharsetLen])
	}

	return deviceID
}

// normalizeHostForDeviceID normalizes a host string by removing default ports.
// This prevents device churn when the port is omitted or explicitly included
// in the URL (e.g., "matrix.example.com:443" vs "matrix.example.com").
//
// Parameters:
//   - host: the host, with an optional port.
//
// Returns:
//   - string: the host without a default port.
func normalizeHostForDeviceID(host string) string {
	// Check for explicit port using net.SplitHostPort.
	// If no port is present, return the host as-is.
	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		// No port present, return host as-is.
		return host
	}

	// Remove default ports to normalize the host.
	// Port 443 is the default for HTTPS, 80 for HTTP.
	if port == "443" || port == "80" {
		return hostname
	}

	// Non-default port, keep it for uniqueness.
	return host
}

// newClient creates a new Matrix client with the specified host and TLS settings.
//
// Parameters:
//   - host: the homeserver host, with an optional port.
//   - disableTLS: connect over HTTP instead of HTTPS.
//   - logger: receives the client's progress messages.
//   - httpClient: holds the HTTP client the API requests go through.
//
// Returns:
//   - *client: the client, not yet logged in.
//
//nolint:exhaustruct_v5 // Intentional partial initialization
func newClient(host string, disableTLS bool, logger types.StdLogger, httpClient *httpclient.Holder) *client {
	client := &client{
		logger: logger,
		apiURL: url.URL{
			Host:   host,
			Scheme: schemeHTTPS,
		},
		httpClient: httpClient,
	}

	if client.logger == nil {
		client.logger = util.DiscardLogger
	}

	if disableTLS {
		client.apiURL.Scheme = schemeHTTP
	}

	client.logger.Printf("Using server: %v\n", client.apiURL.String())

	return client
}
