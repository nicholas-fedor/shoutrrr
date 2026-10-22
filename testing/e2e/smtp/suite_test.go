package e2e_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/email/smtp"
)

// address is a mail address as the Mailpit API returns it.
type address struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

// mailpitMessage is the part of a stored message the tests check, as the
// Mailpit API returns it.
type mailpitMessage struct {
	From     address   `json:"From"`
	Subject  string    `json:"Subject"`
	Text     string    `json:"Text"`
	HTML     string    `json:"HTML"`
	Username string    `json:"Username"`
	To       []address `json:"To"`
}

// Addresses and credentials of the local Mailpit stack that docker-compose.yaml starts.
const (
	// starttlsHost serves SMTP with optional STARTTLS and requires the account below.
	starttlsHost = "localhost:1587"
	// starttlsAPI is the API of the STARTTLS instance.
	starttlsAPI = "http://localhost:8025"
	// tlsHost serves SMTP over implicit TLS and accepts any credentials.
	tlsHost = "localhost:1465"
	// tlsAPI is the API of the implicit TLS instance.
	tlsAPI = "http://localhost:8026"
	// username is the account the STARTTLS instance requires.
	username = "shoutrrr"
	// password is the password of that account.
	password = "testing"
	// fromAddress is the sender of every test message.
	fromAddress = "sender@example.com"
	// toAddress is the default recipient of every test message.
	toAddress = "recipient@example.com"
)

// Timeouts for the tests' own requests to the Mailpit API.
const (
	// apiTimeout bounds each API request.
	apiTimeout = 2 * time.Second
	// deliveryTimeout bounds the wait for a sent message to be stored.
	deliveryTimeout = 5 * time.Second
)

// errUnexpectedStatus indicates a Mailpit API response other than 200 OK.
var errUnexpectedStatus = errors.New("unexpected status")

// requireMailpit skips the test when the Mailpit API at apiURL is not reachable.
//
// Parameters:
//   - t: the test.
//   - apiURL: the base URL of a Mailpit instance's API.
func requireMailpit(t *testing.T, apiURL string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), apiTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/api/v1/info", http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("Mailpit at %s not available, skipping SMTP e2e test: %v", apiURL, err)
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Mailpit at %s returned %s, skipping SMTP e2e test", apiURL, resp.Status)
	}
}

// serviceURL builds an SMTP service URL for host with the given credentials
// and query values, sending from [fromAddress] to [toAddress] unless the query
// sets other addresses.
//
// Parameters:
//   - host: the SMTP host and port.
//   - user: the account, or "" for none.
//   - pass: the account's password.
//   - query: the query values to set.
//
// Returns:
//   - string: the service URL.
func serviceURL(host, user, pass string, query map[string]string) string {
	values := url.Values{}
	values.Set("fromaddress", fromAddress)
	values.Set("toaddresses", toAddress)
	values.Set("skiptlsverify", "yes")

	for key, value := range query {
		values.Set(key, value)
	}

	serviceURL := &url.URL{Scheme: "smtp", Host: host, Path: "/", RawQuery: values.Encode()}
	if user != "" {
		serviceURL.User = url.UserPassword(user, pass)
	}

	return serviceURL.String()
}

// newService initializes an smtp.Service from a service URL with a logger that
// writes to the test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *smtp.Service: the initialized service.
func newService(t *testing.T, rawURL string) *smtp.Service {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &smtp.Service{}
	require.NoError(t, service.Initialize(parsedURL, log.New(t.Output(), "[Test] ", 0)))

	return service
}

// uniqueSubject appends a suffix that is unique to this send, so a message
// stored by an earlier run cannot satisfy a lookup.
//
// Parameters:
//   - text: the subject text.
//
// Returns:
//   - string: the subject with a unique suffix.
func uniqueSubject(text string) string {
	return fmt.Sprintf("%s %d", text, time.Now().UnixNano())
}

// fetchMessage waits for Mailpit to store the message with the given subject
// and returns it.
//
// Parameters:
//   - t: the test.
//   - apiURL: the base URL of the Mailpit instance's API.
//   - subject: the message's unique subject.
//
// Returns:
//   - mailpitMessage: the stored message.
func fetchMessage(t *testing.T, apiURL, subject string) mailpitMessage {
	t.Helper()

	return fetchMessages(t, apiURL, subject, 1)[0]
}

// fetchMessages waits for Mailpit to store count messages with the given
// subject and returns them.
//
// Parameters:
//   - t: the test.
//   - apiURL: the base URL of the Mailpit instance's API.
//   - subject: the messages' unique subject.
//   - count: the number of messages to wait for.
//
// Returns:
//   - []mailpitMessage: the stored messages.
func fetchMessages(t *testing.T, apiURL, subject string, count int) []mailpitMessage {
	t.Helper()

	searchURL := apiURL + "/api/v1/search?" + url.Values{"query": {fmt.Sprintf("subject:%q", subject)}}.Encode()

	// The condition runs on another goroutine, so it reports failures by
	// returning false instead of through require, and records the IDs only
	// on the attempt that succeeds.
	var ids []string

	require.Eventually(t, func() bool {
		var found struct {
			Messages []struct {
				ID string `json:"ID"`
			} `json:"messages"`
		}

		if fetchJSON(t.Context(), searchURL, &found) != nil || len(found.Messages) != count {
			return false
		}

		for _, summary := range found.Messages {
			ids = append(ids, summary.ID)
		}

		return true
	}, deliveryTimeout, 100*time.Millisecond, "not %d messages with subject %q", count, subject)

	messages := make([]mailpitMessage, len(ids))
	for i, id := range ids {
		getJSON(t, apiURL+"/api/v1/message/"+url.PathEscape(id), &messages[i])
	}

	return messages
}

// getJSON requests a Mailpit API URL and decodes its JSON response into target.
//
// Parameters:
//   - t: the test.
//   - apiURL: the URL to request.
//   - target: the value to decode the response into.
func getJSON(t *testing.T, apiURL string, target any) {
	t.Helper()

	require.NoError(t, fetchJSON(t.Context(), apiURL, target))
}

// fetchJSON requests a Mailpit API URL and decodes its JSON response into
// target.
//
// Parameters:
//   - ctx: cancellation for the request, which also ends after [apiTimeout].
//   - apiURL: the URL to request.
//   - target: the value to decode the response into.
//
// Returns:
//   - error: a request, status, or decoding error.
func fetchJSON(ctx context.Context, apiURL string, target any) error {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", apiURL, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s from %s", errUnexpectedStatus, resp.Status, apiURL)
	}

	if err := json.UnmarshalRead(resp.Body, target, jsonopts.Default); err != nil {
		return fmt.Errorf("decoding %s: %w", apiURL, err)
	}

	return nil
}
