package generate

import (
	"net/url"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_loadArgsFromAltSources tests the loadArgsFromAltSources function with various scenarios.
func Test_loadArgsFromAltSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		wantService  string
		wantGen      string
		wantNoChange bool
	}{
		{
			name:        "positional service only",
			args:        []string{"discord"},
			wantService: "discord",
			wantGen:     "basic",
		},
		{
			name:        "positional both args",
			args:        []string{"discord", "basic"},
			wantService: "discord",
			wantGen:     "basic",
		},
		{
			name:         "no positional args",
			args:         []string{},
			wantNoChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a cobra command with the expected flags
			cmd := &cobra.Command{
				Use: "generate",
			}
			cmd.Flags().StringP("service", "s", "", "Notification service")
			cmd.Flags().StringP("generator", "g", "basic", "Generator to use")

			loadArgsFromAltSources(cmd, tt.args)

			if tt.wantNoChange {
				// Verify flags remain at their defaults/empty values
				service, _ := cmd.Flags().GetString("service")
				assert.Empty(t, service, "service flag should remain empty")
			} else {
				// Verify the flags were set correctly
				service, _ := cmd.Flags().GetString("service")
				gen, _ := cmd.Flags().GetString("generator")

				assert.Equal(t, tt.wantService, service, "service flag mismatch")
				assert.Equal(t, tt.wantGen, gen, "generator flag mismatch")
			}
		})
	}
}

// Test_maskSensitiveURL tests the maskSensitiveURL function with various service schemas.
func Test_maskSensitiveURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		serviceSchema string
		urlStr        string
		want          string
	}{
		{
			name:          "discord service masks user",
			serviceSchema: "discord",
			urlStr:        "https://token@discord.com/api/webhooks/123",
			want:          "https://REDACTED@discord.com/api/webhooks/123",
		},
		{
			name:          "slack service masks user",
			serviceSchema: "slack",
			urlStr:        "https://token@slack.com/services/123",
			want:          "https://REDACTED@slack.com/services/123",
		},
		{
			name:          "teams service masks user",
			serviceSchema: "teams",
			urlStr:        "https://token@teams.com/webhook",
			want:          "https://REDACTED@teams.com/webhook",
		},
		{
			name:          "smtp service masks password",
			serviceSchema: "smtp",
			urlStr:        "smtp://user:pass@host:587",
			want:          "smtp://user:REDACTED@host:587",
		},
		{
			name:          "pushover service masks token and user",
			serviceSchema: "pushover",
			urlStr:        "pushover://pushover.net?token=abc&user=def",
			want:          "pushover://pushover.net?token=REDACTED&user=REDACTED",
		},
		{
			name:          "gotify service masks token",
			serviceSchema: "gotify",
			urlStr:        "gotify://gotify.net?token=secret",
			want:          "gotify://gotify.net?token=REDACTED",
		},
		{
			name:          "generic service masks user and query params",
			serviceSchema: "unknown",
			urlStr:        "https://user:pass@host?a=1&b=2",
			want:          "https://REDACTED@host?a=REDACTED&b=REDACTED",
		},
		{
			name:          "invalid URL returns original",
			serviceSchema: "discord",
			urlStr:        "://invalid",
			want:          "://invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := maskSensitiveURL(tt.serviceSchema, tt.urlStr)
			assert.Equal(t, tt.want, got, "maskSensitiveURL() returned unexpected value")
		})
	}
}

// Test_maskUser tests the maskUser function.
func Test_maskUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		urlStr      string
		placeholder string
		wantUser    string
	}{
		{
			name:        "masks user with placeholder",
			urlStr:      "https://token@host/path",
			placeholder: "REDACTED",
			wantUser:    "REDACTED",
		},
		{
			name:        "empty user remains nil",
			urlStr:      "https://host/path",
			placeholder: "REDACTED",
			wantUser:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.urlStr)
			require.NoError(t, err, "failed to parse URL")

			maskUser(parsedURL, tt.placeholder)

			if tt.wantUser == "" {
				assert.Nil(t, parsedURL.User, "user should be nil")
			} else {
				assert.Equal(t, tt.wantUser, parsedURL.User.Username(), "user mismatch")
			}
		})
	}
}

// Test_maskSMTPUser tests the maskSMTPUser function.
func Test_maskSMTPUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		urlStr   string
		wantUser string
		wantPass string
	}{
		{
			name:     "masks password preserving username",
			urlStr:   "smtp://user:pass@host:587",
			wantUser: "user",
			wantPass: "REDACTED",
		},
		{
			name:     "user without password gets redacted password",
			urlStr:   "smtp://user@host:587",
			wantUser: "user",
			wantPass: "REDACTED",
		},
		{
			name:     "nil user remains nil",
			urlStr:   "smtp://host:587",
			wantUser: "",
			wantPass: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.urlStr)
			require.NoError(t, err, "failed to parse URL")

			maskSMTPUser(parsedURL)

			if tt.wantUser == "" {
				assert.Nil(t, parsedURL.User, "user should be nil")
			} else {
				assert.Equal(t, tt.wantUser, parsedURL.User.Username(), "username mismatch")

				pass, hasPass := parsedURL.User.Password()
				if tt.wantPass != "" {
					assert.True(t, hasPass, "password should be set")
					assert.Equal(t, tt.wantPass, pass, "password mismatch")
				}
			}
		})
	}
}

// Test_maskPushoverQuery tests the maskPushoverQuery function.
func Test_maskPushoverQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		urlStr   string
		wantTok  string
		wantUser string
	}{
		{
			name:     "masks token and user",
			urlStr:   "pushover://?token=abc&user=def",
			wantTok:  "REDACTED",
			wantUser: "REDACTED",
		},
		{
			name:     "masks only token",
			urlStr:   "pushover://?token=abc",
			wantTok:  "REDACTED",
			wantUser: "",
		},
		{
			name:     "leaves other params unchanged",
			urlStr:   "pushover://?other=value",
			wantTok:  "",
			wantUser: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.urlStr)
			require.NoError(t, err, "failed to parse URL")

			maskPushoverQuery(parsedURL)

			query := parsedURL.Query()
			if tt.wantTok != "" {
				assert.Equal(t, tt.wantTok, query.Get("token"), "token mismatch")
			}

			if tt.wantUser != "" {
				assert.Equal(t, tt.wantUser, query.Get("user"), "user mismatch")
			}

			if tt.wantTok == "" && tt.wantUser == "" {
				// Verify other params are unchanged
				assert.Equal(t, "value", query.Get("other"), "other param should be unchanged")
			}
		})
	}
}

// Test_maskGotifyQuery tests the maskGotifyQuery function.
func Test_maskGotifyQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		urlStr  string
		wantTok string
	}{
		{
			name:    "masks token",
			urlStr:  "gotify://?token=secret",
			wantTok: "REDACTED",
		},
		{
			name:    "leaves other params unchanged",
			urlStr:  "gotify://?other=value",
			wantTok: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.urlStr)
			require.NoError(t, err, "failed to parse URL")

			maskGotifyQuery(parsedURL)

			query := parsedURL.Query()
			if tt.wantTok != "" {
				assert.Equal(t, tt.wantTok, query.Get("token"), "token mismatch")
			} else {
				// Verify other params are unchanged
				assert.Equal(t, "value", query.Get("other"), "other param should be unchanged")
			}
		})
	}
}

// Test_maskGeneric tests the maskGeneric function.
func Test_maskGeneric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		urlStr       string
		wantUser     string
		wantParams   map[string]string
		wantQueryLen int
	}{
		{
			name:     "masks user and all query params",
			urlStr:   "https://user:pass@host?a=1&b=2",
			wantUser: "REDACTED",
			wantParams: map[string]string{
				"a": "REDACTED",
				"b": "REDACTED",
			},
			wantQueryLen: 2,
		},
		{
			name:     "masks only user when no query params",
			urlStr:   "https://user:pass@host",
			wantUser: "REDACTED",
		},
		{
			name:         "nil user with query params",
			urlStr:       "https://host?a=1",
			wantUser:     "",
			wantParams:   map[string]string{"a": "REDACTED"},
			wantQueryLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.urlStr)
			require.NoError(t, err, "failed to parse URL")

			maskGeneric(parsedURL)

			if tt.wantUser != "" {
				assert.Equal(t, tt.wantUser, parsedURL.User.Username(), "user mismatch")
			} else if parsedURL.User != nil {
				t.Errorf("expected nil user, got %v", parsedURL.User)
			}

			query := parsedURL.Query()
			if tt.wantParams != nil {
				assert.Len(t, query, tt.wantQueryLen, "query param count mismatch")

				for key, wantVal := range tt.wantParams {
					assert.Equal(t, wantVal, query.Get(key), "param %s mismatch", key)
				}
			}
		})
	}
}
