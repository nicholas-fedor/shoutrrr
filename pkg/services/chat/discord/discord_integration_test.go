package discord_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// recordedRequest is a request the fake webhook received.
type recordedRequest struct {
	// header holds the request headers.
	header http.Header
	// method is the HTTP method.
	method string
	// url is the full request URL.
	url string
	// body is the request body.
	body []byte
}

// reply is a scripted answer from the fake webhook: a response, or a
// transport error when err is set.
type reply struct {
	// header holds the response headers.
	header http.Header
	// err is the transport error returned instead of a response.
	err error
	// body is the response body.
	body string
	// status is the response status code.
	status int
}

// webhook is a fake Discord webhook endpoint that records each request and
// answers it with the next scripted reply.
type webhook struct {
	// requests holds the requests in the order received.
	requests []recordedRequest
	// replies holds the scripted replies, used in order.
	replies []reply
	// mu guards requests.
	mu sync.Mutex
}

// sentFile is a file part of a multipart request.
type sentFile struct {
	// field is the form field name.
	field string
	// name is the file name from the Content-Disposition header.
	name string
	// data is the file content.
	data []byte
}

// wirePayload is the part of the webhook JSON payload the tests decode.
type wirePayload struct {
	// Content is the plain message content.
	Content string `json:"content"`
	// Username is the username override.
	Username string `json:"username"`
	// Embeds holds the embeds.
	Embeds []wireEmbed `json:"embeds"`
	// Attachments holds the attachment metadata for the uploaded files.
	Attachments []wireAttachment `json:"attachments"`
}

// wireEmbed is the part of an embed the tests decode.
type wireEmbed struct {
	// Author is the embed author, or nil when the embed has none.
	Author *wireAuthor `json:"author"`
	// Title is the embed title.
	Title string `json:"title"`
	// Description is the embed text.
	Description string `json:"description"`
	// Fields holds the embed fields.
	Fields []wireField `json:"fields"`
}

// wireAuthor is the part of an embed author the tests decode.
type wireAuthor struct {
	// Name is the author name.
	Name string `json:"name"`
}

// wireField is an embed field.
type wireField struct {
	// Name is the field name.
	Name string `json:"name"`
	// Value is the field value.
	Value string `json:"value"`
}

// wireAttachment is the attachment metadata for one uploaded file.
type wireAttachment struct {
	// Filename is the uploaded file's name.
	Filename string `json:"filename"`
	// ID is the index of the file part.
	ID int `json:"id"`
}

// timeoutError is a [net.Error] that reports a timeout, which the service
// retries at the transport level.
type timeoutError struct{}

// roundTripFunc adapts a function to an [http.RoundTripper].
type roundTripFunc func(req *http.Request) (*http.Response, error)

// Service URLs and messages shared by the tests.
const (
	// testServiceURL is the service URL for webhook test-webhook with token
	// test-token.
	testServiceURL = "discord://test-token@test-webhook"
	// testPostURL is the webhook API URL for testServiceURL.
	testPostURL = "https://discord.com/api/webhooks/test-webhook/test-token"
	// testMessage is the message the tests send when its text does not matter.
	testMessage = "Test message"
)

// errNetwork is a transport error that is not transient.
var errNetwork = errors.New("network connection failed")

// noContent is the reply Discord gives a successful webhook execution.
var noContent = reply{status: http.StatusNoContent}

// Compile-time check that timeoutError is a net.Error.
var _ net.Error = timeoutError{}

// Do records req and answers it with the next scripted reply.
//
// Parameters:
//   - req: the request the service sent.
//
// Returns:
//   - *http.Response: the scripted response, or nil for a transport error.
//   - error: the scripted transport error, or nil.
func (w *webhook) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("reading request body: %w", err)
	}

	w.mu.Lock()
	index := len(w.requests)
	w.requests = append(w.requests, recordedRequest{
		header: req.Header.Clone(),
		method: req.Method,
		url:    req.URL.String(),
		body:   body,
	})
	next := w.replies[index]
	w.mu.Unlock()

	if next.err != nil {
		return nil, next.err
	}

	header := next.header.Clone()
	if header == nil {
		header = http.Header{}
	}

	return &http.Response{
		StatusCode: next.status,
		Status:     strconv.Itoa(next.status) + " " + http.StatusText(next.status),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(next.body)),
		Request:    req,
	}, nil
}

// sent returns the requests received so far.
//
// Returns:
//   - []recordedRequest: the requests, in order.
func (w *webhook) sent() []recordedRequest {
	w.mu.Lock()
	defer w.mu.Unlock()

	return append([]recordedRequest(nil), w.requests...)
}

// Error returns the error message.
//
// Returns:
//   - string: the message.
func (timeoutError) Error() string { return "i/o timeout" }

// Temporary reports that the error is not temporary.
//
// Returns:
//   - bool: false.
func (timeoutError) Temporary() bool { return false }

// Timeout reports that the error is a timeout.
//
// Returns:
//   - bool: true.
func (timeoutError) Timeout() bool { return true }

// RoundTrip calls f with the request.
//
// Parameters:
//   - req: the request to send.
//
// Returns:
//   - *http.Response: the response from f.
//   - error: the error from f.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TestIntegrationSendPlainRequest verifies the request a plain Send makes for
// a service initialized from a URL: a JSON POST with the Shoutrrr user agent
// to the webhook API URL, with the thread, username, avatar, title, colors,
// and line splitting from the URL and the send params, and the message text
// carried exactly.
func TestIntegrationSendPlainRequest(t *testing.T) {
	t.Parallel()

	// tests lists service URLs, params, and messages with the URL and JSON body
	// the request should have.
	tests := []struct {
		name     string
		rawURL   string
		params   *types.Params
		message  string
		wantURL  string
		wantBody string
	}{
		{
			name:     "plain message",
			rawURL:   testServiceURL,
			message:  testMessage,
			wantURL:  testPostURL,
			wantBody: `{"content":"Test message"}`,
		},
		{
			name:     "empty params",
			rawURL:   testServiceURL,
			params:   &types.Params{},
			message:  testMessage,
			wantURL:  testPostURL,
			wantBody: `{"content":"Test message"}`,
		},
		{
			name:     "numeric webhook ID and token",
			rawURL:   "discord://123@456",
			message:  testMessage,
			wantURL:  "https://discord.com/api/webhooks/456/123",
			wantBody: `{"content":"Test message"}`,
		},
		{
			name:     "thread from the URL",
			rawURL:   testServiceURL + "?thread_id=123456789",
			message:  testMessage,
			wantURL:  testPostURL + "?thread_id=123456789",
			wantBody: `{"content":"Test message"}`,
		},
		{
			name:     "thread ID with punctuation",
			rawURL:   testServiceURL + "?thread_id=thread-123_456.789",
			message:  testMessage,
			wantURL:  testPostURL + "?thread_id=thread-123_456.789",
			wantBody: `{"content":"Test message"}`,
		},
		{
			name:     "thread ID with surrounding spaces",
			rawURL:   testServiceURL + "?thread_id=%20%20123456789%20%20",
			message:  testMessage,
			wantURL:  testPostURL + "?thread_id=123456789",
			wantBody: `{"content":"Test message"}`,
		},
		{
			name:     "thread from params replaces the URL's",
			rawURL:   testServiceURL + "?thread_id=111222333",
			params:   &types.Params{"thread_id": "444555666"},
			message:  testMessage,
			wantURL:  testPostURL + "?thread_id=444555666",
			wantBody: `{"content":"Test message"}`,
		},
		{
			name:     "username and avatar from the URL",
			rawURL:   testServiceURL + "?username=URLBot&avatar=https://example.com/url-avatar.png",
			message:  testMessage,
			wantURL:  testPostURL,
			wantBody: `{"content":"Test message","username":"URLBot","avatar_url":"https://example.com/url-avatar.png"}`,
		},
		{
			name:     "username and avatar from params",
			rawURL:   testServiceURL,
			params:   &types.Params{"username": "テストボット", "avatar": "https://example.com/avatar.png?size=128"},
			message:  testMessage,
			wantURL:  testPostURL,
			wantBody: `{"content":"Test message","username":"テストボット","avatar_url":"https://example.com/avatar.png?size=128"}`,
		},
		{
			name:     "params replace the URL's username",
			rawURL:   testServiceURL + "?username=URLBot",
			params:   &types.Params{"username": "ParamBot"},
			message:  testMessage,
			wantURL:  testPostURL,
			wantBody: `{"content":"Test message","username":"ParamBot"}`,
		},
		{
			name:     "avatarurl alias with a data URL",
			rawURL:   testServiceURL,
			params:   &types.Params{"avatarurl": "data:image/png;base64,iVBORw0KGgo="},
			message:  testMessage,
			wantURL:  testPostURL,
			wantBody: `{"content":"Test message","avatar_url":"data:image/png;base64,iVBORw0KGgo="}`,
		},
		{
			name:     "thread with username and avatar",
			rawURL:   testServiceURL + "?thread_id=custom-thread",
			params:   &types.Params{"username": "ThreadBot", "avatar": "https://example.com/thread-avatar.png"},
			message:  "Thread message with custom appearance",
			wantURL:  testPostURL + "?thread_id=custom-thread",
			wantBody: `{"content":"Thread message with custom appearance","username":"ThreadBot","avatar_url":"https://example.com/thread-avatar.png"}`,
		},
		{
			name:     "title from params makes an embed with the default color",
			rawURL:   testServiceURL,
			params:   &types.Params{"title": "Custom Title"},
			message:  testMessage,
			wantURL:  testPostURL,
			wantBody: `{"embeds":[{"title":"Custom Title","description":"Test message","color":5298687}]}`,
		},
		{
			name:     "title and hex color from the URL",
			rawURL:   testServiceURL + "?title=Alert&color=ff0000",
			message:  testMessage,
			wantURL:  testPostURL,
			wantBody: `{"embeds":[{"title":"Alert","description":"Test message","color":16711680}]}`,
		},
		{
			name:     "color with a hash prefix from params",
			rawURL:   testServiceURL,
			params:   &types.Params{"title": "Alert", "color": "#00ff00"},
			message:  testMessage,
			wantURL:  testPostURL,
			wantBody: `{"embeds":[{"title":"Alert","description":"Test message","color":65280}]}`,
		},
		{
			name:   "every param together, with color digits read as hex",
			rawURL: testServiceURL,
			params: &types.Params{
				"username":   "ComplexBot",
				"avatar":     "https://example.com/complex-avatar.png",
				"color":      "16776960",
				"title":      "Complex Test",
				"splitLines": "false",
			},
			message:  "Complex parameter combination test",
			wantURL:  testPostURL,
			wantBody: `{"embeds":[{"title":"Complex Test","description":"Complex parameter combination test","color":376924512}],"username":"ComplexBot","avatar_url":"https://example.com/complex-avatar.png"}`,
		},
		{
			name:     "each line becomes an embed and empty lines are skipped",
			rawURL:   testServiceURL,
			message:  "Line 1\nLine 2\n\nLine 3",
			wantURL:  testPostURL,
			wantBody: `{"embeds":[{"description":"Line 1","color":5298687},{"description":"Line 2","color":5298687},{"description":"Line 3","color":5298687}]}`,
		},
		{
			name:     "lines stay together when splitLines is off",
			rawURL:   testServiceURL,
			params:   &types.Params{"splitLines": "false"},
			message:  "Line 1\nLine 2",
			wantURL:  testPostURL,
			wantBody: `{"content":"Line 1\nLine 2"}`,
		},
		{
			name:     "unicode text",
			rawURL:   testServiceURL,
			message:  "Hello 🌍 世界 こんにちは مرحبا café naïve",
			wantURL:  testPostURL,
			wantBody: `{"content":"Hello 🌍 世界 こんにちは مرحبا café naïve"}`,
		},
		{
			name:     "zero-width spaces",
			rawURL:   testServiceURL,
			message:  "test\u200bhidden\u200btext",
			wantURL:  testPostURL,
			wantBody: `{"content":"test\u200bhidden\u200btext"}`,
		},
		{
			name:     "control characters",
			rawURL:   testServiceURL,
			message:  "Col1\tCol2\x00More\x1b[31mRed\x1b[0m\fPage\vTab\bEnd",
			wantURL:  testPostURL,
			wantBody: `{"content":"Col1\tCol2\u0000More\u001b[31mRed\u001b[0m\fPage\u000bTab\bEnd"}`,
		},
		{
			name:     "invalid UTF-8 becomes replacement characters",
			rawURL:   testServiceURL,
			message:  string([]byte{0x80, 0x81, 0x82}),
			wantURL:  testPostURL,
			wantBody: `{"content":"\ufffd\ufffd\ufffd"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, hook := newWebhook(t, noContent)

			require.NoError(t, newService(t, tt.rawURL, client).Send(tt.message, tt.params))

			sent := hook.sent()
			require.Len(t, sent, 1)
			assert.Equal(t, http.MethodPost, sent[0].method)
			assert.Equal(t, tt.wantURL, sent[0].url)
			assert.Equal(t, "application/json", sent[0].header.Get("Content-Type"))
			assert.Equal(t, meta.UserAgent(), sent[0].header.Get("User-Agent"))
			assert.JSONEq(t, tt.wantBody, string(sent[0].body))
		})
	}
}

// TestIntegrationSendParamsApplyToOneSend verifies that send params change
// only the send they are passed to, so the next send uses the URL's settings.
func TestIntegrationSendParamsApplyToOneSend(t *testing.T) {
	t.Parallel()

	client, hook := newWebhook(t, noContent, noContent)
	service := newService(t, testServiceURL+"?thread_id=123", client)

	require.NoError(t, service.Send("First", &types.Params{"username": "OnceBot", "thread_id": "456", "title": "Once"}))
	require.NoError(t, service.Send("Second", nil))

	sent := hook.sent()
	require.Len(t, sent, 2)
	assert.Equal(t, testPostURL+"?thread_id=456", sent[0].url)
	assert.JSONEq(t, `{"embeds":[{"title":"Once","description":"First","color":5298687}],"username":"OnceBot"}`, string(sent[0].body))
	assert.Equal(t, testPostURL+"?thread_id=123", sent[1].url)
	assert.JSONEq(t, `{"content":"Second"}`, string(sent[1].body))
}

// TestIntegrationSendJSONMode verifies that JSON mode, enabled by the json
// query option or the /raw path, posts the message unchanged as the request
// body, even when it is not valid JSON.
func TestIntegrationSendJSONMode(t *testing.T) {
	t.Parallel()

	// tests lists JSON mode service URLs and messages with the URL the request
	// should go to.
	tests := []struct {
		name    string
		rawURL  string
		message string
		wantURL string
	}{
		{
			name:    "json option",
			rawURL:  testServiceURL + "?json=true",
			message: `{"content": "Raw JSON message", "tts": true, "embeds": [{"title": "JSON Embed"}]}`,
			wantURL: testPostURL,
		},
		{
			name:    "raw path",
			rawURL:  testServiceURL + "/raw",
			message: `{"content": "Raw JSON message"}`,
			wantURL: testPostURL,
		},
		{
			name:    "invalid JSON is passed through",
			rawURL:  testServiceURL + "?json=true",
			message: `{"invalid": json}`,
			wantURL: testPostURL,
		},
		{
			name:    "thread from the URL",
			rawURL:  testServiceURL + "?json=true&thread_id=987654321",
			message: `{"content": "Thread JSON message"}`,
			wantURL: testPostURL + "?thread_id=987654321",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, hook := newWebhook(t, noContent)

			require.NoError(t, newService(t, tt.rawURL, client).Send(tt.message, nil))

			sent := hook.sent()
			require.Len(t, sent, 1)
			assert.Equal(t, tt.wantURL, sent[0].url)
			assert.Equal(t, "application/json", sent[0].header.Get("Content-Type"))
			assert.Equal(t, tt.message, string(sent[0].body))
		})
	}
}

// TestIntegrationSendLongMessage verifies that a long message is spread over
// as many requests as Discord's limits need, with no embed over the chunk
// size, no request over the total size including its title, and no request
// over the embed count, and that the text survives intact. A single line
// longer than a chunk is shortened when lines are split.
func TestIntegrationSendLongMessage(t *testing.T) {
	t.Parallel()

	shortLines := make([]string, 25)
	for i := range shortLines {
		shortLines[i] = "Line " + strconv.Itoa(i+1)
	}

	longLines := make([]string, 12)
	for i := range longLines {
		longLines[i] = strings.Repeat(strconv.Itoa(i%10), 1000)
	}

	chunked := strings.Repeat("a", 2*discord.TotalChunkSize+500)

	// tests lists long messages with the number of requests they need, the
	// separator between the sent pieces, and the text the pieces join to.
	tests := []struct {
		name         string
		rawURL       string
		params       *types.Params
		message      string
		separator    string
		wantText     string
		wantRequests int
	}{
		{
			name:         "chunked when lines are not split",
			rawURL:       testServiceURL + "?splitLines=false",
			message:      chunked,
			wantText:     chunked,
			wantRequests: 3,
		},
		{
			name:         "lines batched by embed count",
			rawURL:       testServiceURL,
			message:      strings.Join(shortLines, "\n"),
			separator:    "\n",
			wantText:     strings.Join(shortLines, "\n"),
			wantRequests: 3,
		},
		{
			name:         "lines batched by total length with a title on each batch",
			rawURL:       testServiceURL,
			params:       &types.Params{"title": strings.Repeat("t", 100)},
			message:      strings.Join(longLines, "\n"),
			separator:    "\n",
			wantText:     strings.Join(longLines, "\n"),
			wantRequests: 3,
		},
		{
			name:         "long line shortened when lines are split",
			rawURL:       testServiceURL,
			message:      strings.Repeat("a", discord.ChunkSize+1000),
			wantText:     strings.Repeat("a", discord.ChunkSize-len(" [...]")) + " [...]",
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, hook := newWebhook(t, repeatReply(noContent, tt.wantRequests)...)

			require.NoError(t, newService(t, tt.rawURL, client).Send(tt.message, tt.params))

			var title string
			if tt.params != nil {
				title = (*tt.params)["title"]
			}

			var pieces []string

			for _, req := range hook.sent() {
				payload := decodePayload(t, req.body)
				total := utf8.RuneCountInString(payload.Content)

				if payload.Content != "" {
					pieces = append(pieces, payload.Content)
				}

				assert.LessOrEqual(t, len(payload.Embeds), discord.MaxEmbeds)

				for i, embed := range payload.Embeds {
					if i == 0 {
						assert.Equal(t, title, embed.Title)
					}

					assert.LessOrEqual(t, utf8.RuneCountInString(embed.Description), discord.ChunkSize)
					total += utf8.RuneCountInString(embed.Title) + utf8.RuneCountInString(embed.Description)
					pieces = append(pieces, embed.Description)
				}

				assert.LessOrEqual(t, total, discord.TotalChunkSize)
			}

			assert.Equal(t, tt.wantText, strings.Join(pieces, tt.separator))
		})
	}
}

// TestIntegrationSendBatchFailure verifies that when one batch of a long
// message fails, the remaining batches are still sent and the send reports the
// failure.
func TestIntegrationSendBatchFailure(t *testing.T) {
	t.Parallel()

	client, hook := newWebhook(t, reply{status: http.StatusBadRequest, body: `{"message": "Invalid Form Body"}`}, noContent)
	service := newService(t, testServiceURL+"?splitLines=false", client)

	err := service.Send(strings.Repeat("a", discord.TotalChunkSize+100), nil)

	require.ErrorIs(t, err, discord.ErrUnexpectedStatus)
	assert.Contains(t, err.Error(), "failed to send discord notification")
	assert.Len(t, hook.sent(), 2)
}

// TestIntegrationSendItemsEmbeds verifies the JSON request SendItemsContext
// makes for rich items: plain content for a single plain item, and otherwise
// embeds with the author, image, thumbnail, fields, level colors and footers,
// UTC timestamps, title, username, and avatar, sent to the configured thread.
func TestIntegrationSendItemsEmbeds(t *testing.T) {
	t.Parallel()

	// tests lists service URLs, params, and items with the URL and JSON body the
	// request should have.
	tests := []struct {
		name     string
		rawURL   string
		params   types.Params
		items    []types.MessageItem
		wantURL  string
		wantBody string
	}{
		{
			name:     "single plain item becomes content",
			rawURL:   testServiceURL,
			items:    []types.MessageItem{{Text: "Test plain text message"}},
			wantURL:  testPostURL,
			wantBody: `{"content":"Test plain text message"}`,
		},
		{
			name:   "embed with every part",
			rawURL: testServiceURL,
			items: []types.MessageItem{{
				Text: "Deployment finished",
				Fields: []types.Field{
					{Key: "embed_author_name", Value: "Test Author"},
					{Key: "embed_author_url", Value: "https://example.com/author"},
					{Key: "embed_author_icon_url", Value: "https://example.com/icon.png"},
					{Key: "embed_image_url", Value: "https://example.com/image.png"},
					{Key: "embed_thumbnail_url", Value: "https://example.com/thumb.png"},
					{Key: "Status", Value: "Active"},
					{Key: "Priority", Value: "High"},
					{Key: "Assignee", Value: "John Doe"},
				},
				Level:     types.Info,
				Timestamp: time.Date(2023, 12, 25, 13, 0, 0, 0, time.FixedZone("CET", 3600)),
			}},
			wantURL: testPostURL,
			wantBody: `{"embeds":[{
				"description":"Deployment finished",
				"color":2394367,
				"footer":{"text":"Info"},
				"timestamp":"2023-12-25T12:00:00Z",
				"author":{"name":"Test Author","url":"https://example.com/author","icon_url":"https://example.com/icon.png"},
				"image":{"url":"https://example.com/image.png"},
				"thumbnail":{"url":"https://example.com/thumb.png"},
				"fields":[{"name":"Status","value":"Active"},{"name":"Priority","value":"High"},{"name":"Assignee","value":"John Doe"}]
			}]}`,
		},
		{
			name:   "several items with default level colors and a title",
			rawURL: testServiceURL,
			params: types.Params{"title": "Status report"},
			items: []types.MessageItem{
				{Text: "Disk full", Level: types.Error},
				{Text: "Disk almost full", Level: types.Warning},
				{Text: "Disk check ran", Level: types.Debug},
				{Text: "Disk report"},
			},
			wantURL: testPostURL,
			wantBody: `{"embeds":[
				{"title":"Status report","description":"Disk full","color":14026000,"footer":{"text":"Error"}},
				{"description":"Disk almost full","color":16761921,"footer":{"text":"Warning"}},
				{"description":"Disk check ran","color":8061099,"footer":{"text":"Debug"}},
				{"description":"Disk report","color":5298687}
			]}`,
		},
		{
			name:   "level colors from the URL",
			rawURL: testServiceURL + "?colorError=ff0000&colorInfo=0x00ff00",
			items: []types.MessageItem{
				{Text: "Failed", Level: types.Error},
				{Text: "Recovered", Level: types.Info},
			},
			wantURL: testPostURL,
			wantBody: `{"embeds":[
				{"description":"Failed","color":16711680,"footer":{"text":"Error"}},
				{"description":"Recovered","color":65280,"footer":{"text":"Info"}}
			]}`,
		},
		{
			name:    "several plain items become embeds",
			rawURL:  testServiceURL,
			items:   []types.MessageItem{{Text: "First message"}, {Text: "Second message"}, {Text: "Third message"}},
			wantURL: testPostURL,
			wantBody: `{"embeds":[
				{"description":"First message","color":5298687},
				{"description":"Second message","color":5298687},
				{"description":"Third message","color":5298687}
			]}`,
		},
		{
			name:     "username and avatar from params",
			rawURL:   testServiceURL,
			params:   types.Params{"username": "TestBot", "avatar": "https://example.com/avatar.png"},
			items:    []types.MessageItem{{Text: "Message with params"}},
			wantURL:  testPostURL,
			wantBody: `{"content":"Message with params","username":"TestBot","avatar_url":"https://example.com/avatar.png"}`,
		},
		{
			name:   "embed sent to the thread from the URL",
			rawURL: testServiceURL + "?thread_id=embed-thread",
			items: []types.MessageItem{{
				Text:   "Thread message with rich embed",
				Fields: []types.Field{{Key: "Thread", Value: "Yes"}, {Key: "Type", Value: "Test"}},
				Level:  types.Info,
			}},
			wantURL: testPostURL + "?thread_id=embed-thread",
			wantBody: `{"embeds":[{
				"description":"Thread message with rich embed",
				"color":2394367,
				"footer":{"text":"Info"},
				"fields":[{"name":"Thread","value":"Yes"},{"name":"Type","value":"Test"}]
			}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, hook := newWebhook(t, noContent)

			require.NoError(t, newService(t, tt.rawURL, client).SendItemsContext(t.Context(), tt.items, tt.params))

			sent := hook.sent()
			require.Len(t, sent, 1)
			assert.Equal(t, http.MethodPost, sent[0].method)
			assert.Equal(t, tt.wantURL, sent[0].url)
			assert.Equal(t, "application/json", sent[0].header.Get("Content-Type"))
			assert.Equal(t, meta.UserAgent(), sent[0].header.Get("User-Agent"))
			assert.JSONEq(t, tt.wantBody, string(sent[0].body))
		})
	}
}

// TestIntegrationSendItemsFiles verifies that items with files are sent as one
// multipart request: a payload_json part with an embed per item and an
// attachment entry per file, and a files[n] part per file carrying its name
// and exact content.
func TestIntegrationSendItemsFiles(t *testing.T) {
	t.Parallel()

	largeData := make([]byte, 5*1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	specialNames := []string{
		"file with spaces*star?question.txt",
		"file(1)[2]^caret|pipe.txt",
		"file+plus%percent$dollar@at.txt",
		`file"quote.txt`,
		`file\backslash.txt`,
		"file/slash.txt",
		"file:colon;semicolon<less>greater.txt",
		"file#hash&ampersand.txt",
		"file'apostrophe`backtick.txt",
		"",
	}

	specialItems := make([]types.MessageItem, 0, len(specialNames))
	specialFiles := make([]sentFile, 0, len(specialNames))

	for i, name := range specialNames {
		specialItems = append(specialItems, types.MessageItem{Text: "Test file", File: &types.File{Name: name, Data: []byte("content")}})
		specialFiles = append(specialFiles, sentFile{field: fmt.Sprintf("files[%d]", i), name: name, data: []byte("content")})
	}

	// tests lists service URLs, params, and items with the URL, payload_json
	// part, and file parts the request should have. wantPayload is compared when
	// set.
	tests := []struct {
		name        string
		rawURL      string
		params      types.Params
		items       []types.MessageItem
		wantURL     string
		wantPayload string
		wantFiles   []sentFile
	}{
		{
			name:        "single file",
			rawURL:      testServiceURL,
			items:       []types.MessageItem{{Text: "Test message with file", File: &types.File{Name: "test.txt", Data: []byte("test file content")}}},
			wantURL:     testPostURL,
			wantPayload: `{"embeds":[{"description":"Test message with file","color":5298687}],"attachments":[{"id":0,"filename":"test.txt"}]}`,
			wantFiles:   []sentFile{{field: "files[0]", name: "test.txt", data: []byte("test file content")}},
		},
		{
			name:   "several files of different types",
			rawURL: testServiceURL,
			items: []types.MessageItem{
				{Text: "JSON", File: &types.File{Name: "data.json", Data: []byte(`{"key": "value"}`)}},
				{Text: "Binary", File: &types.File{Name: "data.bin", Data: []byte{0x00, 0x01, 0x02, 0xFF}}},
				{Text: "Image", File: &types.File{Name: "image.png", Data: []byte("fake png content")}},
				{Text: "Unicode", File: &types.File{Name: "unicode.txt", Data: []byte("Hello 世界 🌍 éñüñ")}},
			},
			wantURL: testPostURL,
			wantPayload: `{
				"embeds":[
					{"description":"JSON","color":5298687},
					{"description":"Binary","color":5298687},
					{"description":"Image","color":5298687},
					{"description":"Unicode","color":5298687}
				],
				"attachments":[
					{"id":0,"filename":"data.json"},
					{"id":1,"filename":"data.bin"},
					{"id":2,"filename":"image.png"},
					{"id":3,"filename":"unicode.txt"}
				]
			}`,
			wantFiles: []sentFile{
				{field: "files[0]", name: "data.json", data: []byte(`{"key": "value"}`)},
				{field: "files[1]", name: "data.bin", data: []byte{0x00, 0x01, 0x02, 0xFF}},
				{field: "files[2]", name: "image.png", data: []byte("fake png content")},
				{field: "files[3]", name: "unicode.txt", data: []byte("Hello 世界 🌍 éñüñ")},
			},
		},
		{
			name:        "large file",
			rawURL:      testServiceURL,
			items:       []types.MessageItem{{Text: "Large file test", File: &types.File{Name: "large.bin", Data: largeData}}},
			wantURL:     testPostURL,
			wantPayload: `{"embeds":[{"description":"Large file test","color":5298687}],"attachments":[{"id":0,"filename":"large.bin"}]}`,
			wantFiles:   []sentFile{{field: "files[0]", name: "large.bin", data: largeData}},
		},
		{
			name:        "empty file",
			rawURL:      testServiceURL,
			items:       []types.MessageItem{{Text: "Empty file", File: &types.File{Name: "empty.txt", Data: []byte{}}}},
			wantURL:     testPostURL,
			wantPayload: `{"embeds":[{"description":"Empty file","color":5298687}],"attachments":[{"id":0,"filename":"empty.txt"}]}`,
			wantFiles:   []sentFile{{field: "files[0]", name: "empty.txt", data: []byte{}}},
		},
		{
			name:      "special characters in file names",
			rawURL:    testServiceURL,
			items:     specialItems,
			wantURL:   testPostURL,
			wantFiles: specialFiles,
		},
		{
			name:   "files with an embed, username, and thread",
			rawURL: testServiceURL + "?thread_id=file-thread",
			params: types.Params{"username": "FileBot"},
			items: []types.MessageItem{
				{
					Text:   "First file in thread",
					Fields: []types.Field{{Key: "Build", Value: "42"}},
					Level:  types.Warning,
					File:   &types.File{Name: "file1.txt", Data: []byte("content 1")},
				},
				{Text: "Second file in thread", File: &types.File{Name: "file2.txt", Data: []byte("content 2")}},
			},
			wantURL: testPostURL + "?thread_id=file-thread",
			wantPayload: `{
				"embeds":[
					{"description":"First file in thread","color":16761921,"footer":{"text":"Warning"},"fields":[{"name":"Build","value":"42"}]},
					{"description":"Second file in thread","color":5298687}
				],
				"attachments":[{"id":0,"filename":"file1.txt"},{"id":1,"filename":"file2.txt"}],
				"username":"FileBot"
			}`,
			wantFiles: []sentFile{
				{field: "files[0]", name: "file1.txt", data: []byte("content 1")},
				{field: "files[1]", name: "file2.txt", data: []byte("content 2")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, hook := newWebhook(t, noContent)

			require.NoError(t, newService(t, tt.rawURL, client).SendItemsContext(t.Context(), tt.items, tt.params))

			sent := hook.sent()
			require.Len(t, sent, 1)
			assert.Equal(t, http.MethodPost, sent[0].method)
			assert.Equal(t, tt.wantURL, sent[0].url)
			assert.Equal(t, meta.UserAgent(), sent[0].header.Get("User-Agent"))

			payloadJSON, files := readMultipart(t, sent[0])

			if tt.wantPayload != "" {
				assert.JSONEq(t, tt.wantPayload, payloadJSON)
			}

			payload := decodePayload(t, []byte(payloadJSON))
			assert.Len(t, payload.Embeds, len(tt.items))

			wantAttachments := make([]wireAttachment, 0, len(tt.wantFiles))
			for i, file := range tt.wantFiles {
				wantAttachments = append(wantAttachments, wireAttachment{Filename: file.name, ID: i})
			}

			assert.Equal(t, wantAttachments, payload.Attachments)
			assert.Equal(t, tt.wantFiles, files)
		})
	}
}

// TestIntegrationSendItemsBatches verifies that rich items past Discord's
// limit of 10 embeds per message are sent in further requests, each with the
// title on its first embed and only its own items' files, numbered from zero,
// and that every batch is sent as embeds, even a batch of one plain item.
func TestIntegrationSendItemsBatches(t *testing.T) {
	t.Parallel()

	const itemCount = discord.MaxEmbeds + 2

	plainItems := make([]types.MessageItem, itemCount)
	fileItems := make([]types.MessageItem, itemCount)

	for i := range itemCount {
		text := "Item " + strconv.Itoa(i)
		plainItems[i] = types.MessageItem{Text: text}
		fileItems[i] = types.MessageItem{Text: text, File: &types.File{Name: "file" + strconv.Itoa(i) + ".txt", Data: []byte(text)}}
	}

	t.Run("embeds", func(t *testing.T) {
		t.Parallel()

		client, hook := newWebhook(t, noContent, noContent)

		require.NoError(t, newService(t, testServiceURL, client).SendItemsContext(t.Context(), plainItems, types.Params{"title": "Report"}))

		sent := hook.sent()
		require.Len(t, sent, 2)

		for i, wantTexts := range [][]string{textsOf(plainItems[:discord.MaxEmbeds]), textsOf(plainItems[discord.MaxEmbeds:])} {
			payload := decodePayload(t, sent[i].body)
			require.Len(t, payload.Embeds, len(wantTexts))
			assert.Equal(t, "Report", payload.Embeds[0].Title)

			for j, embed := range payload.Embeds {
				assert.Equal(t, wantTexts[j], embed.Description)
			}
		}
	})

	t.Run("files", func(t *testing.T) {
		t.Parallel()

		client, hook := newWebhook(t, noContent, noContent)

		require.NoError(t, newService(t, testServiceURL, client).SendItemsContext(t.Context(), fileItems, nil))

		sent := hook.sent()
		require.Len(t, sent, 2)

		for i, batch := range [][]types.MessageItem{fileItems[:discord.MaxEmbeds], fileItems[discord.MaxEmbeds:]} {
			payloadJSON, files := readMultipart(t, sent[i])
			payload := decodePayload(t, []byte(payloadJSON))

			wantAttachments := make([]wireAttachment, 0, len(batch))
			wantFiles := make([]sentFile, 0, len(batch))

			for j, item := range batch {
				wantAttachments = append(wantAttachments, wireAttachment{Filename: item.File.Name, ID: j})
				wantFiles = append(wantFiles, sentFile{field: fmt.Sprintf("files[%d]", j), name: item.File.Name, data: item.File.Data})
			}

			assert.Len(t, payload.Embeds, len(batch))
			assert.Equal(t, wantAttachments, payload.Attachments)
			assert.Equal(t, wantFiles, files)
		}
	})

	t.Run("single plain item in the last batch", func(t *testing.T) {
		t.Parallel()

		long := strings.Repeat("a", discord.ChunkSize+1)
		items := append(slices.Clone(plainItems[:discord.MaxEmbeds]), types.MessageItem{Text: long})

		client, hook := newWebhook(t, noContent, noContent)

		require.NoError(t, newService(t, testServiceURL, client).SendItemsContext(t.Context(), items, nil))

		sent := hook.sent()
		require.Len(t, sent, 2)

		payload := decodePayload(t, sent[1].body)
		assert.Empty(t, payload.Content)
		require.Len(t, payload.Embeds, 1)
		assert.Equal(t, long, payload.Embeds[0].Description)
	})

	t.Run("failed batch", func(t *testing.T) {
		t.Parallel()

		client, hook := newWebhook(t, reply{status: http.StatusBadRequest, body: `{"message": "Invalid Form Body"}`}, noContent)

		err := newService(t, testServiceURL, client).SendItemsContext(t.Context(), plainItems, nil)

		require.ErrorIs(t, err, discord.ErrUnexpectedStatus)
		assert.Len(t, hook.sent(), 2)
	})
}

// TestIntegrationSendItemsEmbedLimits verifies that rich sends stay within
// Discord's embed limits: text past a field's limit is shortened with " [...]",
// fields past the 25th are left out, a long single plain item is sent as an
// embed instead of content, items are batched so a message's embeds stay
// within 6000 characters including the title, and a single item over that
// total drops fields from the end.
func TestIntegrationSendItemsEmbedLimits(t *testing.T) {
	t.Parallel()

	shortened := func(r string, limit int) string {
		return strings.Repeat(r, limit-len(" [...]")) + " [...]"
	}

	manyFields := make([]types.Field, 30)
	for i := range manyFields {
		manyFields[i] = types.Field{Key: "f" + strconv.Itoa(i), Value: "v"}
	}

	bigFields := make([]types.Field, 25)
	for i := range bigFields {
		bigFields[i] = types.Field{Key: fmt.Sprintf("%0100d", i), Value: strings.Repeat("v", 1000)}
	}

	itemsOf := func(count, length int, level types.MessageLevel) []types.MessageItem {
		items := make([]types.MessageItem, count)
		for i := range items {
			items[i] = types.MessageItem{Text: strings.Repeat(strconv.Itoa(i), length), Level: level}
		}

		return items
	}

	// tests lists items and params with the embeds each request should carry.
	tests := []struct {
		params types.Params
		name   string
		items  []types.MessageItem
		want   [][]wireEmbed
	}{
		{
			name:  "long description",
			items: []types.MessageItem{{Text: strings.Repeat("a", 5000), Level: types.Info}},
			want:  [][]wireEmbed{{{Description: shortened("a", 4096)}}},
		},
		{
			name:  "long single plain item",
			items: []types.MessageItem{{Text: strings.Repeat("a", discord.ChunkSize+1)}},
			want:  [][]wireEmbed{{{Description: strings.Repeat("a", discord.ChunkSize+1)}}},
		},
		{
			name:   "long title",
			params: types.Params{"title": strings.Repeat("t", 300)},
			items:  []types.MessageItem{{Text: "body"}},
			want:   [][]wireEmbed{{{Title: shortened("t", 256), Description: "body"}}},
		},
		{
			name:  "fields past the 25th",
			items: []types.MessageItem{{Text: "body", Fields: manyFields}},
			want: [][]wireEmbed{{{Description: "body", Fields: func() []wireField {
				fields := make([]wireField, 25)
				for i := range fields {
					fields[i] = wireField{Name: "f" + strconv.Itoa(i), Value: "v"}
				}

				return fields
			}()}}},
		},
		{
			name: "long field name, field value, and author name",
			items: []types.MessageItem{{Text: "body", Fields: []types.Field{
				{Key: strings.Repeat("n", 300), Value: strings.Repeat("v", 1500)},
				{Key: "embed_author_name", Value: strings.Repeat("a", 300)},
			}}},
			want: [][]wireEmbed{{{
				Author:      &wireAuthor{Name: shortened("a", 256)},
				Description: "body",
				Fields:      []wireField{{Name: shortened("n", 256), Value: shortened("v", 1024)}},
			}}},
		},
		{
			name:  "batched by combined length",
			items: itemsOf(4, 2000, types.Info),
			want: [][]wireEmbed{
				{{Description: strings.Repeat("0", 2000)}, {Description: strings.Repeat("1", 2000)}},
				{{Description: strings.Repeat("2", 2000)}, {Description: strings.Repeat("3", 2000)}},
			},
		},
		{
			name:   "title counted in each batch",
			params: types.Params{"title": strings.Repeat("t", 100)},
			items:  itemsOf(3, 1990, types.Unknown),
			want: [][]wireEmbed{
				{{Title: strings.Repeat("t", 100), Description: strings.Repeat("0", 1990)}, {Description: strings.Repeat("1", 1990)}},
				{{Title: strings.Repeat("t", 100), Description: strings.Repeat("2", 1990)}},
			},
		},
		{
			name:  "single item over the combined length",
			items: []types.MessageItem{{Text: strings.Repeat("d", 4000), Fields: bigFields}},
			want: [][]wireEmbed{{{
				Description: strings.Repeat("d", 4000),
				Fields:      []wireField{{Name: fmt.Sprintf("%0100d", 0), Value: strings.Repeat("v", 1000)}},
			}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, hook := newWebhook(t, repeatReply(noContent, len(tt.want))...)

			require.NoError(t, newService(t, testServiceURL, client).SendItemsContext(t.Context(), tt.items, tt.params))

			sent := hook.sent()
			require.Len(t, sent, len(tt.want))

			for i, want := range tt.want {
				payload := decodePayload(t, sent[i].body)
				assert.Empty(t, payload.Content, "request %d", i)
				assert.Equal(t, want, payload.Embeds, "request %d", i)
			}
		})
	}
}

// TestIntegrationSendRetries verifies how a send started with SendContext
// retries with the service's default sleeper: it waits for Retry-After or an
// exponential backoff on rate limits and server errors, retries transient
// transport errors, resends the same payload each time, and reports the
// failure once retries run out, the wait cannot fit the deadline, or the
// context is canceled. The fake clock checks the total wait.
func TestIntegrationSendRetries(t *testing.T) {
	t.Parallel()

	rateLimited := reply{status: http.StatusTooManyRequests, body: `{"message": "You are being rate limited.", "retry_after": 1}`}
	serverError := reply{status: http.StatusInternalServerError, body: `{"message": "Server error"}`}
	timeout := reply{err: timeoutError{}}

	// tests lists scripted replies and contexts with the number of requests,
	// the total wait, and the error to expect.
	tests := []struct {
		name         string
		replies      []reply
		context      func(ctx context.Context) (context.Context, context.CancelFunc)
		wantErr      error
		wantElapsed  time.Duration
		wantRequests int
	}{
		{
			name:         "Retry-After then success",
			replies:      []reply{{status: http.StatusTooManyRequests, header: http.Header{"Retry-After": {"2"}}}, noContent},
			wantElapsed:  2 * time.Second,
			wantRequests: 2,
		},
		{
			name:         "rate limit without Retry-After backs off then succeeds",
			replies:      []reply{rateLimited, noContent},
			wantElapsed:  time.Second,
			wantRequests: 2,
		},
		{
			name:         "server error then success",
			replies:      []reply{{status: http.StatusServiceUnavailable}, noContent},
			wantElapsed:  time.Second,
			wantRequests: 2,
		},
		{
			name:         "transport timeout then success",
			replies:      []reply{timeout, noContent},
			wantElapsed:  time.Second,
			wantRequests: 2,
		},
		{
			name:         "zero Retry-After until retries run out",
			replies:      repeatReply(reply{status: http.StatusTooManyRequests, header: http.Header{"Retry-After": {"0"}}}, 6),
			wantErr:      discord.ErrMaxRetries,
			wantRequests: 6,
		},
		{
			name:         "rate limit backoff until retries run out",
			replies:      repeatReply(rateLimited, 6),
			wantErr:      discord.ErrMaxRetries,
			wantElapsed:  31 * time.Second,
			wantRequests: 6,
		},
		{
			name:         "server errors until retries run out",
			replies:      repeatReply(serverError, 6),
			wantErr:      discord.ErrMaxRetries,
			wantElapsed:  31 * time.Second,
			wantRequests: 6,
		},
		{
			name:         "transport timeouts until retries run out",
			replies:      repeatReply(timeout, 4),
			wantErr:      timeoutError{},
			wantElapsed:  7 * time.Second,
			wantRequests: 4,
		},
		{
			name:         "other transport errors are not retried",
			replies:      []reply{{err: errNetwork}},
			wantErr:      errNetwork,
			wantRequests: 1,
		},
		{
			name:    "Retry-After beyond the send deadline",
			replies: []reply{{status: http.StatusTooManyRequests, header: http.Header{"Retry-After": {"10"}}}},
			context: func(ctx context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, 5*time.Second)
			},
			wantErr:      discord.ErrRateLimited,
			wantRequests: 1,
		},
		{
			name:    "cancel during the backoff",
			replies: []reply{serverError},
			context: func(ctx context.Context) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(ctx)
				time.AfterFunc(500*time.Millisecond, cancel)

				return ctx, cancel
			},
			wantErr:      context.Canceled,
			wantElapsed:  500 * time.Millisecond,
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()

				if tt.context != nil {
					var cancel context.CancelFunc

					ctx, cancel = tt.context(ctx)
					defer cancel()
				}

				client, hook := newWebhook(t, tt.replies...)
				service := newService(t, testServiceURL, client)
				start := time.Now()

				err := service.SendContext(ctx, testMessage, nil)

				assert.Equal(t, tt.wantElapsed, time.Since(start))

				if tt.wantErr == nil {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, tt.wantErr)
					assert.Contains(t, err.Error(), "failed to send discord notification")
				}

				sent := hook.sent()
				require.Len(t, sent, tt.wantRequests)

				for _, req := range sent {
					assert.JSONEq(t, `{"content":"Test message"}`, string(req.body))
				}
			})
		})
	}
}

// TestIntegrationSendStatusErrors verifies that error statuses other than
// rate limits and server errors fail at once with ErrUnexpectedStatus and the
// status, for plain, JSON mode, and rich sends, and that any 2xx status
// Discord uses counts as success whatever the body.
func TestIntegrationSendStatusErrors(t *testing.T) {
	t.Parallel()

	fileItems := []types.MessageItem{{Text: "Test", File: &types.File{Name: "large.txt", Data: make([]byte, 1024)}}}

	// tests lists service URLs, sends, and statuses with the error text to
	// expect, or none for success.
	tests := []struct {
		name     string
		rawURL   string
		send     func(t *testing.T, service *discord.Service) error
		body     string
		wantText string
		status   int
	}{
		{name: "bad request", status: http.StatusBadRequest, body: `{"message": "Invalid webhook"}`, wantText: "failed to send discord notification: unexpected response status code: 400 Bad Request"},
		{name: "unauthorized", status: http.StatusUnauthorized, body: `{"message": "Invalid token"}`, wantText: "401 Unauthorized"},
		{name: "forbidden", status: http.StatusForbidden, body: `{"message": "Missing permissions"}`, wantText: "403 Forbidden"},
		{name: "unknown webhook", status: http.StatusNotFound, body: `{"message": "Unknown Webhook", "code": 10015}`, wantText: "404 Not Found"},
		{
			name:     "JSON mode",
			rawURL:   testServiceURL + "?json=true",
			status:   http.StatusBadRequest,
			body:     `{"message": "Cannot send an empty message"}`,
			wantText: "sending JSON message: unexpected response status code: 400 Bad Request",
		},
		{
			name: "rich send with a file",
			send: func(t *testing.T, service *discord.Service) error {
				t.Helper()

				return service.SendItemsContext(t.Context(), fileItems, nil)
			},
			status:   http.StatusRequestEntityTooLarge,
			body:     `{"message": "Request entity too large"}`,
			wantText: "unexpected response status code: 413 Request Entity Too Large",
		},
		{name: "OK with a body that is not JSON", status: http.StatusOK, body: "invalid json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rawURL := tt.rawURL
			if rawURL == "" {
				rawURL = testServiceURL
			}

			client, hook := newWebhook(t, reply{status: tt.status, body: tt.body})
			service := newService(t, rawURL, client)

			var err error
			if tt.send != nil {
				err = tt.send(t, service)
			} else {
				err = service.Send(testMessage, nil)
			}

			assert.Len(t, hook.sent(), 1)

			if tt.wantText == "" {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, discord.ErrUnexpectedStatus)
			assert.Contains(t, err.Error(), tt.wantText)
		})
	}
}

// TestIntegrationRejectedBeforeSending verifies that sends with an unknown
// param or no items fail without a request. The mock client has no
// expectations, so a request fails the test.
func TestIntegrationRejectedBeforeSending(t *testing.T) {
	t.Parallel()

	// tests lists sends that must fail before any request, with the error text
	// and, when set, the error to expect.
	tests := []struct {
		name     string
		send     func(t *testing.T, service *discord.Service) error
		wantErr  error
		wantText string
	}{
		{
			name: "unknown param",
			send: func(_ *testing.T, service *discord.Service) error {
				return service.Send(testMessage, &types.Params{"invalid_key": "value"})
			},
			wantText: "updating config from params",
		},
		{
			name: "unknown param with more items than one message holds",
			send: func(t *testing.T, service *discord.Service) error {
				t.Helper()

				items := make([]types.MessageItem, discord.MaxEmbeds+1)
				for i := range items {
					items[i] = types.MessageItem{Text: "Item " + strconv.Itoa(i)}
				}

				return service.SendItemsContext(t.Context(), items, types.Params{"invalid_key": "value"})
			},
			wantText: "updating config from params",
		},
		{
			name: "single item without text",
			send: func(t *testing.T, service *discord.Service) error {
				t.Helper()

				return service.SendItemsContext(t.Context(), []types.MessageItem{{Text: ""}}, nil)
			},
			wantErr:  discord.ErrEmptyMessage,
			wantText: "creating payload",
		},
		{
			name: "nil items",
			send: func(t *testing.T, service *discord.Service) error {
				t.Helper()

				return service.SendItemsContext(t.Context(), nil, nil)
			},
			wantErr:  discord.ErrEmptyMessage,
			wantText: "creating payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.send(t, newService(t, testServiceURL, mocks.NewMockHTTPClient(t)))

			require.Error(t, err)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}

			assert.Contains(t, err.Error(), tt.wantText)
		})
	}
}

// TestIntegrationInitializeRejectsPaths verifies that Initialize rejects
// service URLs with a path other than /raw, including URLs pasted from the
// webhook API and URLs without a scheme.
func TestIntegrationInitializeRejectsPaths(t *testing.T) {
	t.Parallel()

	// tests lists service URLs that Initialize must reject.
	tests := []struct {
		name   string
		rawURL string
	}{
		{name: "webhook API URL", rawURL: "http://discord.com/api/webhooks/test/test"},
		{name: "nested path", rawURL: "discord://token@webhook/invalid/path"},
		{name: "no scheme", rawURL: "token@webhook"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL, err := url.Parse(tt.rawURL)
			require.NoError(t, err)

			err = (&discord.Service{}).Initialize(serviceURL, log.New(t.Output(), "", 0))

			require.ErrorIs(t, err, discord.ErrIllegalURLArgument)
			assert.Contains(t, err.Error(), "setting config URL")
		})
	}
}

// TestIntegrationSetHTTPClient verifies that a client injected with
// SetHTTPClient, as the router injects one, carries the send, and that a nil
// client restores the default client in place of the deprecated client field.
func TestIntegrationSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("injected http.Client", func(t *testing.T) {
		t.Parallel()

		requests := make(chan *http.Request, 1)
		transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests <- req

			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Request: req}, nil
		})

		service := newService(t, testServiceURL, nil)
		service.SetHTTPClient(&http.Client{Transport: transport})

		require.NoError(t, service.Send(testMessage, nil))

		req := <-requests
		assert.Equal(t, testPostURL, req.URL.String())
		assert.Equal(t, meta.UserAgent(), req.Header.Get("User-Agent"))
	})

	// nilClients lists the nil clients that restore the default client.
	nilClients := []struct {
		name   string
		client types.HTTPClient
	}{
		{name: "nil interface", client: nil},
		{name: "nil http.Client", client: (*http.Client)(nil)},
	}

	for _, tt := range nilClients {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newService(t, testServiceURL, mocks.NewMockHTTPClient(t))
			service.SetHTTPClient(tt.client)

			// The context ends before the send, so the default client returns
			// before it dials. The mock fails the test if it is called.
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := service.SendContext(ctx, testMessage, nil)

			require.ErrorIs(t, err, context.Canceled)
			assert.Nil(t, service.HTTPClient, "SetHTTPClient clears the deprecated client field")
		})
	}
}

// TestIntegrationConcurrentSends verifies that concurrent sends on one service
// each use their own params, so a username never reaches another send's
// request. Run with -race, it also checks the sends share no unsynchronized
// state.
func TestIntegrationConcurrentSends(t *testing.T) {
	t.Parallel()

	const sends = 10

	client, hook := newWebhook(t, repeatReply(noContent, sends)...)
	service := newService(t, testServiceURL, client)

	errs := make([]error, sends)

	var wg sync.WaitGroup

	for i := range sends {
		wg.Go(func() {
			suffix := strconv.Itoa(i)
			errs[i] = service.Send("message "+suffix, &types.Params{"username": "bot " + suffix})
		})
	}

	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}

	sent := hook.sent()
	require.Len(t, sent, sends)

	for _, req := range sent {
		payload := decodePayload(t, req.body)
		assert.Equal(t, "bot "+strings.TrimPrefix(payload.Content, "message "), payload.Username)
	}
}

// newWebhook returns a mock HTTP client that expects exactly one request per
// reply and answers them in order through a fake webhook that records them.
// A test that expects no request uses [mocks.NewMockHTTPClient] directly.
//
// Parameters:
//   - t: the test that owns the mock.
//   - replies: the replies, at least one.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *webhook: the fake webhook with the recorded requests.
func newWebhook(t *testing.T, replies ...reply) (*mocks.MockHTTPClient, *webhook) {
	t.Helper()
	require.NotEmpty(t, replies, "a mock with no expected calls accepts any number of calls")

	hook := &webhook{replies: replies}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(hook.Do).Times(len(replies))

	return client, hook
}

// newService returns a service initialized from rawURL that sends through
// client and logs to the test output. The client is set before Initialize,
// which keeps it.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - client: the HTTP client, or nil for the default client.
//
// Returns:
//   - *discord.Service: the service.
func newService(t *testing.T, rawURL string, client types.HTTPClient) *discord.Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &discord.Service{HTTPClient: client}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))

	return service
}

// textsOf returns the text of each item.
//
// Parameters:
//   - items: the message items.
//
// Returns:
//   - []string: the texts, in order.
func textsOf(items []types.MessageItem) []string {
	texts := make([]string, 0, len(items))
	for _, item := range items {
		texts = append(texts, item.Text)
	}

	return texts
}

// repeatReply returns count copies of r.
//
// Parameters:
//   - r: the reply to repeat.
//   - count: the number of copies.
//
// Returns:
//   - []reply: the replies.
func repeatReply(r reply, count int) []reply {
	replies := make([]reply, count)
	for i := range replies {
		replies[i] = r
	}

	return replies
}

// decodePayload decodes a webhook JSON payload.
//
// Parameters:
//   - t: the test.
//   - body: the JSON payload.
//
// Returns:
//   - wirePayload: the decoded payload.
func decodePayload(t *testing.T, body []byte) wirePayload {
	t.Helper()

	var payload wirePayload
	require.NoError(t, json.Unmarshal(body, &payload))

	return payload
}

// readMultipart parses a multipart request into its payload_json part and its
// file parts. File names come from the raw Content-Disposition header, since
// [multipart.Part.FileName] drops everything up to the last slash.
//
// Parameters:
//   - t: the test.
//   - req: the recorded request.
//
// Returns:
//   - string: the payload_json part.
//   - []sentFile: the file parts, in order.
func readMultipart(t *testing.T, req recordedRequest) (string, []sentFile) {
	t.Helper()

	mediaType, params, err := mime.ParseMediaType(req.header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)

	reader := multipart.NewReader(bytes.NewReader(req.body), params["boundary"])

	var (
		payloadJSON string
		files       []sentFile
	)

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(t, err)

		data, err := io.ReadAll(part)
		require.NoError(t, err)

		_, disposition, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		require.NoError(t, err)

		if disposition["name"] == "payload_json" {
			payloadJSON = string(data)

			continue
		}

		files = append(files, sentFile{field: disposition["name"], name: disposition["filename"], data: data})
	}

	return payloadJSON, files
}
