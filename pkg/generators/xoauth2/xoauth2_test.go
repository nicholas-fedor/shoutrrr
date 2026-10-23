package xoauth2

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/email/smtp"
)

// tokenEndpoint is a fake OAuth2 token endpoint that answers every request with
// one status and body, and records the last request's form.
type tokenEndpoint struct {
	// form is the last token request's form.
	form url.Values
	// body is the JSON response body.
	body string
	// mu guards form.
	mu sync.Mutex
	// status is the response status.
	status int
}

// TestGenerator_Generate checks how Generate picks a generator from the
// provider and arguments, and the errors each path reports.
func TestGenerator_Generate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		props     map[string]string
		args      []string
		wantErr   bool
		errMsg    string
		wantErrIs []error
	}{
		{
			name:      "gmail provider with credentials file",
			props:     map[string]string{"provider": "gmail"},
			args:      []string{"/nonexistent/credentials.json"},
			wantErr:   true,
			errMsg:    "failed to read file",
			wantErrIs: []error{ErrReadFileFailed, os.ErrNotExist},
		},
		{
			name:      "gmail provider without args",
			props:     map[string]string{"provider": "gmail"},
			args:      []string{},
			wantErr:   true,
			errMsg:    "credentials file",
			wantErrIs: []error{ErrCredentialsFileRequired},
		},
		{
			name:    "non-gmail provider with file arg",
			props:   map[string]string{"provider": "other"},
			args:    []string{"/nonexistent/config.json"},
			wantErr: true,
			errMsg:  "failed to read file",
		},
		{
			name:    "no provider with file arg",
			props:   map[string]string{},
			args:    []string{"/nonexistent/config.json"},
			wantErr: true,
			errMsg:  "failed to read file",
		},
		{
			name:    "interactive generator no args fails at scan",
			props:   map[string]string{},
			args:    []string{},
			wantErr: true,
			errMsg:  "failed to scan input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := &Generator{}

			got, err := g.Generate(nil, tt.props, tt.args)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				for _, target := range tt.wantErrIs {
					require.ErrorIs(t, err, target)
				}

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

// ServeHTTP records the request form and writes the scripted response.
//
// Parameters:
//   - w: the response writer.
//   - r: the token request.
func (e *tokenEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()

	e.mu.Lock()
	e.form = r.PostForm
	e.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.status)
	_, _ = w.Write([]byte(e.body))
}

// lastForm returns the last token request's form.
//
// Returns:
//   - url.Values: the form, or nil before any request.
func (e *tokenEndpoint) lastForm() url.Values {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.form
}

// Test_runOAuth2Flow runs the whole terminal flow against a local token
// endpoint, checking what the SMTP configuration stores and what the user sees.
func Test_runOAuth2Flow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// host is the SMTP host passed to the flow.
		host string
		// input is what the user types.
		input string
		// status and body are the token endpoint's response.
		status int
		body   string
		// opts are the authorization URL options.
		opts []oauth2.AuthCodeOption
		// wantErrIs lists the errors the flow must wrap. Empty means success.
		wantErrIs []error
		// check verifies a successful flow's configuration and output.
		check func(t *testing.T, config *smtp.Config, output, tokenURL string)
	}{
		{
			name:   "refresh token stored without a password",
			host:   "smtp.gmail.com",
			input:  "the-code\nsender@example.com\n",
			status: http.StatusOK,
			body:   `{"access_token":"access","token_type":"Bearer","expires_in":3599,"refresh_token":"refresh"}`,
			opts:   []oauth2.AuthCodeOption{oauth2.AccessTypeOffline, oauth2.ApprovalForce},
			check: func(t *testing.T, config *smtp.Config, output, tokenURL string) {
				t.Helper()

				assert.Empty(t, config.Password)
				assert.Equal(t, "client-id", config.ClientID)
				assert.Equal(t, "client-secret", config.ClientSecret)
				assert.Equal(t, "refresh", config.RefreshToken)
				assert.Equal(t, tokenURL, config.TokenURL)
				assert.Equal(t, smtp.AuthTypes.OAuth2, config.Auth)
				assert.Equal(t, "sender@example.com", config.Username)
				assert.Equal(t, GmailSMTPPortStartTLS, config.Port)
				assert.Equal(t, "localhost", config.ClientHost)
				assert.Equal(t, smtp.EncMethods.Auto, config.Encryption)
				assert.Equal(t, 10*time.Second, config.Timeout)
				assert.Contains(t, output, "access_type=offline")
				assert.Contains(t, output, "prompt=consent")
				assert.NotContains(t, output, "Warning")
			},
		},
		{
			name:   "access token as the password without a refresh token",
			host:   "smtp.example.com",
			input:  "the-code\nsender@example.com\n",
			status: http.StatusOK,
			body:   `{"access_token":"access","token_type":"Bearer","expires_in":3599}`,
			check: func(t *testing.T, config *smtp.Config, output, _ string) {
				t.Helper()

				assert.Equal(t, "access", config.Password)
				assert.Empty(t, config.ClientID)
				assert.Empty(t, config.ClientSecret)
				assert.Empty(t, config.RefreshToken)
				assert.Empty(t, config.TokenURL)
				assert.Equal(t, DefaultSMTPPort, config.Port)
				assert.NotContains(t, output, "access_type=offline")
				assert.Contains(t, output, "Warning: the provider returned no refresh token")
				assert.Contains(t, output, " at ")
			},
		},
		{
			name:      "exchange error leaves out the code",
			host:      "smtp.example.com",
			input:     "the-code\nsender@example.com\n",
			status:    http.StatusBadRequest,
			body:      `{"error":"invalid_grant"}`,
			wantErrIs: []error{ErrTokenExchangeFailed},
		},
		{
			name:      "no verification code",
			host:      "smtp.example.com",
			status:    http.StatusOK,
			body:      `{"access_token":"access","token_type":"Bearer"}`,
			wantErrIs: []error{ErrScanFailed},
		},
		{
			name:      "no sender",
			host:      "smtp.example.com",
			input:     "the-code\n",
			status:    http.StatusOK,
			body:      `{"access_token":"access","token_type":"Bearer"}`,
			wantErrIs: []error{ErrScanFailed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			endpoint := &tokenEndpoint{status: tt.status, body: tt.body}
			server := httptest.NewServer(endpoint)
			t.Cleanup(server.Close)

			conf := &oauth2.Config{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://auth.example.com/authorize",
					TokenURL: server.URL + "/token",
				},
				RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
				Scopes:      []string{"https://mail.google.com/"},
			}

			var output strings.Builder

			config, err := runOAuth2Flow(t.Context(), conf, tt.host, strings.NewReader(tt.input), &output, tt.opts...)

			if len(tt.wantErrIs) > 0 {
				require.Error(t, err)
				assert.Nil(t, config)
				assert.NotContains(t, err.Error(), "the-code")

				for _, target := range tt.wantErrIs {
					require.ErrorIs(t, err, target)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, config)
			assert.Equal(t, "the-code", endpoint.lastForm().Get("code"))
			tt.check(t, config, output.String(), conf.Endpoint.TokenURL)
		})
	}
}

// Test_noRefreshTokenWarning checks that the warning names the expiry only
// when the provider sent one.
func Test_noRefreshTokenWarning(t *testing.T) {
	t.Parallel()

	expiry := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

	assert.Equal(t,
		"Warning: the provider returned no refresh token, so the URL works only until the access token expires"+
			" at 2026-10-23T12:00:00Z.",
		noRefreshTokenWarning(&oauth2.Token{Expiry: expiry}),
	)
	assert.Equal(t,
		"Warning: the provider returned no refresh token, so the URL works only until the access token expires.",
		noRefreshTokenWarning(&oauth2.Token{}),
	)
}

// Test_oauth2GeneratorFile checks the errors for a settings file that cannot be
// read or is not JSON.
func Test_oauth2GeneratorFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		wantErr bool
		errType error
		errMsg  string
	}{
		{
			name:    "nonexistent file",
			file:    "/nonexistent/path/config.json",
			wantErr: true,
			errType: ErrReadFileFailed,
			errMsg:  "failed to read file",
		},
		{
			name:    "invalid JSON file",
			file:    createTempFile(t, "invalid json content"),
			wantErr: true,
			errType: ErrUnmarshalFailed,
			errMsg:  "failed to unmarshal JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := oauth2GeneratorFile(tt.file)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

// Test_oauth2GeneratorGmail checks the errors for a Google credentials file that
// cannot be read or parsed.
func Test_oauth2GeneratorGmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		wantErr bool
		errType error
		errMsg  string
	}{
		{
			name:    "nonexistent credentials file",
			file:    "/nonexistent/path/credentials.json",
			wantErr: true,
			errType: ErrReadFileFailed,
			errMsg:  "failed to read file",
		},
		{
			name:    "invalid JSON credentials file",
			file:    createTempFile(t, "invalid json content"),
			wantErr: true,
			errMsg:  "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := oauth2GeneratorGmail(tt.file)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

// createTempFile creates a temporary file with the given content and returns its path.
func createTempFile(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-config.json")

	err := os.WriteFile(tmpFile, []byte(content), 0o600)
	require.NoError(t, err, "failed to create temp file")

	return tmpFile
}
