package matrix

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// HTTPClient defines the interface for HTTP operations.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// client manages interactions with the Matrix API.
type client struct {
	apiURL      url.URL
	accessToken string
	logger      types.StdLogger
	httpClient  HTTPClient
}

// DefaultHTTPClient is the default implementation using http.DefaultClient with timeout.
type DefaultHTTPClient struct {
	client *http.Client
}

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

	// defaultHTTPTimeout is the timeout for HTTP requests.
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
)

// Do performs the HTTP request.
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %w", err)
	}

	return resp, nil
}

// apiDo performs an HTTP request to the Matrix API.
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
func (c *client) apiGet(ctx context.Context, path string, response any) error {
	reqCtx, cancel := context.WithTimeout(
		ctx,
		defaultHTTPTimeout,
	)
	defer cancel()

	req, err := http.NewRequestWithContext(
		reqCtx,
		http.MethodGet,
		c.buildURL(path),
		http.NoBody,
	)
	if err != nil {
		return fmt.Errorf("creating GET request: %w", err)
	}

	c.setAuthorizationHeader(req)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing GET request: %w", err)
	}

	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading GET response body: %w", err)
	}

	if res.StatusCode >= httpClientErrorStatus {
		resError := &apiResError{}
		if err = json.Unmarshal(body, resError); err == nil {
			return resError
		}

		return fmt.Errorf(
			"%w: %v (unmarshal error: %w)",
			ErrUnexpectedStatus,
			res.Status, err,
		)
	}

	if err = json.Unmarshal(body, response); err != nil {
		return fmt.Errorf("unmarshaling GET response: %w", err)
	}

	return nil
}

// apiPost performs a POST request to the Matrix API with the provided context.
func (c *client) apiPost(ctx context.Context, path string, request, response any) error {
	return c.apiDo(ctx, http.MethodPost, path, request, response)
}

// apiPut performs a PUT request to the Matrix API with the provided context.
func (c *client) apiPut(ctx context.Context, path string, request, response any) error {
	return c.apiDo(ctx, http.MethodPut, path, request, response)
}

// buildURL returns a copy of the base API URL with the specified path set.
func (c *client) buildURL(path string) string {
	u := c.apiURL
	u.Path = path

	return u.String()
}

// doSingleRequest performs a single HTTP request without retry logic.
func (c *client) doSingleRequest(ctx context.Context, method, path string, request, response any) error {
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshaling %s request: %w", method, err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		reqCtx,
		method,
		c.buildURL(path),
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("creating %s request: %w", method, err)
	}

	req.Header.Set("Content-Type", contentType)
	c.setAuthorizationHeader(req)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing %s request: %w", method, err)
	}

	defer func() { _ = res.Body.Close() }()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading %s response body: %w", method, err)
	}

	if res.StatusCode >= httpClientErrorStatus {
		resError := &apiResError{}
		if err = json.Unmarshal(body, resError); err == nil {
			return resError
		}

		return fmt.Errorf(
			"%w: %v (unmarshal error: %w)",
			ErrUnexpectedStatus,
			res.Status,
			err,
		)
	}

	if err = json.Unmarshal(body, response); err != nil {
		return fmt.Errorf("unmarshaling %s response: %w", method, err)
	}

	return nil
}

// getJoinedRooms retrieves the list of rooms the client has joined.
func (c *client) getJoinedRooms(ctx context.Context) ([]string, error) {
	response := apiResJoinedRooms{}

	err := c.apiGet(ctx, apiJoinedRooms, &response)
	if err != nil {
		return []string{}, err
	}

	return response.Rooms, nil
}

// joinRoom joins a specified room and returns its ID.
func (c *client) joinRoom(ctx context.Context, room string) (string, error) {
	resRoom := apiResRoom{}

	err := c.apiPost(
		ctx,
		fmt.Sprintf(apiRoomJoin, room),
		nil,
		&resRoom,
	)
	if err != nil {
		return "", err
	}

	return resRoom.RoomID, nil
}

// logf logs a formatted message using the client's logger.
func (c *client) logf(format string, v ...any) {
	c.logger.Printf(format, v...)
}

// login authenticates the client using a username and password.
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
func (c *client) loginPassword(ctx context.Context, user, password string) error {
	response := apiResLogin{}

	//nolint:exhaustruct // Intentional zero-value initialization for apiReqLogin
	err := c.apiPost(
		ctx,
		apiLogin,
		apiReqLogin{
			Type:       flowLoginPassword,
			Password:   password,
			Identifier: newUserIdentifier(user),
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

	return nil
}

// loginToken performs a token-based login to the Matrix server.
func (c *client) loginToken(ctx context.Context, token string) error {
	response := apiResLogin{}
	//nolint:exhaustruct // Intentional zero-value initialization for apiReqLogin
	err := c.apiPost(
		ctx,
		apiLogin,
		apiReqLogin{
			Type:  flowLoginToken,
			Token: token,
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

	return nil
}

// sendMessage sends a message to the specified rooms or all joined rooms if none are specified.
func (c *client) sendMessage(ctx context.Context, message string, rooms []string) []error {
	if len(rooms) >= minSliceLength {
		return c.sendToExplicitRooms(ctx, rooms, message)
	}

	return c.sendToJoinedRooms(ctx, message)
}

// sendMessageToRoom sends a message to a specific room using PUT method with transaction ID.
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
func (c *client) sendToExplicitRooms(ctx context.Context, rooms []string, message string) []error {
	var errs []error

	for _, room := range rooms {
		c.logf("Sending message to '%v'...\n", room)

		roomID, err := c.joinRoom(ctx, room)
		if err != nil {
			errs = append(errs, fmt.Errorf("error joining room %v: %w", roomID, err))

			continue
		}

		if room != roomID {
			c.logf("Resolved room alias '%v' to ID '%v'", room, roomID)
		}

		if err := c.sendMessageToRoom(ctx, message, roomID); err != nil {
			errs = append(
				errs,
				fmt.Errorf("failed to send message to room '%v': %w", roomID, err),
			)
		}
	}

	return errs
}

// sendToJoinedRooms sends a message to all joined rooms and collects any errors.
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
func (c *client) setAuthorizationHeader(req *http.Request) {
	if c.accessToken != "" {
		req.Header.Set(authorizationHeader, bearerPrefix+c.accessToken)
	}
}

// updateAccessToken updates the API URL query with the current access token.
func (c *client) updateAccessToken() {
	query := c.apiURL.Query()
	query.Set(accessTokenKey, c.accessToken)
	c.apiURL.RawQuery = query.Encode()
}

// useToken sets the access token for the client.
func (c *client) useToken(token string) {
	c.accessToken = token
	c.updateAccessToken()
}

// isRateLimitedError checks if the error is a rate limiting error.
func isRateLimitedError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *apiResError
	if errors.As(err, &apiErr) {
		return apiErr.IsRateLimited()
	}

	return false
}

// generateTransactionID generates a unique transaction ID using a timestamp
// and random bytes to ensure uniqueness across requests.
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

// newClient creates a new Matrix client with the specified host and TLS settings.
//
//nolint:exhaustruct // Intentional partial initialization
func newClient(host string, disableTLS bool, logger types.StdLogger) *client {
	client := &client{
		logger: logger,
		apiURL: url.URL{
			Host:   host,
			Scheme: schemeHTTPS,
		},
		httpClient: &DefaultHTTPClient{
			client: &http.Client{
				Timeout: defaultHTTPTimeout,
			},
		},
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
