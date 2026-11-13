package e2e_test

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
)

// replyRecorder is an HTTP client that sends through [http.DefaultClient] and
// keeps the body of each successful response, which with wait=true is the
// message Discord created. Responses the service retries, such as a rate
// limit, are left out.
type replyRecorder struct {
	// bodies holds the body of each successful response, in order.
	bodies [][]byte
}

// createdMessage is the part of the message object Discord returns that the
// tests read.
type createdMessage struct {
	// ID is the message's ID.
	ID string `json:"id"`
	// ChannelID is the channel the message was sent in, which for a message
	// that created a thread is that thread.
	ChannelID string `json:"channel_id"`
	// Content is the message content.
	Content string `json:"content"`
	// Mentions lists the users the message pinged.
	Mentions []mentionedUser `json:"mentions"`
	// MentionRoles lists the IDs of the roles the message pinged.
	MentionRoles []string `json:"mention_roles"`
	// MentionEveryone reports whether the message pinged @everyone or @here.
	MentionEveryone bool `json:"mention_everyone"`
}

// mentionedUser is the part of a mentioned user object the tests read.
type mentionedUser struct {
	// ID is the user's ID.
	ID string `json:"id"`
}

// Environment variables that hold the webhook URLs the tests send to.
const (
	// webhookURLVar holds the service URL of a webhook on a text channel.
	webhookURLVar = "SHOUTRRR_DISCORD_URL"
	// forumURLVar holds the service URL of a webhook on a forum channel,
	// where a webhook can create threads.
	forumURLVar = "SHOUTRRR_DISCORD_FORUM_URL"
)

// testDelay spaces the tests out to avoid hitting Discord's rate limits.
const testDelay = 100 * time.Millisecond

// TestMain loads the .env file in this directory, if any, before running the
// tests. Values in the file replace those already in the environment.
//
// Environment variables:
//   - SHOUTRRR_DISCORD_URL: the Discord webhook service URL, such as discord://TOKEN@WEBHOOK_ID
//   - SHOUTRRR_DISCORD_FORUM_URL: the service URL of a webhook on a forum channel (optional)
//   - SHOUTRRR_DISCORD_THREAD_ID: an existing thread in the webhook's channel (optional)
//   - SHOUTRRR_DISCORD_USER_ID: a user the mention test pings (optional)
//   - SHOUTRRR_DISCORD_ROLE_ID: a mentionable role the mention test pings (optional)
func TestMain(m *testing.M) {
	loadEnvFile(".env")

	os.Exit(m.Run())
}

// newService initializes a discord.Service from the service URL in an
// environment variable with the given query values set, skipping the test
// when the URL is unset, then waits [testDelay] to avoid hitting Discord's
// rate limits.
//
// Parameters:
//   - t: the test.
//   - urlVar: the environment variable that holds the service URL.
//   - query: the query values to set, or nil.
//
// Returns:
//   - *discord.Service: the initialized service.
func newService(t *testing.T, urlVar string, query map[string]string) *discord.Service {
	t.Helper()

	envURL := os.Getenv(urlVar)
	if envURL == "" {
		t.Skipf("%s not set, skipping Discord e2e test", urlVar)
	}

	time.Sleep(testDelay)

	serviceURL, err := url.Parse(envURL)
	require.NoError(t, err)

	values := serviceURL.Query()
	for key, value := range query {
		values.Set(key, value)
	}

	serviceURL.RawQuery = values.Encode()

	service := &discord.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "[Test] ", 0)))

	return service
}

// Do sends req and keeps a copy of the body of a successful response.
//
// Parameters:
//   - req: the request to send.
//
// Returns:
//   - *http.Response: the response, with a body that can still be read.
//   - error: the transport or read failure.
func (r *replyRecorder) Do(req *http.Request) (*http.Response, error) {
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// The response gets a copy as its body, so the original is closed here.
	original := res.Body
	defer func() { _ = original.Close() }()

	body, err := io.ReadAll(original)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusOK {
		r.bodies = append(r.bodies, body)
	}

	res.Body = io.NopCloser(bytes.NewReader(body))

	return res, nil
}

// loadEnvFile sets the variables in a .env file, replacing any values already
// in the environment.
//
// Parameters:
//   - filename: the .env file, which may be missing.
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		// .env file doesn't exist, skip loading
		return
	}

	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			_ = os.Setenv(key, value) // Ignore error as it's test setup
		}
	}
}
