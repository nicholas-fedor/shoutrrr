package notifiarr

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
	// accept is the request's Accept header.
	accept string
	// body is the request body.
	body string
}

// Values shared by the service tests.
const (
	// testServiceURL is a service URL with only the API key.
	testServiceURL = "notifiarr://" + testAPIKey
	// testEndpoint is the passthrough URL testServiceURL posts to.
	testEndpoint = APIBaseURL + "/" + testAPIKey
)

// errTransport is the transport failure returned by the failing HTTP client.
var errTransport = errors.New("transport failure")

// newRespondingClient returns a mock HTTP client that expects one request,
// records it, and answers with the given status.
//
// Parameters:
//   - t: the test that owns the mock.
//   - status: the response status line, such as "200 OK".
//   - statusCode: the response status code.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *capturedRequest: the request, filled in once the client receives it.
func newRespondingClient(t *testing.T, status string, statusCode int) (*mocks.MockHTTPClient, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		*captured = capturedRequest{
			method:      req.Method,
			url:         req.URL.String(),
			contentType: req.Header.Get("Content-Type"),
			accept:      req.Header.Get("Accept"),
			body:        string(body),
		}

		return &http.Response{Status: status, StatusCode: statusCode, Body: io.NopCloser(strings.NewReader(`{"result":"success"}`))}, nil
	}).Once()

	return client, captured
}

// newTestService returns a service initialized from rawURL that logs to the
// test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, rawURL string) *Service {
	t.Helper()

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(t.Output(), "", 0)))

	return service
}

// TestGetID verifies that the service identifies itself by its scheme.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "notifiarr", (&Service{}).GetID())
}

// TestServiceTimeout verifies that the send budget is [requestTimeout].
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, requestTimeout, (&Service{}).ServiceTimeout(nil))
}

// TestInitialize verifies that Initialize applies the default name and parses
// the service URL.
func TestInitialize(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL+"?channel=123456789")

	assert.Equal(t, &Config{APIKey: testAPIKey, Name: "Shoutrrr", Channel: "123456789"}, service.Config)
}

// TestGetServiceURLFromCustom verifies that a notifiarr+ custom URL and a
// plain service URL both become service URLs with the API key.
func TestGetServiceURLFromCustom(t *testing.T) {
	t.Parallel()

	// tests lists custom URLs with the service URL each should produce.
	tests := []struct {
		name      string
		customURL string
		want      string
	}{
		{
			name:      "passthrough webhook",
			customURL: "notifiarr+https://notifiarr.com/api/v1/notification/passthrough/" + testAPIKey + "?channel=123456789",
			want:      "notifiarr://" + testAPIKey + "?channel=123456789",
		},
		{name: "service URL", customURL: testServiceURL, want: testServiceURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL, err := (&Service{}).GetServiceURLFromCustom(parseTestURL(t, tt.customURL))
			require.NoError(t, err)

			assert.Equal(t, tt.want, serviceURL.String())
		})
	}
}

// TestSend verifies the request a send makes: a JSON POST to the passthrough
// endpoint for the API key, with the notification metadata and the Discord
// payload built from the config, params, and message mentions.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs, messages, and params with the JSON body the
	// send should produce.
	tests := []struct {
		params   *types.Params
		name     string
		rawURL   string
		message  string
		wantBody string
	}{
		{
			name:     "message only",
			rawURL:   testServiceURL,
			message:  "Hello",
			wantBody: `{"notification":{"name":"Shoutrrr"},"discord":{"text":{"description":"Hello"}}}`,
		},
		{
			name: "configured Discord props",
			rawURL: testServiceURL + "?channel=735481457153277994&color=FF0000" +
				"&thumbnail=https://example.com/thumb.png&image=https://example.com/img.png&name=Backup",
			message: "Hello",
			wantBody: `{"notification":{"name":"Backup"},"discord":{"color":"FF0000",` +
				`"images":{"thumbnail":"https://example.com/thumb.png","image":"https://example.com/img.png"},` +
				`"text":{"description":"Hello"},"ids":{"channel":735481457153277994}}}`,
		},
		{
			name:    "params",
			rawURL:  testServiceURL + "?channel=1",
			message: "Body of the notification here",
			params: &types.Params{
				"id": "12345", "update": "true", "name": "1:1 App", "channel": "2", "color": "FFFFFF",
				"title": "Notification Title", "icon": "https://example.com/icon.png", "content": "Content here",
				"footer": "Footer Text", "fields": `[{"title":"Field Title","text":"Inline true","inline":true}]`,
				"thumbnail": "https://example.com/thumb.png", "image": "https://example.com/img.png",
			},
			wantBody: `{"notification":{"update":true,"name":"1:1 App","event":"12345"},"discord":{"color":"FFFFFF",` +
				`"images":{"thumbnail":"https://example.com/thumb.png","image":"https://example.com/img.png"},` +
				`"text":{"title":"Notification Title","icon":"https://example.com/icon.png","content":"Content here",` +
				`"description":"Body of the notification here","fields":[{"title":"Field Title","text":"Inline true","inline":true}],` +
				`"footer":"Footer Text"},"ids":{"channel":2}}}`,
		},
		{
			name:     "update false",
			rawURL:   testServiceURL,
			message:  "Hello",
			params:   &types.Params{"update": "false"},
			wantBody: `{"notification":{"update":false,"name":"Shoutrrr"},"discord":{"text":{"description":"Hello"}}}`,
		},
		{
			name:     "mentions ping the first user and role",
			rawURL:   testServiceURL,
			message:  "Hi <@!123> <@456> <@&789> <@&999>",
			wantBody: `{"notification":{"name":"Shoutrrr"},"discord":{"ping":{"pingUser":123,"pingRole":789},"text":{"description":"Hi <@!123> <@456> <@&789> <@&999>"}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, "200 OK", http.StatusOK)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send(tt.message, tt.params))

			assert.Equal(t, http.MethodPost, captured.method)
			assert.Equal(t, testEndpoint, captured.url)
			assert.Equal(t, "application/json", captured.contentType)
			assert.Equal(t, "application/json", captured.accept)
			assert.JSONEq(t, tt.wantBody, captured.body)
		})
	}
}

// TestSendLeavesParamsAndConfigUnchanged verifies that a send keeps param
// overrides out of the stored config and does not change the caller's
// params.
func TestSendLeavesParamsAndConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL+"?channel=1")
	want := *service.Config
	client, _ := newRespondingClient(t, "200 OK", http.StatusOK)
	service.SetHTTPClient(client)

	params := types.Params{"channel": "2", "name": "Other", "title": "Title"}
	require.NoError(t, service.Send("Hello", &params))

	assert.Equal(t, types.Params{"channel": "2", "name": "Other", "title": "Title"}, params)
	assert.Equal(t, want, *service.Config)
}

// TestSendParams verifies which send params a send accepts. The payload
// params, the reserved title, message, and level params, and the config keys
// in any case are accepted. A misspelled param fails the send before any
// request.
func TestSendParams(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL+"?channel=1")

	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))
	require.ErrorIs(t, service.Send("Hello", &types.Params{"contnet": "x"}), format.ErrInvalidConfigKey)

	client, captured := newRespondingClient(t, "200 OK", http.StatusOK)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Hello", &types.Params{
		"icon": "https://example.com/icon.png", "content": "Above", "footer": "Below",
		"fields": `[{"title":"Disk","text":"95%","inline":true}]`, "update": "true", "id": "event-1",
		types.TitleKey: "Title", types.MessageKey: "ignored", types.LevelKey: "Error", "Color": "ff0000",
	}))

	assert.Contains(t, captured.body, `"event":"event-1"`)
	assert.Contains(t, captured.body, `"update":true`)
	assert.Contains(t, captured.body, `"title":"Title"`)
	assert.Contains(t, captured.body, `"color":"ff0000"`)
	assert.Contains(t, captured.body, `"footer":"Below"`)
}

// TestSendAPIKeyParam verifies that an apikey param applies to that send only,
// as the other config keys do. The request goes to its API key.
func TestSendAPIKeyParam(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)
	client, captured := newRespondingClient(t, "200 OK", http.StatusOK)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Hello", &types.Params{"apikey": "otherkey456"}))

	assert.Equal(t, APIBaseURL+"/otherkey456", captured.url)
	assert.Equal(t, testAPIKey, service.Config.APIKey)
}

// TestSendLogsStatusWithoutBody verifies that a send logs the response status
// but not the response body.
func TestSendLogsStatusWithoutBody(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(&logs, "", 0)))

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		return &http.Response{Status: "200 OK", StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(req.URL.String() + " BODYmarker"))}, nil
	}).Once()
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Hello", nil))

	assert.Contains(t, logs.String(), "200 OK")
	assert.NotContains(t, logs.String(), testAPIKey)
	assert.NotContains(t, logs.String(), "BODYmarker")
}

// TestSendContextErrors verifies that a send rejects an empty message or an
// invalid payload before any request, reports an authentication failure and
// other error statuses, wraps a transport error without the API key, and
// stops when the caller's context is canceled.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	// rejectedSends lists sends that fail before any request.
	rejectedSends := []struct {
		params  *types.Params
		wantErr error
		name    string
		rawURL  string
		message string
	}{
		{name: "empty message", rawURL: testServiceURL, message: "", wantErr: ErrEmptyMessage},
		{name: "channel not a number", rawURL: testServiceURL + "?channel=general", message: "Hello", wantErr: ErrInvalidChannelID},
		{name: "fields not JSON", rawURL: testServiceURL, message: "Hello", params: &types.Params{"fields": "title=text"}},
	}

	for _, tt := range rejectedSends {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))

			err := service.SendContext(t.Context(), tt.message, tt.params)

			require.Error(t, err)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}

	// statusTests lists error responses with the error each should produce.
	statusTests := []struct {
		wantErr    error
		name       string
		status     string
		statusCode int
	}{
		{name: "unauthorized", status: "401 Unauthorized", statusCode: http.StatusUnauthorized, wantErr: ErrAuthenticationFailed},
		{name: "server error", status: "500 Internal Server Error", statusCode: http.StatusInternalServerError, wantErr: ErrUnexpectedStatus},
		{name: "redirect", status: "302 Found", statusCode: http.StatusFound, wantErr: ErrUnexpectedStatus},
	}

	for _, tt := range statusTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, testServiceURL)
			client, _ := newRespondingClient(t, tt.status, tt.statusCode)
			service.SetHTTPClient(client)

			err := service.SendContext(t.Context(), "Hello", nil)

			require.ErrorIs(t, err, ErrSendFailed)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errTransport}
		}).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Hello", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		require.ErrorIs(t, err, errTransport)
		assert.NotContains(t, err.Error(), testAPIKey)
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		service := newTestService(t, testServiceURL)
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		}).Once()
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "Hello", nil), context.Canceled)
	})
}

// TestSendEscapesAPIKey verifies that the API key is sent as one escaped path
// segment. A slash, question mark, hash, or percent sign in it then cannot
// change the endpoint or start a query or fragment.
func TestSendEscapesAPIKey(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "notifiarr://placeholder?apikey=a%2F..%2Fx%3Fq%3D1%23f%25zz")
	client, captured := newRespondingClient(t, "200 OK", http.StatusOK)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Hello", nil))
	assert.Equal(t, APIBaseURL+"/a%2F..%2Fx%3Fq=1%23f%25zz", captured.url)
}

// TestSendChecksConfig verifies that a send checks a config built in code as
// reading a URL does, so a config without an API key fails before any request.
// A config without an app name sends Shoutrrr, the name a URL without one
// gets.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	t.Run("no API key", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))
		*service.Config = Config{}

		require.ErrorIs(t, service.Send("Hello", nil), ErrAPIKeyMissing)
	})

	t.Run("port alone as the API key", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))
		service.Config.APIKey = ":8080"

		require.ErrorIs(t, service.Send("Hello", nil), ErrAPIKeyMissing)
	})

	t.Run("no app name", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		client, captured := newRespondingClient(t, "200 OK", http.StatusOK)
		service.SetHTTPClient(client)
		service.Config.Name = ""

		require.NoError(t, service.Send("Hello", nil))
		assert.Contains(t, captured.body, `"name":"Shoutrrr"`)
	})
}

// TestSendDefaultClient verifies that a send without a custom client, or with
// a nil *http.Client, uses a default client that honors the caller's context.
// The context is canceled before the send, so the transport returns before it
// dials Notifiarr.
func TestSendDefaultClient(t *testing.T) {
	t.Parallel()

	// tests lists how each case sets up the service's HTTP client.
	tests := []struct {
		configure func(*Service)
		name      string
	}{
		{name: "no client set", configure: func(*Service) {}},
		{name: "nil *http.Client", configure: func(s *Service) { s.SetHTTPClient((*http.Client)(nil)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, testServiceURL)
			tt.configure(service)

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := service.SendContext(ctx, "Hello", nil)

			require.ErrorIs(t, err, context.Canceled)
			assert.NotContains(t, err.Error(), testAPIKey)
		})
	}
}

// TestSetHTTPClient verifies that SetHTTPClient makes sends use a custom
// client, and that a nil client or a nil *http.Client restores the default.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("custom client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := newTestService(t, testServiceURL)
		service.SetHTTPClient(client)

		assert.Same(t, client, service.client.Client())
	})

	// nilClients lists the nil clients that restore the default client.
	nilClients := []struct {
		name   string
		client types.HTTPClient
	}{
		{name: "nil", client: nil},
		{name: "nil *http.Client", client: (*http.Client)(nil)},
	}

	for _, tt := range nilClients {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, testServiceURL)
			defaultClient := service.client.Client()

			service.SetHTTPClient(mocks.NewMockHTTPClient(t))
			service.SetHTTPClient(tt.client)

			assert.Same(t, defaultClient, service.client.Client())
		})
	}
}

// TestDefaultHTTPClient verifies that the default client has no request
// timeout, since the send budget bounds each send, and sends through
// http.DefaultTransport, so it honors the proxy environment variables.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)

	httpClient, ok := service.client.Client().(*http.Client)
	require.True(t, ok)
	assert.Zero(t, httpClient.Timeout)
	assert.Nil(t, httpClient.Transport)
}

// TestParseChannelID verifies that a numeric channel parses, including a
// 64-bit Discord snowflake, which a 32-bit platform rejects as out of range,
// and that other values, such as digits with text around them or a sign, are
// rejected as invalid syntax.
func TestParseChannelID(t *testing.T) {
	t.Parallel()

	service := &Service{}

	for input, want := range map[string]int64{"123456789": 123456789, "735481457153277994": 735481457153277994} {
		channelID, err := service.ParseChannelID(input)

		if want > math.MaxInt {
			require.ErrorIs(t, err, ErrInvalidChannelID, input)
			require.ErrorIs(t, err, strconv.ErrRange, input)

			continue
		}

		require.NoError(t, err, input)
		assert.Equal(t, want, int64(channelID), input)
	}

	// Only digits form a channel ID, so text around them or a sign is rejected.
	for _, input := range []string{"", "general", "123abc", " 123", "123 456", "12.5", "-123", "+123"} {
		_, err := service.ParseChannelID(input)

		require.ErrorIs(t, err, ErrInvalidChannelID, input)
		require.ErrorIs(t, err, strconv.ErrSyntax, input)
	}

	// Digits past the int range are rejected as out of range on every platform.
	_, err := service.ParseChannelID("99999999999999999999")
	require.ErrorIs(t, err, ErrInvalidChannelID)
	require.ErrorIs(t, err, strconv.ErrRange)
}

// TestParseFields verifies that a JSON array becomes fields, with member names
// in any case, and that other values are rejected.
func TestParseFields(t *testing.T) {
	t.Parallel()

	service := &Service{}

	fields, err := service.ParseFields(`[{"title":"Disk","text":"95%","inline":true},{"title":"Host","text":"db1","inline":false}]`)
	require.NoError(t, err)
	assert.Equal(t, []Field{{Title: "Disk", Text: "95%", Inline: true}, {Title: "Host", Text: "db1"}}, fields)

	fields, err = service.ParseFields(`[{"Title":"Disk","TEXT":"95%","Inline":true}]`)
	require.NoError(t, err)
	assert.Equal(t, []Field{{Title: "Disk", Text: "95%", Inline: true}}, fields)

	fields, err = service.ParseFields(`[]`)
	require.NoError(t, err)
	assert.Empty(t, fields)

	_, err = service.ParseFields(`{"title":"Disk"}`)
	require.Error(t, err)
}

// TestParseMentions verifies that user, nickname, and role mentions are found
// in a message, with nickname mentions normalized, and that malformed ones
// are ignored.
func TestParseMentions(t *testing.T) {
	t.Parallel()

	// tests lists messages with the mentions each should contain.
	tests := []struct {
		name    string
		message string
		want    []string
	}{
		{name: "user", message: "Hello <@123>", want: []string{"<@123>"}},
		{name: "nickname", message: "<@!456> at the start", want: []string{"<@456>"}},
		{name: "roles", message: "<@&789> and <@&790>", want: []string{"<@&789>", "<@&790>"}},
		{name: "mixed", message: "<@1> <@&2> <@!3>", want: []string{"<@1>", "<@&2>", "<@3>"}},
		{name: "malformed", message: "<@abc> <@123 <@&> @123", want: nil},
		{name: "none", message: "no mentions here", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, (&Service{}).ParseMentions(tt.message))
		})
	}
}

// TestParseMention verifies the mention type and ID for user, nickname, and
// role mentions, and that other strings are not mentions.
func TestParseMention(t *testing.T) {
	t.Parallel()

	// tests lists mention strings with the type and ID each should produce.
	tests := []struct {
		mention  string
		wantType int
		wantID   int
	}{
		{mention: "<@123>", wantType: MentionTypeUser, wantID: 123},
		{mention: "<@!456>", wantType: MentionTypeUser, wantID: 456},
		{mention: "<@&789>", wantType: MentionTypeRole, wantID: 789},
		{mention: "@123", wantType: MentionTypeNone},
		{mention: "<@123", wantType: MentionTypeNone},
		{mention: "<@abc>", wantType: MentionTypeNone},
		{mention: "<@&abc>", wantType: MentionTypeNone},
	}

	for _, tt := range tests {
		t.Run(tt.mention, func(t *testing.T) {
			t.Parallel()

			mentionType, id := ParseMention(tt.mention)

			assert.Equal(t, tt.wantType, mentionType)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

// TestExtractPingIDs verifies that user and role mentions are split into
// their IDs, skipping invalid ones.
func TestExtractPingIDs(t *testing.T) {
	t.Parallel()

	users, roles := (&Service{}).ExtractPingIDs([]string{"<@1>", "<@&2>", "<@3>", "invalid"})
	assert.Equal(t, []int{1, 3}, users)
	assert.Equal(t, []int{2}, roles)

	users, roles = (&Service{}).ExtractPingIDs(nil)
	assert.Empty(t, users)
	assert.Empty(t, roles)
}

// TestParseUpdateFlag verifies that only "true" and "false" set the update
// flag.
func TestParseUpdateFlag(t *testing.T) {
	t.Parallel()

	trueFlag := ParseUpdateFlag(types.Params{"update": "true"})
	require.NotNil(t, trueFlag)
	assert.True(t, *trueFlag)

	falseFlag := ParseUpdateFlag(types.Params{"update": "false"})
	require.NotNil(t, falseFlag)
	assert.False(t, *falseFlag)

	for _, params := range []types.Params{{"update": "yes"}, {"update": ""}, {}} {
		assert.Nil(t, ParseUpdateFlag(params), params)
	}
}

// TestBuildDiscordPayloadWithoutFields verifies that no Discord payload is
// built when no Discord field is present.
func TestBuildDiscordPayloadWithoutFields(t *testing.T) {
	t.Parallel()

	discord, err := (&Service{}).buildDiscordPayload(presenceFlags{}, "", types.Params{}, &Config{})

	require.ErrorIs(t, err, ErrNoDiscordFields)
	assert.Nil(t, discord)
}
