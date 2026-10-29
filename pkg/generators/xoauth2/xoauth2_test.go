package xoauth2

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net"
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

// syncBuffer is an output buffer that a running flow writes to while the test
// reads it.
type syncBuffer struct {
	// buf holds the output.
	buf strings.Builder
	// mu guards buf.
	mu sync.Mutex
}

// flowRun is a flow running in the background, with input the test writes line
// by line and output the test reads.
type flowRun struct {
	// input is the write end of the flow's input.
	input *io.PipeWriter
	// output is the flow's output.
	output *syncBuffer
	// done receives the flow's result.
	done chan flowResult
}

// flowResult is the result of a flow.
type flowResult struct {
	// err is the flow's error.
	err error
	// config is the generated configuration.
	config *smtp.Config
}

// Values shared by the tests.
const (
	// testAuthURL is the authorization endpoint the tests configure.
	testAuthURL = "https://auth.example.com/authorize"
	// testCode is the authorization code the browser receives.
	testCode = "the-code"
	// testSender is the sender the user enters.
	testSender = "sender@example.com"
	// refreshBody is a token response with a refresh token.
	refreshBody = `{"access_token":"access","token_type":"Bearer","expires_in":3599,"refresh_token":"refresh"}`
	// flowTimeout bounds each wait on a running flow.
	flowTimeout = 10 * time.Second
)

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

// String returns the output so far.
//
// Returns:
//   - string: the output.
func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// Write appends p to the buffer.
//
// Parameters:
//   - p: the bytes to append.
//
// Returns:
//   - int: the number of bytes written.
//   - error: always nil.
func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

// authRequest waits for the authorization URL in the flow's output and returns
// its query.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - url.Values: the authorization URL's query.
func (f *flowRun) authRequest(t *testing.T) url.Values {
	t.Helper()

	output := f.waitFor(t, testAuthURL+"?")

	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, testAuthURL+"?") {
			authURL, err := url.Parse(line)
			require.NoError(t, err)

			return authURL.Query()
		}
	}

	require.FailNow(t, "no authorization URL in the output", output)

	return nil
}

// result waits for the flow to end.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - flowResult: the flow's result.
func (f *flowRun) result(t *testing.T) flowResult {
	t.Helper()

	select {
	case result := <-f.done:
		return result
	case <-time.After(flowTimeout):
		require.FailNow(t, "the flow did not end", f.output.String())

		return flowResult{err: nil, config: nil}
	}
}

// send writes lines to the flow's input. The write runs in the background,
// because the pipe blocks until the flow reads.
//
// Parameters:
//   - lines: the text to write, which may hold several lines.
func (f *flowRun) send(lines string) {
	go func() { _, _ = io.WriteString(f.input, lines+"\n") }()
}

// waitFor waits until the flow's output contains text.
//
// Parameters:
//   - t: the test.
//   - text: the text to wait for.
//
// Returns:
//   - string: the output at that point.
func (f *flowRun) waitFor(t *testing.T, text string) string {
	t.Helper()

	require.Eventually(t, func() bool {
		return strings.Contains(f.output.String(), text)
	}, flowTimeout, 5*time.Millisecond, "the output never contained %q:\n%s", text, f.output.String())

	return f.output.String()
}

// startFlow runs run in the background with a pipe as its input.
//
// Parameters:
//   - t: the test that owns the flow.
//   - run: the flow, given its input and output.
//
// Returns:
//   - *flowRun: the running flow.
func startFlow(t *testing.T, run func(input io.Reader, output io.Writer) (*smtp.Config, error)) *flowRun {
	t.Helper()

	reader, writer := io.Pipe()
	flow := &flowRun{input: writer, output: &syncBuffer{}, done: make(chan flowResult, 1)}

	t.Cleanup(func() { _ = writer.Close() })

	go func() {
		config, err := run(reader, flow.output)
		flow.done <- flowResult{err: err, config: config}
	}()

	return flow
}

// startRunOAuth2Flow runs runOAuth2Flow in the background with conf.
//
// Parameters:
//   - t: the test that owns the flow.
//   - ctx: bounds the flow.
//   - conf: the OAuth2 settings.
//
// Returns:
//   - *flowRun: the running flow.
func startRunOAuth2Flow(t *testing.T, ctx context.Context, conf *oauth2.Config) *flowRun {
	t.Helper()

	return startFlow(t, func(input io.Reader, output io.Writer) (*smtp.Config, error) {
		lines := newLineReader(input)
		defer lines.stop()

		return runOAuth2Flow(ctx, conf, "smtp.example.com", lines, output)
	})
}

// startGenerateFlow runs [Generator.Generate] in the background with props
// and args.
//
// Parameters:
//   - t: the test that owns the flow.
//   - props: the generator props, such as the provider.
//   - args: the generator arguments, such as a settings file.
//
// Returns:
//   - *flowRun: the running flow.
func startGenerateFlow(t *testing.T, props map[string]string, args []string) *flowRun {
	t.Helper()

	return startFlow(t, func(input io.Reader, output io.Writer) (*smtp.Config, error) {
		config, err := (&Generator{Input: input, Output: output}).Generate(nil, props, args)
		if err != nil {
			return nil, err
		}

		smtpConfig, ok := config.(*smtp.Config)
		require.True(t, ok)

		return smtpConfig, nil
	})
}

// newTestConfig returns OAuth2 settings for the fake endpoints with redirectURL.
//
// Parameters:
//   - tokenURL: the fake token endpoint's URL.
//   - redirectURL: the redirect URL.
//
// Returns:
//   - *oauth2.Config: the settings.
func newTestConfig(tokenURL, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Endpoint: oauth2.Endpoint{
			AuthURL:   testAuthURL,
			TokenURL:  tokenURL,
			AuthStyle: oauth2.AuthStyleInParams,
		},
		RedirectURL: redirectURL,
		Scopes:      []string{"https://mail.google.com/"},
	}
}

// newTokenServer starts a fake token endpoint that answers with status and
// body.
//
// Parameters:
//   - t: the test that owns the server.
//   - status: the response status.
//   - body: the response body.
//
// Returns:
//   - *tokenEndpoint: the endpoint.
//   - string: the endpoint's URL.
func newTokenServer(t *testing.T, status int, body string) (*tokenEndpoint, string) {
	t.Helper()

	endpoint := &tokenEndpoint{status: status, body: body}
	server := httptest.NewServer(endpoint)
	t.Cleanup(server.Close)

	return endpoint, server.URL + "/token"
}

// browse requests a redirect address the way a browser does after the user
// authorized.
//
// Parameters:
//   - t: the test.
//   - redirect: the redirect URL from the authorization request.
//   - query: the query the provider adds.
//
// Returns:
//   - int: the response status.
//   - string: the response body.
func browse(t *testing.T, redirect string, query url.Values) (int, string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, redirect+"?"+query.Encode(), http.NoBody)
	require.NoError(t, err)

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	return res.StatusCode, string(body)
}

// requirePKCE checks that the token request sent the verifier whose S256
// challenge the authorization request carried.
//
// Parameters:
//   - t: the test.
//   - auth: the authorization request's query.
//   - form: the token request's form.
func requirePKCE(t *testing.T, auth, form url.Values) {
	t.Helper()

	require.Equal(t, "S256", auth.Get("code_challenge_method"))

	verifier := form.Get("code_verifier")
	require.NotEmpty(t, verifier, "the token request must send the code verifier")

	sum := sha256.Sum256([]byte(verifier))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), auth.Get("code_challenge"))
}

// redirectQuery returns the query a provider adds to the redirect.
//
// Parameters:
//   - state: the state from the authorization request.
//   - pairs: further keys and values.
//
// Returns:
//   - url.Values: the query.
func redirectQuery(state string, pairs ...string) url.Values {
	query := url.Values{"state": {state}}
	for i := 0; i+1 < len(pairs); i += 2 {
		query.Set(pairs[i], pairs[i+1])
	}

	return query
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

// TestGenerator_Generate checks how Generate picks a generator from the
// provider and arguments, and the errors each path reports.
func TestGenerator_Generate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		props     map[string]string
		args      []string
		errMsg    string
		wantErrIs []error
	}{
		{
			name:      "gmail provider with credentials file",
			props:     map[string]string{"provider": "gmail"},
			args:      []string{"/nonexistent/credentials.json"},
			errMsg:    "failed to read file",
			wantErrIs: []error{ErrReadFileFailed, os.ErrNotExist},
		},
		{
			name:      "gmail provider without args",
			props:     map[string]string{"provider": "gmail"},
			args:      []string{},
			errMsg:    "credentials file",
			wantErrIs: []error{ErrCredentialsFileRequired},
		},
		{
			name:   "non-gmail provider with file arg",
			props:  map[string]string{"provider": "other"},
			args:   []string{"/nonexistent/config.json"},
			errMsg: "failed to read file",
		},
		{
			name:   "no provider with file arg",
			props:  map[string]string{},
			args:   []string{"/nonexistent/config.json"},
			errMsg: "failed to read file",
		},
		{
			name:      "interactive generator no args fails at scan",
			props:     map[string]string{},
			args:      []string{},
			errMsg:    "failed to scan input",
			wantErrIs: []error{ErrScanFailed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := &Generator{Input: strings.NewReader(""), Output: io.Discard}

			got, err := g.Generate(nil, tt.props, tt.args)

			require.Error(t, err)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), tt.errMsg)

			for _, target := range tt.wantErrIs {
				require.ErrorIs(t, err, target)
			}
		})
	}
}

// Test_runOAuth2Flow runs the paste flow, used for a redirect that is not on
// the loopback interface, against a local token endpoint. It checks what the
// SMTP configuration stores, what the user sees, and that the token request
// carries the PKCE verifier.
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
			input:  testCode + "\n" + testSender + "\n",
			status: http.StatusOK,
			body:   refreshBody,
			opts:   []oauth2.AuthCodeOption{oauth2.AccessTypeOffline, oauth2.ApprovalForce},
			check: func(t *testing.T, config *smtp.Config, output, tokenURL string) {
				t.Helper()

				assert.Empty(t, config.Password)
				assert.Equal(t, "client-id", config.ClientID)
				assert.Equal(t, "client-secret", config.ClientSecret)
				assert.Equal(t, "refresh", config.RefreshToken)
				assert.Equal(t, tokenURL, config.TokenURL)
				assert.Equal(t, smtp.AuthTypes.OAuth2, config.Auth)
				assert.Equal(t, testSender, config.Username)
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
			input:  testCode + "\n" + testSender + "\n",
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
			input:     testCode + "\n" + testSender + "\n",
			status:    http.StatusBadRequest,
			body:      `{"error":"invalid_grant"}`,
			wantErrIs: []error{ErrTokenExchangeFailed},
		},
		{
			name:      "no verification code",
			host:      "smtp.example.com",
			status:    http.StatusOK,
			body:      refreshBody,
			wantErrIs: []error{ErrScanFailed},
		},
		{
			name:      "no sender",
			host:      "smtp.example.com",
			input:     testCode + "\n",
			status:    http.StatusOK,
			body:      refreshBody,
			wantErrIs: []error{ErrScanFailed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			endpoint, tokenURL := newTokenServer(t, tt.status, tt.body)
			conf := newTestConfig(tokenURL, "urn:ietf:wg:oauth:2.0:oob")

			var output strings.Builder

			lines := newLineReader(strings.NewReader(tt.input))
			defer lines.stop()

			config, err := runOAuth2Flow(t.Context(), conf, tt.host, lines, &output, tt.opts...)

			if len(tt.wantErrIs) > 0 {
				require.Error(t, err)
				assert.Nil(t, config)
				assert.NotContains(t, err.Error(), testCode)

				for _, target := range tt.wantErrIs {
					require.ErrorIs(t, err, target)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, config)

			form := endpoint.lastForm()
			assert.Equal(t, testCode, form.Get("code"))
			assert.Equal(t, "urn:ietf:wg:oauth:2.0:oob", form.Get("redirect_uri"))

			authURL, err := url.Parse(strings.Split(strings.TrimPrefix(output.String(), "Visit the following URL to authorize Shoutrrr:\n"), "\n")[0])
			require.NoError(t, err)
			requirePKCE(t, authURL.Query(), form)

			tt.check(t, config, output.String(), conf.Endpoint.TokenURL)
		})
	}
}

// TestPastedRedirectAddress verifies that the paste flow reads a pasted
// address the browser was sent to: the code when its state matches, and an
// error for another state, a provider error, or a missing code.
func TestPastedRedirectAddress(t *testing.T) {
	t.Parallel()

	// tests lists the extra query of the pasted address, whether its state is
	// the flow's, and the error expected.
	tests := []struct {
		wantErr   error
		name      string
		pairs     []string
		ownState  bool
		wantPaste bool
	}{
		{name: "code with the flow's state", pairs: []string{"code", testCode}, ownState: true},
		{name: "code with another state", pairs: []string{"code", testCode}, wantErr: ErrStateMismatch},
		{name: "provider error", pairs: []string{"error", "access_denied"}, ownState: true, wantErr: ErrAuthorizationDenied},
		{name: "no code", ownState: true, wantErr: ErrNoAuthorizationCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			endpoint, tokenURL := newTokenServer(t, http.StatusOK, refreshBody)
			flow := startRunOAuth2Flow(t, t.Context(), newTestConfig(tokenURL, "https://app.example.com/callback"))

			state := flow.authRequest(t).Get("state")
			if !tt.ownState {
				state = "another-state"
			}

			flow.send("https://app.example.com/callback?" + redirectQuery(state, tt.pairs...).Encode())

			if tt.wantErr != nil {
				result := flow.result(t)
				require.ErrorIs(t, result.err, tt.wantErr)
				assert.Nil(t, endpoint.lastForm(), "no code may be exchanged")

				return
			}

			flow.waitFor(t, "Enter sender e-mail")
			flow.send(testSender)

			result := flow.result(t)
			require.NoError(t, result.err)
			assert.Equal(t, testCode, endpoint.lastForm().Get("code"))
		})
	}
}

// TestLoopbackRedirect verifies the loopback flow.
//   - A local server receives the browser's redirect on a free port.
//   - It refuses a request with another state or for another path without
//     ending the flow.
//   - It shows a page for the redirect, and the sender prompt starts on its
//     own line.
//   - The token request carries the redirect and the PKCE verifier.
func TestLoopbackRedirect(t *testing.T) {
	t.Parallel()

	// redirects lists the configured loopback redirects with the host the
	// redirect keeps.
	redirects := []struct {
		name     string
		redirect string
		wantHost string
	}{
		{name: "127.0.0.1 without a port", redirect: "http://127.0.0.1", wantHost: "127.0.0.1"},
		{name: "localhost with a path", redirect: "http://localhost/callback", wantHost: "localhost"},
		{name: "localhost in capitals", redirect: "http://LOCALHOST/callback", wantHost: "LOCALHOST"},
	}

	for _, tt := range redirects {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			endpoint, tokenURL := newTokenServer(t, http.StatusOK, refreshBody)
			flow := startRunOAuth2Flow(t, t.Context(), newTestConfig(tokenURL, tt.redirect))

			auth := flow.authRequest(t)
			redirect, err := url.Parse(auth.Get("redirect_uri"))
			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, redirect.Hostname())
			assert.NotEmpty(t, redirect.Port(), "the redirect must name the server's port")

			status, _ := browse(t, auth.Get("redirect_uri"), redirectQuery("another-state", "code", "forged"))
			assert.Equal(t, http.StatusBadRequest, status)

			other := *redirect
			other.Path = "/other"
			status, _ = browse(t, other.String(), redirectQuery(auth.Get("state"), "code", "forged"))
			assert.Equal(t, http.StatusNotFound, status, "a request for another path must not end the flow")

			status, body := browse(t, auth.Get("redirect_uri"), redirectQuery(auth.Get("state"), "code", testCode))
			assert.Equal(t, http.StatusOK, status)
			assert.Contains(t, body, "Authorization complete")

			output := flow.waitFor(t, "Enter sender e-mail")
			assert.Contains(t, output, "the code it shows: \nEnter sender e-mail", "the sender prompt must start on its own line")
			flow.send(testSender)

			result := flow.result(t)
			require.NoError(t, result.err)
			assert.Equal(t, "refresh", result.config.RefreshToken)

			form := endpoint.lastForm()
			assert.Equal(t, testCode, form.Get("code"))
			assert.Equal(t, auth.Get("redirect_uri"), form.Get("redirect_uri"))
			requirePKCE(t, auth, form)
		})
	}
}

// TestLoopbackRedirectOutcomes verifies the other ways a loopback flow ends.
//   - A provider error ends it with [ErrAuthorizationDenied].
//   - A pasted address or code completes it without the browser.
//   - Input that ends fails it with [ErrScanFailed] before any exchange.
//   - An empty pasted line keeps the wait.
//   - A canceled context ends the wait.
func TestLoopbackRedirectOutcomes(t *testing.T) {
	t.Parallel()

	t.Run("provider error", func(t *testing.T) {
		t.Parallel()

		endpoint, tokenURL := newTokenServer(t, http.StatusOK, refreshBody)
		flow := startRunOAuth2Flow(t, t.Context(), newTestConfig(tokenURL, "http://127.0.0.1"))

		auth := flow.authRequest(t)
		status, body := browse(t, auth.Get("redirect_uri"), redirectQuery(auth.Get("state"), "error", "access_denied"))
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, "not granted")

		result := flow.result(t)
		require.ErrorIs(t, result.err, ErrAuthorizationDenied)
		assert.Contains(t, result.err.Error(), "access_denied")
		assert.Nil(t, endpoint.lastForm())
	})

	// pastes lists what the user pastes instead of the browser returning.
	pastes := []struct {
		paste func(redirect, state string) string
		name  string
	}{
		{name: "pasted address", paste: func(redirect, state string) string {
			return redirect + "?" + redirectQuery(state, "code", testCode).Encode()
		}},
		{name: "pasted code", paste: func(string, string) string { return testCode }},
	}

	for _, tt := range pastes {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			endpoint, tokenURL := newTokenServer(t, http.StatusOK, refreshBody)
			flow := startRunOAuth2Flow(t, t.Context(), newTestConfig(tokenURL, "http://127.0.0.1"))

			auth := flow.authRequest(t)
			flow.send(tt.paste(auth.Get("redirect_uri"), auth.Get("state")))
			flow.waitFor(t, "Enter sender e-mail")
			flow.send(testSender)

			result := flow.result(t)
			require.NoError(t, result.err)
			assert.Equal(t, testCode, endpoint.lastForm().Get("code"))
			assert.Equal(t, auth.Get("redirect_uri"), endpoint.lastForm().Get("redirect_uri"))
		})
	}

	t.Run("input ends before the browser returns", func(t *testing.T) {
		t.Parallel()

		endpoint, tokenURL := newTokenServer(t, http.StatusOK, refreshBody)
		flow := startRunOAuth2Flow(t, t.Context(), newTestConfig(tokenURL, "http://127.0.0.1"))

		flow.authRequest(t)
		require.NoError(t, flow.input.Close())

		result := flow.result(t)
		require.ErrorIs(t, result.err, ErrScanFailed)
		assert.Contains(t, result.err.Error(), "verification code")
		assert.Nil(t, endpoint.lastForm(), "no code may be exchanged without input for the sender")
	})

	t.Run("empty line keeps the wait", func(t *testing.T) {
		t.Parallel()

		endpoint, tokenURL := newTokenServer(t, http.StatusOK, refreshBody)
		flow := startRunOAuth2Flow(t, t.Context(), newTestConfig(tokenURL, "http://127.0.0.1"))

		flow.authRequest(t)
		flow.send("  \n" + testCode)
		flow.waitFor(t, "Enter sender e-mail")
		flow.send(testSender)

		result := flow.result(t)
		require.NoError(t, result.err)
		assert.Equal(t, testCode, endpoint.lastForm().Get("code"), "the empty line must not be taken as the code")
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		_, tokenURL := newTokenServer(t, http.StatusOK, refreshBody)
		flow := startRunOAuth2Flow(t, ctx, newTestConfig(tokenURL, "http://127.0.0.1"))

		flow.authRequest(t)
		cancel()

		require.ErrorIs(t, flow.result(t).err, context.Canceled)
	})
}

// TestLoopbackListenFailure verifies that a loopback redirect whose port is
// taken falls back to the paste flow. The user is told why, the configured
// redirect is kept, and a pasted address with the flow's state completes it.
func TestLoopbackListenFailure(t *testing.T) {
	t.Parallel()

	var listenConfig net.ListenConfig

	taken, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { _ = taken.Close() })

	redirect := "http://" + taken.Addr().String() + "/callback"
	endpoint, tokenURL := newTokenServer(t, http.StatusOK, refreshBody)
	flow := startRunOAuth2Flow(t, t.Context(), newTestConfig(tokenURL, redirect))

	auth := flow.authRequest(t)
	output := flow.waitFor(t, "Enter the verification code")
	assert.Contains(t, output, "Cannot receive the browser's redirect")
	assert.Equal(t, redirect, auth.Get("redirect_uri"))

	flow.send(redirect + "?" + redirectQuery(auth.Get("state"), "code", testCode).Encode())
	flow.waitFor(t, "Enter sender e-mail")
	flow.send(testSender)

	result := flow.result(t)
	require.NoError(t, result.err)
	assert.Equal(t, testCode, endpoint.lastForm().Get("code"))
	assert.Equal(t, redirect, endpoint.lastForm().Get("redirect_uri"))
	requirePKCE(t, auth, endpoint.lastForm())
}

// TestGenerateGmailLoopback verifies that the gmail provider replaces the
// out-of-band redirect, which Google rejects, with a loopback redirect on
// 127.0.0.1. The flow then completes through the browser's redirect.
func TestGenerateGmailLoopback(t *testing.T) {
	t.Parallel()

	endpoint, tokenURL := newTokenServer(t, http.StatusOK, refreshBody)
	credentials := createTempFile(t, `{"installed":{"client_id":"client-id","client_secret":"client-secret",`+
		`"auth_uri":"`+testAuthURL+`","token_uri":"`+tokenURL+`",`+
		`"redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`)

	flow := startGenerateFlow(t, map[string]string{"provider": "gmail"}, []string{credentials})

	auth := flow.authRequest(t)
	assert.True(t, strings.HasPrefix(auth.Get("redirect_uri"), "http://127.0.0.1:"), auth.Get("redirect_uri"))
	assert.Equal(t, "offline", auth.Get("access_type"))

	status, _ := browse(t, auth.Get("redirect_uri"), redirectQuery(auth.Get("state"), "code", testCode))
	assert.Equal(t, http.StatusOK, status)

	flow.waitFor(t, "Enter sender e-mail")
	flow.send(testSender)

	result := flow.result(t)
	require.NoError(t, result.err)
	assert.Equal(t, "smtp.gmail.com", result.config.Host)
	assert.Equal(t, "refresh", result.config.RefreshToken)
	requirePKCE(t, auth, endpoint.lastForm())
}

// TestGenerateFileNamesInAnyCase verifies that a provider file whose member
// names differ in case from the documented names still configures the flow,
// because users write the file themselves.
func TestGenerateFileNamesInAnyCase(t *testing.T) {
	t.Parallel()

	endpoint, tokenURL := newTokenServer(t, http.StatusOK, refreshBody)
	file := createTempFile(t, `{"Client_ID":"client-id","CLIENT_SECRET":"client-secret",`+
		`"Redirect_URL":"http://127.0.0.1","Auth_URL":"`+testAuthURL+`","TOKEN_URL":"`+tokenURL+`",`+
		`"SMTP_Hostname":"smtp.example.com","Scopes":["mail"]}`)

	flow := startGenerateFlow(t, map[string]string{}, []string{file})

	auth := flow.authRequest(t)
	assert.Equal(t, "client-id", auth.Get("client_id"))
	assert.Equal(t, "mail", auth.Get("scope"))

	status, _ := browse(t, auth.Get("redirect_uri"), redirectQuery(auth.Get("state"), "code", testCode))
	assert.Equal(t, http.StatusOK, status)

	flow.waitFor(t, "Enter sender e-mail")
	flow.send(testSender)

	result := flow.result(t)
	require.NoError(t, result.err)
	assert.Equal(t, "smtp.example.com", result.config.Host)
	assert.Equal(t, "client-secret", result.config.ClientSecret)
	requirePKCE(t, auth, endpoint.lastForm())
}

// TestGenerateInteractive verifies the interactive generator end to end with
// every answer written at once, so the answers after the settings, the pasted
// code and the sender, must not be lost to an earlier read.
func TestGenerateInteractive(t *testing.T) {
	t.Parallel()

	endpoint, tokenURL := newTokenServer(t, http.StatusOK, refreshBody)

	flow := startGenerateFlow(t, map[string]string{}, nil)

	flow.send(strings.Join([]string{
		"client-id", "client-secret", testAuthURL, tokenURL,
		"urn:ietf:wg:oauth:2.0:oob", "https://mail.google.com/", "smtp.example.com",
		testCode, testSender,
	}, "\n"))

	result := flow.result(t)
	require.NoError(t, result.err)
	assert.Equal(t, "smtp.example.com", result.config.Host)
	assert.Equal(t, testSender, result.config.Username)
	assert.Equal(t, testCode, endpoint.lastForm().Get("code"))
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
		errType error
		name    string
		file    string
		errMsg  string
	}{
		{name: "nonexistent file", file: "/nonexistent/path/config.json", errType: ErrReadFileFailed, errMsg: "failed to read file"},
		{name: "invalid JSON file", file: createTempFile(t, "invalid json content"), errType: ErrUnmarshalFailed, errMsg: "failed to unmarshal JSON"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lines := newLineReader(strings.NewReader(""))
			defer lines.stop()

			got, err := oauth2GeneratorFile(t.Context(), tt.file, lines, io.Discard)

			require.ErrorIs(t, err, tt.errType)
			assert.Contains(t, err.Error(), tt.errMsg)
			assert.Nil(t, got)
		})
	}
}

// Test_oauth2GeneratorGmail checks the errors for a Google credentials file that
// cannot be read or parsed.
func Test_oauth2GeneratorGmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		errType error
		name    string
		file    string
		errMsg  string
	}{
		{name: "nonexistent credentials file", file: "/nonexistent/path/credentials.json", errType: ErrReadFileFailed, errMsg: "failed to read file"},
		{name: "invalid JSON credentials file", file: createTempFile(t, "invalid json content"), errMsg: "invalid character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lines := newLineReader(strings.NewReader(""))
			defer lines.stop()

			got, err := oauth2GeneratorGmail(t.Context(), tt.file, lines, io.Discard)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
			assert.Nil(t, got)

			if tt.errType != nil {
				require.ErrorIs(t, err, tt.errType)
			}
		})
	}
}
