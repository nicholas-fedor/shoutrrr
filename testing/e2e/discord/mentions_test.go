package e2e_test

import (
	"bytes"
	"encoding/json/v2"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// replyRecorder is an HTTP client that sends through [http.DefaultClient] and
// keeps the body of the last response, which with wait=true is the message
// Discord created.
type replyRecorder struct {
	// body is the body of the last response.
	body []byte
}

// createdMessage is the part of the message object Discord returns that
// records which mentions it parsed.
type createdMessage struct {
	// Content is the message content.
	Content string `json:"content"`
	// Mentions lists the users the message pinged.
	Mentions []mentionedUser `json:"mentions"`
	// MentionRoles lists the IDs of the roles the message pinged.
	MentionRoles []string `json:"mention_roles"`
	// MentionEveryone reports whether the message pinged @everyone or @here.
	MentionEveryone bool `json:"mention_everyone"`
}

// mentionedUser is the part of a mentioned user object the test reads.
type mentionedUser struct {
	// ID is the user's ID.
	ID string `json:"id"`
}

// Do sends req and keeps a copy of the response body.
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

	defer func() { _ = res.Body.Close() }()

	r.body, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	res.Body = io.NopCloser(bytes.NewReader(r.body))

	return res, nil
}

// TestDiscordMentions verifies with the messages Discord created that the
// mentions key makes a message ping, and that a mention in the text alone
// does not. Without the key, Discord would ping @everyone from a channel
// webhook's content.
//
// The user and role cases need SHOUTRRR_DISCORD_USER_ID and
// SHOUTRRR_DISCORD_ROLE_ID and skip without them. Each case reads which
// mentions Discord parsed from the reply that wait=true returns.
//
//nolint:paralleltest // Sends to a live webhook one message at a time.
func TestDiscordMentions(t *testing.T) {
	userID := os.Getenv("SHOUTRRR_DISCORD_USER_ID")
	roleID := os.Getenv("SHOUTRRR_DISCORD_ROLE_ID")

	// sends lists the messages with the query values set on the URL for each,
	// the environment variable a case needs, if any, and the mentions Discord
	// should parse.
	sends := []struct {
		query        map[string]string
		name         string
		message      string
		needs        string
		wantPrefix   string
		wantUser     string
		wantRole     string
		wantEveryone bool
	}{
		{
			name:    "everyone in the text alone does not ping",
			message: "E2E Test: @everyone in the text without the mentions key",
		},
		{
			name:         "here pings from plain content",
			query:        map[string]string{"mentions": "here"},
			message:      "E2E Test: mentions=here on plain content",
			wantPrefix:   "@here ",
			wantEveryone: true,
		},
		{
			name:         "here pings beside embeds",
			query:        map[string]string{"mentions": "here", "title": "E2E Test"},
			message:      "E2E Test: mentions=here on an embed message",
			wantPrefix:   "@here",
			wantEveryone: true,
		},
		{
			name:       "user pings",
			query:      map[string]string{"mentions": "user:" + userID},
			message:    "E2E Test: mentions=user:ID",
			needs:      "SHOUTRRR_DISCORD_USER_ID",
			wantPrefix: "<@" + userID + "> ",
			wantUser:   userID,
		},
		{
			name:       "role pings",
			query:      map[string]string{"mentions": "role:" + roleID},
			message:    "E2E Test: mentions=role:ID",
			needs:      "SHOUTRRR_DISCORD_ROLE_ID",
			wantPrefix: "<@&" + roleID + "> ",
			wantRole:   roleID,
		},
	}

	for _, tt := range sends {
		t.Run(tt.name, func(t *testing.T) {
			if tt.needs != "" && os.Getenv(tt.needs) == "" {
				t.Skipf("%s not set, skipping mention test", tt.needs)
			}

			service := newService(t, tt.query)
			recorder := &replyRecorder{body: nil}
			service.SetHTTPClient(recorder)

			require.NoError(t, service.Send(tt.message, nil))

			var created createdMessage
			require.NoError(t, json.Unmarshal(recorder.body, &created), "Discord returns the created message")

			assert.True(t, strings.HasPrefix(created.Content, tt.wantPrefix), "content %q starts with %q", created.Content, tt.wantPrefix)
			assert.Equal(t, tt.wantEveryone, created.MentionEveryone, "mention_everyone")

			if tt.wantUser != "" {
				assert.True(t, slices.ContainsFunc(created.Mentions, func(user mentionedUser) bool {
					return user.ID == tt.wantUser
				}), "mentions holds the user")
			}

			if tt.wantRole != "" {
				assert.Contains(t, created.MentionRoles, tt.wantRole, "mention_roles holds the role")
			}
		})
	}
}
