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
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/internal/sequential"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
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
	// logger is the service's logger, which receives the client's progress
	// records. A logger that the service is given later reaches the client
	// through it.
	logger *standard.Logger
	// httpClient sends the API requests.
	httpClient types.HTTPClient
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

	// minSliceLength is the minimum length for slice operations.
	minSliceLength = 1

	// defaultHTTPTimeout is the part of the send budget for each request, and
	// the timeout of [DefaultHTTPClient].
	defaultHTTPTimeout = 10 * time.Second

	// maxRateLimitRetries is how many times a rate-limited request is sent
	// again. It bounds the retries of a homeserver that keeps asking for
	// short waits.
	maxRateLimitRetries = 3
	// noRetry is the negative wait that marks a response that may not be
	// retried.
	noRetry time.Duration = -1

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

// apiDo sends a request to the Matrix API and decodes its JSON response. A
// rate-limited request is sent again, up to three times, after the wait the
// homeserver asks for, when that wait fits before ctx's deadline.
//
// Parameters:
//   - ctx: cancellation and deadline for the request and the waits.
//   - method: the HTTP method.
//   - path: the API path.
//   - request: the value sent as the JSON request body, or nil to send no
//     body.
//   - response: the value the JSON response is decoded into.
//
// Returns:
//   - error: the Matrix API error for an error status, [ErrUnexpectedStatus]
//     for an error status without one, the rate limit's error joined with
//     ctx's error when ctx ends during a wait, or the marshaling, request,
//     read, or decoding error. Reading
//     stops at [httpbody.MaxSize]. An error status is reported from the bytes
//     read, and an oversized success body wraps [httpbody.ErrTooLarge].
func (c *client) apiDo(ctx context.Context, method, path string, request, response any) error {
	var body []byte

	if request != nil {
		var err error
		if body, err = json.Marshal(request, jsonopts.Default); err != nil {
			return fmt.Errorf("marshaling %s request: %w", method, err)
		}
	}

	retryAfter, err := c.send(ctx, method, path, body, response)

	for retries := 0; retries < maxRateLimitRetries && retryFits(ctx, retryAfter); retries++ {
		c.logger.LogAttrs(ctx, slog.LevelDebug, "rate limited, waiting to retry", slog.Duration(logging.KeyWait, retryAfter))

		timer := time.NewTimer(retryAfter)

		select {
		case <-ctx.Done():
			timer.Stop()

			return fmt.Errorf("waiting to retry %s request after %w: %w", method, err, ctx.Err())
		case <-timer.C:
		}

		retryAfter, err = c.send(ctx, method, path, body, response)
	}

	return err
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

	err := c.apiDo(ctx, http.MethodGet, apiJoinedRooms, nil, &response)
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

	err := c.apiDo(
		ctx,
		http.MethodPost,
		fmt.Sprintf(apiRoomJoin, room),
		&struct{}{},
		&resRoom,
	)
	if err != nil {
		return "", err
	}

	return resRoom.RoomID, nil
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
	if err := c.apiDo(ctx, http.MethodGet, apiLogin, nil, &resLogin); err != nil {
		return fmt.Errorf("failed to get login flows: %w", err)
	}

	flows := make([]string, 0, len(resLogin.Flows))
	for _, flow := range resLogin.Flows {
		flows = append(flows, string(flow.Type))

		// Prefer password login when a user is configured
		if flow.Type == flowLoginPassword && user != "" {
			c.logger.LogAttrs(ctx, slog.LevelDebug, "using login flow", slog.String(logging.KeyFlow, string(flow.Type)))

			return c.loginPassword(ctx, user, password)
		}

		// Only use token login when no user is configured
		if flow.Type == flowLoginToken && user == "" {
			c.logger.LogAttrs(ctx, slog.LevelDebug, "using login flow", slog.String(logging.KeyFlow, string(flow.Type)))

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

	//nolint:exhaustruct_v5 // Intentional zero-value initialization for apiReqLogin
	err := c.apiDo(
		ctx,
		http.MethodPost,
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

	// The access token is a credential, so the record leaves it out.
	c.logger.LogAttrs(ctx, slog.LevelDebug, "logged in",
		slog.String(logging.KeyHomeserver, response.HomeServer), slog.String(logging.KeyDevice, response.DeviceID))

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

	//nolint:exhaustruct_v5 // Intentional zero-value initialization for apiReqLogin
	err := c.apiDo(
		ctx,
		http.MethodPost,
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

	// The access token is a credential, so the record leaves it out.
	c.logger.LogAttrs(ctx, slog.LevelDebug, "logged in",
		slog.String(logging.KeyHomeserver, response.HomeServer), slog.String(logging.KeyDevice, response.DeviceID))

	return nil
}

// send makes one request to the Matrix API and decodes its JSON response.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - method: the HTTP method.
//   - path: the API path.
//   - body: the JSON request body, or nil to send none.
//   - response: the value the JSON response is decoded into.
//
// Returns:
//   - time.Duration: for a rate-limited request, the wait the homeserver
//     asks for, from the Retry-After header or else retry_after_ms. It is
//     negative for any other request, and for a 429 that gives no wait.
//   - error: the Matrix API error for an error status, [ErrUnexpectedStatus]
//     for an error status without one, or the request, read, or decoding
//     error. Reading stops at [httpbody.MaxSize].
func (c *client) send(ctx context.Context, method, path string, body []byte, response any) (time.Duration, error) {
	var reader io.Reader = http.NoBody
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.buildURL(path), reader)
	if err != nil {
		return noRetry, fmt.Errorf("creating %s request: %w", method, redact.URLError(err))
	}

	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}

	c.setAuthorizationHeader(req)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return noRetry, fmt.Errorf("executing %s request: %w", method, redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	resBody, readErr := httpbody.Read(res.Body, httpbody.MaxSize)

	// An error status is reported from the first bytes of an oversized body.
	if res.StatusCode >= http.StatusBadRequest && (readErr == nil || errors.Is(readErr, httpbody.ErrTooLarge)) {
		resError := &apiResError{}
		err = json.Unmarshal(resBody, resError, jsonopts.Default)

		// A proxy in front of the homeserver can answer a 429 with its own
		// body, so the Retry-After header counts even when the body is not a
		// Matrix error.
		wait := noRetry
		if res.StatusCode == http.StatusTooManyRequests {
			wait = retryWait(res.Header, resError)
		}

		if err != nil {
			return wait, fmt.Errorf(
				"%w: %v (unmarshal error: %w)",
				ErrUnexpectedStatus,
				res.Status,
				err,
			)
		}

		return wait, resError
	}

	if readErr != nil {
		return noRetry, fmt.Errorf("reading %s response body: %w", method, readErr)
	}

	if err = json.Unmarshal(resBody, response, jsonopts.Default); err != nil {
		return noRetry, fmt.Errorf("unmarshaling %s response: %w", method, err)
	}

	return noRetry, nil
}

// sendMessage sends a message to the specified rooms or all joined rooms if none are specified.
//
// Parameters:
//   - ctx: bounds the requests.
//   - message: the message.
//   - rooms: the room aliases or IDs, or none for all joined rooms.
//
// Returns:
//   - error: the errors of the rooms the message could not reach and ctx's
//     error when it ended the send, joined. A failed lookup of the joined rooms
//     returns its own error.
func (c *client) sendMessage(ctx context.Context, message string, rooms []string) error {
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

	return c.apiDo(ctx, http.MethodPut, path, request, &resEvent)
}

// sendToExplicitRooms sends a message to explicitly specified rooms, and keeps
// sending after a room fails. Once ctx ends, it sends to no more rooms.
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
//   - error: the errors of the rooms that are empty or that the message could
//     not reach, and ctx's error when it ended the send, joined.
func (c *client) sendToExplicitRooms(ctx context.Context, rooms []string, message string) error {
	return sequential.Send(
		ctx,
		rooms,
		sequential.PerTarget(requestsPerRoom*defaultHTTPTimeout),
		func(ctx context.Context, room string) error {
			room = strings.TrimSpace(room)
			if room == "" {
				return ErrEmptyRoom
			}

			// Rooms identify the recipients, so the records leave them out.
			c.logger.Debug(ctx, "sending message")

			if !strings.HasPrefix(room, "!") {
				roomID, err := c.joinRoom(ctx, room)
				if err != nil {
					return fmt.Errorf("error joining room %v: %w", room, err)
				}

				if room != roomID {
					c.logger.Debug(ctx, "room alias resolved")
				}

				room = roomID
			}

			if err := c.sendMessageToRoom(ctx, message, room); err != nil {
				return fmt.Errorf("failed to send message to room '%v': %w", room, err)
			}

			return nil
		},
	)
}

// sendToJoinedRooms sends a message to all joined rooms, and keeps sending
// after a room fails. Once ctx ends, it sends to no more rooms.
//
// Parameters:
//   - ctx: bounds the requests.
//   - message: the message.
//
// Returns:
//   - error: the errors of the rooms the message could not reach and ctx's
//     error when it ended the send, joined. A failed lookup of the joined rooms
//     returns its own error.
func (c *client) sendToJoinedRooms(ctx context.Context, message string) error {
	joinedRooms, err := c.getJoinedRooms(ctx)
	if err != nil {
		return fmt.Errorf("failed to get joined rooms: %w", err)
	}

	return sequential.Send(ctx, joinedRooms, sequential.PerTarget(defaultHTTPTimeout),
		func(ctx context.Context, roomID string) error {
			// Rooms identify the recipients, so the record leaves them out.
			c.logger.Debug(ctx, "sending message")

			if err := c.sendMessageToRoom(ctx, message, roomID); err != nil {
				return fmt.Errorf("failed to send message to room '%v': %w", roomID, err)
			}

			return nil
		})
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

// retryFits reports whether a retry may follow a wait. It may when the
// homeserver asked for a wait and the wait ends before ctx's deadline. A room
// request's deadline is the room's share of the send's budget, so its wait
// cannot take time from later rooms. A wait spends the time of every request
// that shares ctx, such as the message after a room join, or the rooms after
// a login.
//
// Parameters:
//   - ctx: the context of the request, whose deadline bounds the wait.
//   - wait: the wait the homeserver asks for, or a negative wait for none.
//
// Returns:
//   - bool: true when the retry may follow the wait.
func retryFits(ctx context.Context, wait time.Duration) bool {
	if wait < 0 {
		return false
	}

	deadline, hasDeadline := ctx.Deadline()

	return !hasDeadline || wait < time.Until(deadline)
}

// retryWait returns the wait that a rate-limited response asks for. The
// Retry-After header comes first, in seconds or as an HTTP date, as the spec
// prefers it. The deprecated retry_after_ms of the error body is the
// fallback, and a value too long for a [time.Duration] becomes the longest
// wait.
//
// Parameters:
//   - header: the response headers.
//   - resError: the decoded error body.
//
// Returns:
//   - time.Duration: the wait, zero or more, or a negative wait when neither
//     gives one.
func retryWait(header http.Header, resError *apiResError) time.Duration {
	if wait, ok := httpclient.RetryAfter(header); ok {
		return wait
	}

	if resError.RetryAfterMS > 0 {
		return time.Duration(min(resError.RetryAfterMS, int64(math.MaxInt64/time.Millisecond))) * time.Millisecond
	}

	return noRetry
}

// newClient creates a Matrix API client for one send.
//
// Parameters:
//   - host: the homeserver host, with an optional port.
//   - scheme: the URL scheme of the requests, http or https.
//   - httpClient: the HTTP client that sends the requests.
//   - logger: the service's logger, which receives the client's progress
//     records. Nil discards them.
//
// Returns:
//   - *client: the client, without an access token.
//
//nolint:exhaustruct_v5 // The access token is set after the client is created.
func newClient(host, scheme string, httpClient types.HTTPClient, logger *standard.Logger) *client {
	if logger == nil {
		logger = &standard.Logger{}
	}

	return &client{
		logger:     logger,
		httpClient: httpClient,
		apiURL:     url.URL{Scheme: scheme, Host: host},
	}
}
