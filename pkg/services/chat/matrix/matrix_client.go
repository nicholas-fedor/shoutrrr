package matrix

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
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
	// logger is the service's logger, which receives the client's progress
	// records. A logger that the service is given later reaches the client
	// through it.
	logger *standard.Logger
	// httpClient sends the API requests.
	httpClient types.HTTPClient
	// accessToken authenticates the requests after login, or is empty before.
	accessToken string
	// renewal, when set, logs the client in again after the homeserver
	// rejects its token.
	//
	// The client clears it before it logs in, so the login's own requests
	// never start another. It is set again only when the login failed because
	// its context ended, and the next request then logs in before it is sent.
	// So a send logs in again once, or once more for each later room after a
	// room's time ran out.
	renewal loginRenewal
	// loginErr, when set, is the error of a renewal that failed for another
	// reason than its context ending.
	//
	// Every later request of the send returns it without being sent.
	loginErr error
	// baseURL is the homeserver's base URL, its scheme and host, which every
	// request path follows.
	baseURL string
	// host is the homeserver host, with an optional port.
	host string
}

// loginRenewal is the login a send's client runs again after the homeserver
// rejects its token.
//
// The service holds the login gate and the stored token, so the renewal goes
// through it.
type loginRenewal struct {
	// service runs the login, or is nil when the client cannot log in again,
	// as with an access token from the URL.
	service *Service
	// config is the send's config, which names the account.
	config *Config
	// rejected, when set, is the token the homeserver rejected, which a login
	// cut short by its context has not replaced yet.
	rejected string
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

// emptyJSONObject is the request body of a room join, which takes no fields. A
// request only reads its body, so concurrent joins share it. It is held as an
// interface value, so passing it as a request does not box the slice again.
var emptyJSONObject any = jsontext.Value(`{}`)

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

// apiDo sends a request to the Matrix API and decodes its JSON response.
//
// A rate-limited request is sent again, up to three times, after the wait the
// homeserver asks for, when that wait fits before ctx's deadline. A request
// whose access token the homeserver rejects as unknown is sent again once
// with a new token, when the client can log in again. After that login fails,
// the client's requests fail with its error without being sent.
//
// Parameters:
//   - ctx: cancellation and deadline for the request and the waits.
//   - method: the HTTP method.
//   - path: the escaped API path, which the request uses as written.
//   - request: the value sent as the JSON request body, sent byte for byte
//     when it is a [jsontext.Value], or nil to send no body.
//   - response: the value the JSON response is decoded into.
//
// Returns:
//   - error: the Matrix API error for an error status, [ErrUnexpectedStatus]
//     for an error status without one, the rate limit's error joined with
//     ctx's error when ctx ends during a wait, the error of logging in again
//     (returned by later requests too, unless ctx ended the login), or the
//     marshaling, request, read, or decoding error. Reading stops at
//     [httpbody.MaxSize]. An error status is reported from the bytes read,
//     and an oversized success body wraps [httpbody.ErrTooLarge].
func (c *client) apiDo(ctx context.Context, method, path string, request, response any) error {
	if c.loginErr != nil {
		return c.loginErr
	}

	if c.renewal.rejected != "" {
		if err := c.renew(ctx, c.renewal.rejected); err != nil {
			return err
		}
	}

	var body []byte

	switch request := request.(type) {
	case nil:
	case jsontext.Value:
		// Raw JSON goes out as it is, which skips the encoder's check and copy.
		body = request
	default:
		var err error
		if body, err = json.Marshal(request, jsonopts.Default); err != nil {
			return fmt.Errorf("marshaling %s request: %w", method, err)
		}
	}

	retryAfter, err := c.send(ctx, method, path, body, response)

	for retries := 0; ; {
		if c.renewal.service != nil && isUnknownToken(err) {
			if err := c.renew(ctx, c.accessToken); err != nil {
				return err
			}

			retryAfter, err = c.send(ctx, method, path, body, response)

			continue
		}

		if retries == maxRateLimitRetries || !retryFits(ctx, retryAfter) {
			return err
		}

		retries++

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

// joinRoom joins a specified room and returns its ID. The room travels
// escaped as one path segment, so a character such as / stays in the room.
//
// Parameters:
//   - ctx: bounds the request.
//   - room: the room alias or ID.
//
// Returns:
//   - string: the room ID.
//   - error: the request error, the API's error, or [ErrInvalidRoomID] for a
//     reply whose room ID lacks the ! sigil.
func (c *client) joinRoom(ctx context.Context, room string) (string, error) {
	resRoom := apiResRoom{}

	err := c.apiDo(
		ctx,
		http.MethodPost,
		apiRoomJoin+url.PathEscape(room),
		emptyJSONObject,
		&resRoom,
	)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(resRoom.RoomID, roomIDSigil) {
		return "", ErrInvalidRoomID
	}

	return resRoom.RoomID, nil
}

// login authenticates the client using a username and password.
//
// It asks the homeserver for its login flows and uses the password flow. The
// login endpoint takes no access token, so the client drops its token first,
// and a failed login leaves it without one.
//
// Parameters:
//   - ctx: bounds the requests.
//   - user: the user name or full Matrix ID.
//   - password: the password.
//
// Returns:
//   - error: the error of the flows request or the password login, or
//     [ErrUnsupportedLoginFlows] when the homeserver offers no password flow.
func (c *client) login(ctx context.Context, user, password string) error {
	c.accessToken = ""

	resLogin := apiResLoginFlows{}
	if err := c.apiDo(ctx, http.MethodGet, apiLogin, nil, &resLogin); err != nil {
		return fmt.Errorf("failed to get login flows: %w", err)
	}

	if slices.ContainsFunc(resLogin.Flows, func(offered flow) bool { return offered.Type == flowLoginPassword }) {
		return c.loginPassword(ctx, user, password)
	}

	flows := make([]string, 0, len(resLogin.Flows))
	for _, offered := range resLogin.Flows {
		flows = append(flows, string(offered.Type))
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
//   - error: the request or API error, or [ErrLoginReplyWithoutToken] for a
//     reply without a token.
func (c *client) loginPassword(ctx context.Context, user, password string) error {
	response := apiResLogin{}

	deviceID := generateDeviceID(user, c.host)

	err := c.apiDo(
		ctx,
		http.MethodPost,
		apiLogin,
		apiReqLogin{
			Type:       flowLoginPassword,
			Password:   password,
			Identifier: identifier{Type: idTypeUser, User: user},
			DeviceID:   deviceID,
		},
		&response,
	)
	if err != nil {
		return fmt.Errorf("failed to log in: %w", err)
	}

	// A reply without a token is not stored, so the next send logs in again
	// instead of sending without authentication.
	if response.AccessToken == "" {
		return ErrLoginReplyWithoutToken
	}

	c.accessToken = response.AccessToken

	// The access token is a credential, so the record leaves it out.
	c.logger.LogAttrs(ctx, slog.LevelDebug, "logged in",
		slog.String(logging.KeyHomeserver, response.HomeServer), slog.String(logging.KeyDevice, response.DeviceID))

	return nil
}

// renew logs the client in again in place of a token the homeserver rejected.
//
// The renewal is cleared while the login runs, so the login's requests never
// start another. A login that ctx cut short leaves the renewal pending for the
// next request, since such a failure says nothing about the homeserver. Any
// other failure is kept, and every later request returns it without being
// sent.
//
// Parameters:
//   - ctx: cancellation for waiting on and running the login.
//   - rejected: the token the homeserver rejected.
//
// Returns:
//   - error: the login failure, or nil when the client holds a new token.
func (c *client) renew(ctx context.Context, rejected string) error {
	renewal := c.renewal
	c.renewal = loginRenewal{service: nil, config: nil, rejected: ""}

	c.logger.Debug(ctx, "access token rejected, logging in again")

	err := renewal.service.logIn(ctx, c, renewal.config, rejected)
	if err == nil {
		return nil
	}

	err = fmt.Errorf("access token rejected: %w", err)

	if ctx.Err() != nil {
		renewal.rejected = rejected
		c.renewal = renewal
	} else {
		c.loginErr = err
	}

	return err
}

// send makes one request to the Matrix API and decodes its JSON response.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - method: the HTTP method.
//   - path: the escaped API path, which the request uses as written.
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

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL, reader)
	if err != nil {
		return noRetry, fmt.Errorf("creating %s request: %w", method, redact.URLError(err))
	}

	// The path is set on the parsed base URL, which spares parsing the whole
	// request URL. RawPath keeps the escapes, such as a / in a room.
	if req.URL.Path, err = url.PathUnescape(path); err != nil {
		return noRetry, fmt.Errorf("creating %s request: %w", method, err)
	}

	req.URL.RawPath = path

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
//     returns its own error. An account in no room returns [ErrNoJoinedRooms].
func (c *client) sendMessage(ctx context.Context, message string, rooms []string) error {
	if len(rooms) >= minSliceLength {
		return c.sendToExplicitRooms(ctx, rooms, message)
	}

	return c.sendToJoinedRooms(ctx, message)
}

// sendMessageToRoom sends a message to a specific room using PUT method with transaction ID.
// The room ID travels escaped as one path segment.
//
// Parameters:
//   - ctx: bounds the request.
//   - message: the message.
//   - roomID: the room ID.
//
// Returns:
//   - error: [ErrInvalidRoomID] for a room ID without the ! sigil, or the
//     request error or the API's error.
func (c *client) sendMessageToRoom(ctx context.Context, message, roomID string) error {
	rest, ok := strings.CutPrefix(roomID, roomIDSigil)
	if !ok {
		return ErrInvalidRoomID
	}

	resEvent := apiResEvent{}
	txnID := generateTransactionID()

	// A path segment holds the ! as is, so only the rest of the room ID is
	// escaped.
	path := apiRooms + roomIDSigil + url.PathEscape(rest) + apiSendMessage + txnID

	request := apiReqSend{
		MsgType: msgTypeText,
		Body:    message,
	}

	return c.apiDo(ctx, http.MethodPut, path, request, &resEvent)
}

// sendToExplicitRooms sends a message to explicitly specified rooms, and keeps
// sending after a room fails. Once ctx ends, it sends to no more rooms.
//
// Each room, an alias or a room ID, is joined first, which accepts an invite
// to it and gives its room ID. A join of a room the account has already joined
// changes nothing.
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
			if room == "" {
				return ErrEmptyRoom
			}

			// Rooms identify the recipients, so the records leave them out.
			c.logger.Debug(ctx, "sending message")

			roomID, err := c.joinRoom(ctx, room)
			if err != nil {
				return fmt.Errorf("error joining room %v: %w", room, err)
			}

			if room != roomID {
				c.logger.Debug(ctx, "room alias resolved")
			}

			if err := c.sendMessageToRoom(ctx, message, roomID); err != nil {
				return fmt.Errorf("failed to send message to room '%v': %w", roomID, err)
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
//     returns its own error. An account in no room returns [ErrNoJoinedRooms].
func (c *client) sendToJoinedRooms(ctx context.Context, message string) error {
	joinedRooms, err := c.getJoinedRooms(ctx)
	if err != nil {
		return fmt.Errorf("failed to get joined rooms: %w", err)
	}

	if len(joinedRooms) == 0 {
		return ErrNoJoinedRooms
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
//   - user: the user name or full Matrix ID.
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
//
// This prevents device churn when the port is omitted or explicitly included
// in the URL, as in matrix.example.com:443 and matrix.example.com. An IPv6
// host keeps its brackets either way, so [2001:db8::1]:443 and [2001:db8::1]
// give the same host.
//
// Parameters:
//   - host: the host, with an optional port.
//
// Returns:
//   - string: the host without a default port, or the host as given when it
//     does not split.
func normalizeHostForDeviceID(host string) string {
	hostname, port, err := util.SplitURLHost(&url.URL{Host: host})
	if err != nil {
		return host
	}

	// Port 443 is the default for HTTPS and 80 for HTTP. Both are dropped
	// whatever the scheme, so existing device IDs stay the same.
	if port == "443" || port == "80" {
		port = ""
	}

	return util.JoinURLHost(hostname, port)
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
		wait, _ := httpclient.Duration(float64(resError.RetryAfterMS), time.Millisecond)

		return wait
	}

	return noRetry
}

// isUnknownToken reports whether err is the homeserver's answer that it does
// not recognize the request's access token. The homeserver answers so for a
// token that expired or was revoked, and for one that another login to the
// same device replaced.
//
// Parameters:
//   - err: the error of a request.
//
// Returns:
//   - bool: whether err is a Matrix error with code M_UNKNOWN_TOKEN.
func isUnknownToken(err error) bool {
	resError, ok := errors.AsType[*apiResError](err)

	return ok && resError.Code == errCodeUnknownToken
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
		baseURL:    (&url.URL{Scheme: scheme, Host: host}).String(),
		host:       host,
	}
}
