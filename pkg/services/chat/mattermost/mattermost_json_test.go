package mattermost

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestJSONSetIcon verifies that SetIcon sets IconURL for an http or https
// icon, IconEmoji for anything else, and clears both for an empty icon.
func TestJSONSetIcon(t *testing.T) {
	t.Parallel()

	// tests lists icons with the icon fields SetIcon should leave set.
	tests := []struct {
		name      string
		icon      string
		wantURL   string
		wantEmoji string
	}{
		{name: "https URL", icon: "https://example.com/logo.png", wantURL: "https://example.com/logo.png"},
		{name: "uppercase http URL", icon: "HTTP://example.com/logo.png", wantURL: "HTTP://example.com/logo.png"},
		{name: "ftp URL", icon: "ftp://example.com/logo.png", wantEmoji: "ftp://example.com/logo.png"},
		{name: "http without slashes", icon: "http:logo", wantEmoji: "http:logo"},
		{name: "long s in the scheme", icon: "httpſ://example.com/logo.png", wantEmoji: "httpſ://example.com/logo.png"},
		{name: "emoji", icon: "tanabata_tree", wantEmoji: "tanabata_tree"},
		{name: "empty", icon: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := JSON{IconEmoji: "old_emoji", IconURL: "https://example.com/old.png"}
			payload.SetIcon(tt.icon)

			assert.Equal(t, tt.wantURL, payload.IconURL)
			assert.Equal(t, tt.wantEmoji, payload.IconEmoji)
		})
	}
}

// TestCreateJSONPayload verifies the payload built from the config and params.
// The title goes on its own line before the message, and either is sent
// alone. A payload without either fails. The user name and channel params
// override the config.
func TestCreateJSONPayload(t *testing.T) {
	t.Parallel()

	// tests lists configs and params with the JSON payload each should produce.
	tests := []struct {
		params  *types.Params
		wantErr error
		config  Config
		name    string
		message string
		want    string
	}{
		{
			name:    "no params",
			message: "this is a message",
			want:    `{"text":"this is a message"}`,
		},
		{
			name:    "configured user name and channel",
			config:  Config{UserName: "testUserName", Channel: "testChannel"},
			message: "this is a message",
			want:    `{"text":"this is a message","username":"testUserName","channel":"testChannel"}`,
		},
		{
			name:    "user name and channel from params",
			config:  Config{UserName: "testUserName", Channel: "testChannel"},
			params:  &types.Params{"username": "overwriteUserName", "channel": "overwriteChannel"},
			message: "this is a message",
			want:    `{"text":"this is a message","username":"overwriteUserName","channel":"overwriteChannel"}`,
		},
		{
			name:    "title",
			config:  Config{Title: "watchtower TEST"},
			message: "this is a message",
			want:    `{"text":"watchtower TEST\nthis is a message"}`,
		},
		{
			name:   "title without a message",
			config: Config{Title: "watchtower TEST"},
			want:   `{"text":"watchtower TEST"}`,
		},
		{
			name:    "no title or message",
			wantErr: ErrMessageMissing,
		},
		{
			name:    "icon emoji",
			config:  Config{Icon: "tanabata_tree"},
			message: "this is a message",
			want:    `{"text":"this is a message","icon_emoji":"tanabata_tree"}`,
		},
		{
			name:    "icon URL in another case",
			config:  Config{Icon: "HTTPS://example.com/Logo.png"},
			message: "this is a message",
			want:    `{"text":"this is a message","icon_url":"HTTPS://example.com/Logo.png"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := CreateJSONPayload(&tt.config, tt.message, tt.params)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(payload))
		})
	}
}
