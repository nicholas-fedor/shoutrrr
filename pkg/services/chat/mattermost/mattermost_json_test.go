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

// TestCreateJSONPayload verifies the payload built from the config and params:
// the title prefix, and the user name, channel, and icon overrides.
func TestCreateJSONPayload(t *testing.T) {
	t.Parallel()

	// tests lists configs and params with the JSON payload each should produce.
	tests := []struct {
		params *types.Params
		config Config
		name   string
		want   string
	}{
		{
			name: "no params",
			want: `{"text":"this is a message"}`,
		},
		{
			name:   "configured user name and channel",
			config: Config{UserName: "testUserName", Channel: "testChannel"},
			want:   `{"text":"this is a message","username":"testUserName","channel":"testChannel"}`,
		},
		{
			name:   "user name and channel from params",
			config: Config{UserName: "testUserName", Channel: "testChannel"},
			params: &types.Params{"username": "overwriteUserName", "channel": "overwriteChannel"},
			want:   `{"text":"this is a message","username":"overwriteUserName","channel":"overwriteChannel"}`,
		},
		{
			name:   "configured title",
			config: Config{Title: "watchtower TEST"},
			want:   `{"text":"watchtower TEST\nthis is a message"}`,
		},
		{
			name:   "title from params",
			params: &types.Params{"title": "watchtower PROD"},
			want:   `{"text":"watchtower PROD\nthis is a message"}`,
		},
		{
			name:   "title from params over configured title",
			config: Config{Title: "Config Title"},
			params: &types.Params{"title": "Params Title"},
			want:   `{"text":"Params Title\nthis is a message"}`,
		},
		{
			name:   "configured icon emoji",
			config: Config{Icon: "tanabata_tree"},
			want:   `{"text":"this is a message","icon_emoji":"tanabata_tree"}`,
		},
		{
			name:   "icon URL from params over configured icon",
			config: Config{Icon: "tanabata_tree"},
			params: &types.Params{"icon": "https://example.com/logo.png"},
			want:   `{"text":"this is a message","icon_url":"https://example.com/logo.png"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := CreateJSONPayload(&tt.config, "this is a message", tt.params)
			require.NoError(t, err)

			assert.JSONEq(t, tt.want, string(payload))
		})
	}
}
