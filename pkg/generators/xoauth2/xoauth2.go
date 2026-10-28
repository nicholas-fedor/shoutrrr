// Package xoauth2 provides OAuth2 authentication for email services.
package xoauth2

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/email/smtp"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Generator is the XOAuth2 Generator implementation.
type Generator struct {
	// Input is the reader the answers are read from. Nil reads os.Stdin.
	Input io.Reader
	// Output is the writer the prompts go to. Nil writes to os.Stdout.
	Output io.Writer
}

// lineReader reads the input one line at a time in its own goroutine, so the
// flow can wait for a pasted line and for the browser's redirect at once. One
// lineReader serves every prompt of a generation, so no prompt loses lines
// that another read ahead.
type lineReader struct {
	// lines receives each input line, and is closed when the input ends.
	lines chan string
	// done is closed when the generation ends, so a pending send gives up.
	done chan struct{}
}

// authResult is the outcome of the authorization step.
type authResult struct {
	// err is the reason the authorization failed, or nil.
	err error
	// code is the authorization code, when err is nil.
	code string
}

// SMTP port constants.
const (
	// DefaultSMTPPort is the standard SMTP port without encryption.
	DefaultSMTPPort uint16 = 25
	// GmailSMTPPortStartTLS is the Gmail SMTP port with STARTTLS.
	GmailSMTPPortStartTLS uint16 = 587
)

// StateLength is the length in bytes for OAuth 2.0 state randomness (128 bits).
const StateLength int = 16

// SMTP settings the generated configuration shares with the SMTP service's
// defaults, so the generated URL works as the service would configure itself.
const (
	// smtpClientHost is the EHLO hostname.
	smtpClientHost = "localhost"
	// smtpTimeout is the session timeout.
	smtpTimeout = 10 * time.Second
)

// Settings of the local server that receives the browser's redirect.
const (
	// callbackReadHeaderTimeout bounds how long the server waits for a
	// request's headers.
	callbackReadHeaderTimeout = 10 * time.Second
	// callbackShutdownTimeout bounds how long the server waits for the
	// redirect's response to finish after the code arrived.
	callbackShutdownTimeout = 5 * time.Second
	// googleLoopbackRedirect is the redirect a Google credentials file is
	// given when it names the copy and paste (out-of-band) redirect, which
	// Google rejects. The port is chosen when the server starts.
	googleLoopbackRedirect = "http://127.0.0.1"
	// successPage is the page the browser shows after a successful redirect.
	successPage = "<!doctype html><title>Shoutrrr</title>" +
		"<p>Authorization complete. You can close this window and return to the terminal.</p>"
	// deniedPage is the page the browser shows when authorization was denied.
	deniedPage = "<!doctype html><title>Shoutrrr</title>" +
		"<p>Authorization was not granted. Return to the terminal for details.</p>"
)

// Errors returned while generating the configuration.
var (
	// ErrReadFileFailed indicates that a settings or credentials file could not
	// be read.
	ErrReadFileFailed = errors.New("failed to read file")
	// ErrUnmarshalFailed indicates that a settings file is not valid JSON.
	ErrUnmarshalFailed = errors.New("failed to unmarshal JSON")
	// ErrScanFailed indicates that input ended before an answer was given.
	ErrScanFailed = errors.New("failed to scan input")
	// ErrTokenExchangeFailed indicates that the provider rejected the
	// verification code or could not be reached.
	ErrTokenExchangeFailed = errors.New("failed to exchange token")
	// ErrCredentialsFileRequired indicates that the gmail provider was selected
	// without a credentials file argument.
	ErrCredentialsFileRequired = errors.New("the gmail provider needs a credentials file argument")
	// ErrAuthorizationDenied indicates that the provider redirected with an
	// error, such as access_denied when the user declined.
	ErrAuthorizationDenied = errors.New("authorization was not granted")
	// ErrStateMismatch indicates a pasted redirect address whose state is not
	// the one this flow sent, so it may come from another authorization.
	ErrStateMismatch = errors.New("the redirect address belongs to another authorization")
	// ErrNoAuthorizationCode indicates a pasted redirect address without a
	// code.
	ErrNoAuthorizationCode = errors.New("the redirect address has no authorization code")
)

// Generate generates a service URL from a set of user questions/answers.
//
// With the gmail provider, the first argument is the Google credentials file.
// Otherwise the first argument, when given, is a JSON file with the provider's
// OAuth2 settings, and without one the settings are asked for interactively.
//
// Parameters:
//   - props: the generator props, which may set the provider.
//   - args: the generator arguments, which may name a credentials or settings file.
//
// Returns:
//   - types.ServiceConfig: the SMTP configuration.
//   - error: [ErrCredentialsFileRequired], a file error wrapping [ErrReadFileFailed]
//     or [ErrUnmarshalFailed], or an input, authorization, or token exchange error.
func (g *Generator) Generate(
	_ types.Service,
	props map[string]string,
	args []string,
) (types.ServiceConfig, error) {
	input := g.Input
	if input == nil {
		input = os.Stdin
	}

	output := g.Output
	if output == nil {
		output = os.Stdout
	}

	gmail := props["provider"] == "gmail"
	if gmail && len(args) == 0 {
		return nil, ErrCredentialsFileRequired
	}

	lines := newLineReader(input)
	defer lines.stop()

	ctx := context.Background()

	if gmail {
		return oauth2GeneratorGmail(ctx, args[0], lines, output)
	}

	if len(args) > 0 {
		return oauth2GeneratorFile(ctx, args[0], lines, output)
	}

	return oauth2Generator(ctx, lines, output)
}

// next returns the next input line.
//
// Returns:
//   - string: the line, without its line ending.
//   - bool: false when the input has ended.
func (l *lineReader) next() (string, bool) {
	line, ok := <-l.lines

	return line, ok
}

// stop ends the generation's use of the reader. A read that is still pending
// on the input stays blocked until the input delivers a line or ends.
func (l *lineReader) stop() {
	close(l.done)
}

// newLineReader starts reading input one line at a time.
//
// Parameters:
//   - input: the reader the lines come from.
//
// Returns:
//   - *lineReader: the reader.
func newLineReader(input io.Reader) *lineReader {
	reader := &lineReader{lines: make(chan string), done: make(chan struct{})}

	go func() {
		defer close(reader.lines)

		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			select {
			case reader.lines <- scanner.Text():
			case <-reader.done:
				return
			}
		}
	}()

	return reader
}

// runOAuth2Flow asks the user to authorize Shoutrrr, exchanges the code for
// tokens, and generates the SMTP configuration.
//
// Every authorization uses PKCE with an S256 challenge and a random state. A
// local server receives a loopback redirect such as http://127.0.0.1 on that
// address. It picks a free port when the address names none. For a browser on
// another machine, the user can paste the address the browser was sent to.
// Pasting the code the browser shows works too. Any other redirect is pasted.
//
// The configuration holds the refresh token and the client settings, so the
// SMTP service can get a current access token for each send. A provider that
// returns no refresh token leaves only the access token, used as the password
// until it expires, and the user is warned.
//
// Parameters:
//   - ctx: bounds the wait for the redirect and the token exchange.
//   - conf: the provider's OAuth2 settings. It is not changed.
//   - host: the SMTP server host.
//   - lines: the user's answers: a pasted code or address, then the sender.
//   - output: receives the prompts and the warning.
//   - opts: extra parameters for the authorization URL.
//
// Returns:
//   - *smtp.Config: the SMTP configuration.
//   - error: an input error wrapping [ErrScanFailed], [ErrAuthorizationDenied],
//     [ErrStateMismatch], [ErrNoAuthorizationCode], a local server error, or an
//     error wrapping [ErrTokenExchangeFailed] and the provider's error.
func runOAuth2Flow(
	ctx context.Context,
	conf *oauth2.Config,
	host string,
	lines *lineReader,
	output io.Writer,
	opts ...oauth2.AuthCodeOption,
) (*smtp.Config, error) {
	stateBytes := make([]byte, StateLength)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("generating random state: %w", err)
	}

	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	verifier := oauth2.GenerateVerifier()
	opts = append(opts, oauth2.S256ChallengeOption(verifier))

	// The loopback flow sets the redirect's port, so it works on a copy.
	flowConf := *conf

	var (
		code string
		err  error
	)

	if redirect, ok := loopbackRedirect(flowConf.RedirectURL); ok {
		code, err = authorizeLoopback(ctx, &flowConf, redirect, state, lines, output, opts)
	} else {
		code, err = authorizePaste(&flowConf, state, lines, output, opts)
	}

	if err != nil {
		return nil, err
	}

	token, err := flowConf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		// The verification code is a credential, so the error leaves it out.
		return nil, fmt.Errorf("%w: %w", ErrTokenExchangeFailed, err)
	}

	sender, err := ask(lines, output, "Enter sender e-mail: ", "sender email")
	if err != nil {
		return nil, err
	}

	if token.RefreshToken == "" {
		if _, err := fmt.Fprintln(output, noRefreshTokenWarning(token)); err != nil {
			return nil, fmt.Errorf("writing to output: %w", err)
		}
	}

	return newSMTPConfig(&flowConf, token, host, sender), nil
}

// authorizeLoopback receives the authorization code on a local server at the
// loopback redirect, or from a pasted line, whichever comes first.
//
// The server answers a request for another path with 404 Not Found. It answers
// a request with another state with an error page. It does the same for a
// request without a code or an error. The server keeps waiting after each of
// these, so a stray or forged request cannot end the flow. Empty pasted lines
// are ignored. Input that ends fails the flow, because the sender's address is
// read from it afterward.
//
// Parameters:
//   - ctx: bounds the wait.
//   - conf: the flow's OAuth2 settings, whose redirect gets the server's port.
//   - redirect: the loopback redirect.
//   - state: the state sent with the authorization request.
//   - lines: the user's input, which may paste the address or the code.
//   - output: receives the authorization URL and the instructions.
//   - opts: the authorization URL parameters.
//
// Returns:
//   - string: the authorization code.
//   - error: a local server error, [ErrAuthorizationDenied], a pasted address
//     error, an input error wrapping [ErrScanFailed], or ctx's error.
func authorizeLoopback(
	ctx context.Context,
	conf *oauth2.Config,
	redirect *url.URL,
	state string,
	lines *lineReader,
	output io.Writer,
	opts []oauth2.AuthCodeOption,
) (string, error) {
	listeners, port, err := listenLoopback(ctx, redirect)
	if err != nil {
		return "", err
	}

	bound := *redirect
	bound.Host = net.JoinHostPort(redirect.Hostname(), port)
	conf.RedirectURL = bound.String()

	path := bound.Path
	if path == "" {
		path = "/"
	}

	results := make(chan authResult, 1)

	//nolint:exhaustruct_v5 // The other server settings keep their defaults.
	server := &http.Server{Handler: callbackHandler(path, state, results), ReadHeaderTimeout: callbackReadHeaderTimeout}

	for _, listener := range listeners {
		go func() {
			if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
				sendResult(results, authResult{err: fmt.Errorf("receiving the browser's redirect: %w", err), code: ""})
			}
		}()
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), callbackShutdownTimeout)
		defer cancel()

		// The code has arrived or the flow failed, so a slow redirect response
		// is cut off rather than reported.
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
		}
	}()

	if _, err := fmt.Fprintf(
		output,
		"Open the following URL in a browser to authorize Shoutrrr:\n%s\n\n"+
			"Waiting for the browser to return to %s.\n"+
			"For a browser on another machine, paste the address it was sent to or the code it shows: ",
		conf.AuthCodeURL(state, opts...),
		conf.RedirectURL,
	); err != nil {
		return "", fmt.Errorf("writing to output: %w", err)
	}

	for {
		select {
		case result := <-results:
			// The prompt for a pasted line is still open, so the next prompt
			// starts on its own line.
			if _, err := fmt.Fprintln(output); err != nil {
				return "", fmt.Errorf("writing to output: %w", err)
			}

			return result.code, result.err
		case line, ok := <-lines.lines:
			if !ok {
				// The sender's address is read from the same input after the
				// code, so the flow cannot complete without more input.
				return "", fmt.Errorf("verification code: %w", ErrScanFailed)
			}

			if strings.TrimSpace(line) == "" {
				// An empty line, such as Enter pressed while waiting, keeps
				// the wait for the browser.
				continue
			}

			return parsePasted(line, state)
		case <-ctx.Done():
			return "", fmt.Errorf("waiting for the browser: %w", ctx.Err())
		}
	}
}

// authorizePaste asks the user to paste the code or the address the browser
// was sent to after authorizing.
//
// Parameters:
//   - conf: the flow's OAuth2 settings.
//   - state: the state sent with the authorization request.
//   - lines: the user's input.
//   - output: receives the authorization URL and the prompt.
//   - opts: the authorization URL parameters.
//
// Returns:
//   - string: the authorization code.
//   - error: an input error wrapping [ErrScanFailed], or a pasted address error.
func authorizePaste(
	conf *oauth2.Config,
	state string,
	lines *lineReader,
	output io.Writer,
	opts []oauth2.AuthCodeOption,
) (string, error) {
	if _, err := fmt.Fprintf(
		output,
		"Visit the following URL to authorize Shoutrrr:\n%s\n\n",
		conf.AuthCodeURL(state, opts...),
	); err != nil {
		return "", fmt.Errorf("writing to output: %w", err)
	}

	line, err := ask(lines, output, "Enter the verification code, or the address the browser was sent to: ", "verification code")
	if err != nil {
		return "", err
	}

	return parsePasted(line, state)
}

// callbackHandler answers the browser's redirect and passes the first valid
// outcome to results. It compares the request's path with the redirect's
// path as plain text, so any path the redirect URL names works.
//
// Parameters:
//   - path: the redirect's path.
//   - state: the state sent with the authorization request.
//   - results: receives the code or the provider's error, once.
//
// Returns:
//   - http.Handler: the handler.
func callbackHandler(path, state string, results chan<- authResult) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)

			return
		}

		query := r.URL.Query()
		if query.Get("state") != state {
			http.Error(w, "This address belongs to another authorization.", http.StatusBadRequest)

			return
		}

		code, err := authorizationCode(query)
		if errors.Is(err, ErrNoAuthorizationCode) {
			http.Error(w, "This address has no authorization code.", http.StatusBadRequest)

			return
		}

		// The outcome goes to the flow before the page is written, so a browser
		// that left or reads slowly cannot hold it back.
		sendResult(results, authResult{err: err, code: code})

		page := successPage
		if err != nil {
			page = deniedPage
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if _, writeErr := io.WriteString(w, page); writeErr != nil {
			// The browser left before the page arrived, which changes nothing.
			return
		}
	})
}

// sendResult passes result to results unless it already holds one, so only
// the first outcome counts.
//
// Parameters:
//   - results: the channel, with room for one result.
//   - result: the outcome.
func sendResult(results chan<- authResult, result authResult) {
	select {
	case results <- result:
	default:
	}
}

// parsePasted reads a pasted line as the address the browser was sent to, or
// as the code itself when it is not an address with a query.
//
// Parameters:
//   - line: the pasted line.
//   - state: the state sent with the authorization request.
//
// Returns:
//   - string: the authorization code.
//   - error: [ErrStateMismatch] for an address with another state, or the
//     error from reading the address's code.
func parsePasted(line, state string) (string, error) {
	line = strings.TrimSpace(line)

	address, err := url.Parse(line)
	if err != nil {
		return line, nil //nolint:nilerr // A line that is not an address is the code itself.
	}

	if address.Scheme == "" || address.RawQuery == "" {
		return line, nil
	}

	query := address.Query()
	if query.Get("state") != state {
		return "", ErrStateMismatch
	}

	return authorizationCode(query)
}

// authorizationCode reads the outcome of an authorization from the query of
// the redirect.
//
// Parameters:
//   - query: the redirect's query.
//
// Returns:
//   - string: the authorization code.
//   - error: [ErrAuthorizationDenied] with the provider's error, or
//     [ErrNoAuthorizationCode].
func authorizationCode(query url.Values) (string, error) {
	if reason := query.Get("error"); reason != "" {
		return "", fmt.Errorf("%w: %s", ErrAuthorizationDenied, reason)
	}

	code := query.Get("code")
	if code == "" {
		return "", ErrNoAuthorizationCode
	}

	return code, nil
}

// loopbackRedirect reports whether a redirect URL is an HTTP address on the
// loopback interface, which a local server can receive.
//
// Parameters:
//   - redirectURL: the configured redirect URL.
//
// Returns:
//   - *url.URL: the parsed redirect.
//   - bool: whether it is a loopback redirect.
func loopbackRedirect(redirectURL string) (*url.URL, bool) {
	redirect, err := url.Parse(redirectURL)
	if err != nil || redirect.Scheme != "http" {
		return nil, false
	}

	// Host names are case-insensitive, and url.Parse keeps the case it is given.
	switch strings.ToLower(redirect.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return redirect, true
	default:
		return nil, false
	}
}

// listenLoopback listens on the redirect's address. Without a port, it picks a
// free one. A browser may resolve localhost to 127.0.0.1 or ::1. So for
// localhost it listens on 127.0.0.1 and also on ::1 with the same port where
// IPv6 is available.
//
// Parameters:
//   - ctx: bounds setting up the listeners.
//   - redirect: the loopback redirect.
//
// Returns:
//   - []net.Listener: the listeners.
//   - string: the port they listen on.
//   - error: the error from listening on the first address.
func listenLoopback(ctx context.Context, redirect *url.URL) ([]net.Listener, string, error) {
	hosts := []string{redirect.Hostname()}
	if strings.EqualFold(hosts[0], "localhost") {
		hosts = []string{"127.0.0.1", "::1"}
	}

	port := redirect.Port()
	if port == "" {
		port = "0"
	}

	var listenConfig net.ListenConfig

	first, err := listenConfig.Listen(ctx, "tcp", net.JoinHostPort(hosts[0], port))
	if err != nil {
		return nil, "", fmt.Errorf("listening for the browser's redirect: %w", err)
	}

	_, port, err = net.SplitHostPort(first.Addr().String())
	if err != nil {
		_ = first.Close()

		return nil, "", fmt.Errorf("reading the redirect server's port: %w", err)
	}

	listeners := []net.Listener{first}

	for _, host := range hosts[1:] {
		// IPv6 may be unavailable, and the first address is enough then.
		if listener, err := listenConfig.Listen(ctx, "tcp", net.JoinHostPort(host, port)); err == nil {
			listeners = append(listeners, listener)
		}
	}

	return listeners, port, nil
}

// ask writes a prompt and reads the answer.
//
// Parameters:
//   - lines: the user's input.
//   - output: receives the prompt.
//   - prompt: the prompt.
//   - field: the answer's name, for the error.
//
// Returns:
//   - string: the answer.
//   - error: an output error, or an error wrapping [ErrScanFailed] when the
//     input has ended.
func ask(lines *lineReader, output io.Writer, prompt, field string) (string, error) {
	if _, err := fmt.Fprint(output, prompt); err != nil {
		return "", fmt.Errorf("writing to output: %w", err)
	}

	answer, ok := lines.next()
	if !ok {
		return "", fmt.Errorf("%s: %w", field, ErrScanFailed)
	}

	return answer, nil
}

// newSMTPConfig builds the SMTP configuration for an authorized sender.
//
// With a refresh token, the configuration holds it with the client ID, client
// secret, and token URL, and no password, since each send gets a current
// access token. Without one, the access token is the password.
//
// Parameters:
//   - conf: the provider's OAuth2 settings.
//   - token: the tokens from the code exchange.
//   - host: the SMTP server host.
//   - sender: the sender's e-mail address.
//
// Returns:
//   - *smtp.Config: the SMTP configuration.
func newSMTPConfig(conf *oauth2.Config, token *oauth2.Token, host, sender string) *smtp.Config {
	// Determine the appropriate port based on the host
	port := DefaultSMTPPort
	if host == "smtp.gmail.com" {
		port = GmailSMTPPortStartTLS // Use 587 for Gmail with STARTTLS
	}

	config := &smtp.Config{
		Host:        host,
		Port:        port,
		Username:    sender,
		Password:    token.AccessToken,
		FromAddress: sender,
		FromName:    "Shoutrrr",
		ToAddresses: []string{sender},
		Auth:        smtp.AuthTypes.OAuth2,
		Encryption:  smtp.EncMethods.Auto,
		ClientHost:  smtpClientHost,
		Timeout:     smtpTimeout,
		UseStartTLS: true,
		UseHTML:     true,
	}

	if token.RefreshToken != "" {
		config.Password = ""
		config.ClientID = conf.ClientID
		config.ClientSecret = conf.ClientSecret
		config.RefreshToken = token.RefreshToken
		config.TokenURL = conf.Endpoint.TokenURL
	}

	return config
}

// noRefreshTokenWarning returns the warning for a provider that returned no
// refresh token, with the access token's expiry when known.
//
// Parameters:
//   - token: the tokens from the code exchange.
//
// Returns:
//   - string: the warning.
func noRefreshTokenWarning(token *oauth2.Token) string {
	warning := "Warning: the provider returned no refresh token, so the URL works only until the access token expires"
	if !token.Expiry.IsZero() {
		warning += " at " + token.Expiry.Format(time.RFC3339)
	}

	return warning + "."
}

// oauth2Generator generates OAuth2 configuration from user input.
//
// Parameters:
//   - ctx: bounds the authorization.
//   - lines: the user's answers.
//   - output: receives the prompts.
//
// Returns:
//   - *smtp.Config: the SMTP configuration.
//   - error: an output error, an error wrapping [ErrScanFailed] when input
//     ends early, or an authorization or token exchange error.
func oauth2Generator(ctx context.Context, lines *lineReader, output io.Writer) (*smtp.Config, error) {
	// questions lists the settings asked for, in order, with their prompts.
	questions := []struct {
		prompt string
		field  string
	}{
		{prompt: "ClientID: ", field: "clientID"},
		{prompt: "ClientSecret: ", field: "clientSecret"},
		{prompt: "AuthURL: ", field: "authURL"},
		{prompt: "TokenURL: ", field: "tokenURL"},
		{prompt: "RedirectURL: ", field: "redirectURL"},
		{prompt: "Scopes: ", field: "scopes"},
		{prompt: "SMTP Hostname: ", field: "hostname"},
	}

	answers := make([]string, len(questions))

	for i, question := range questions {
		answer, err := ask(lines, output, question.prompt, question.field)
		if err != nil {
			return nil, err
		}

		answers[i] = answer
	}

	conf := oauth2.Config{
		ClientID:     answers[0],
		ClientSecret: answers[1],

		Endpoint: oauth2.Endpoint{
			AuthURL:   answers[2],
			TokenURL:  answers[3],
			AuthStyle: oauth2.AuthStyleAutoDetect,
		},
		RedirectURL: answers[4],
		Scopes:      strings.Split(answers[5], ","),
	}

	return runOAuth2Flow(ctx, &conf, answers[6], lines, output)
}

// oauth2GeneratorFile generates OAuth2 configuration from a JSON file.
//
// Parameters:
//   - ctx: bounds the authorization.
//   - file: the path of the JSON file with the provider's OAuth2 settings.
//   - lines: the user's answers.
//   - output: receives the prompts.
//
// Returns:
//   - *smtp.Config: the SMTP configuration.
//   - error: a file error wrapping [ErrReadFileFailed] or [ErrUnmarshalFailed] and
//     the underlying error, or an input, authorization, or token exchange error.
func oauth2GeneratorFile(ctx context.Context, file string, lines *lineReader, output io.Writer) (*smtp.Config, error) {
	jsonData, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", file, ErrReadFileFailed, err)
	}

	var providerConfig struct {
		// ClientID is the OAuth2 client ID.
		ClientID string `json:"client_id"`
		// ClientSecret is the OAuth2 client secret.
		ClientSecret string `json:"client_secret"`
		// RedirectURL is where the provider sends the user after authorizing.
		RedirectURL string `json:"redirect_url"`
		// AuthURL is the provider's authorization endpoint.
		AuthURL string `json:"auth_url"`
		// TokenURL is the provider's token endpoint.
		TokenURL string `json:"token_url"`
		// Hostname is the SMTP server host.
		Hostname string `json:"smtp_hostname"`
		// Scopes are the OAuth2 scopes to request.
		Scopes []string `json:"scopes"`
	}

	if err := json.Unmarshal(jsonData, &providerConfig, jsonopts.Default); err != nil {
		return nil, fmt.Errorf("%s: %w: %w", file, ErrUnmarshalFailed, err)
	}

	conf := oauth2.Config{
		ClientID:     providerConfig.ClientID,
		ClientSecret: providerConfig.ClientSecret,

		Endpoint: oauth2.Endpoint{
			AuthURL:   providerConfig.AuthURL,
			TokenURL:  providerConfig.TokenURL,
			AuthStyle: oauth2.AuthStyleAutoDetect,
		},
		RedirectURL: providerConfig.RedirectURL,
		Scopes:      providerConfig.Scopes,
	}

	return runOAuth2Flow(ctx, &conf, providerConfig.Hostname, lines, output)
}

// oauth2GeneratorGmail generates OAuth2 configuration for Gmail using credentials file.
//
// Older Google desktop credentials name the copy and paste (out-of-band)
// redirect first. Google rejects that redirect, so such a file is given a
// loopback redirect on 127.0.0.1 instead.
//
// Parameters:
//   - ctx: bounds the authorization.
//   - credFile: the path of the Google credentials file.
//   - lines: the user's answers.
//   - output: receives the prompts.
//
// Returns:
//   - *smtp.Config: the SMTP configuration.
//   - error: a file error wrapping [ErrReadFileFailed] and the underlying error, a
//     credentials parsing error, or an input, authorization, or token exchange error.
func oauth2GeneratorGmail(ctx context.Context, credFile string, lines *lineReader, output io.Writer) (*smtp.Config, error) {
	data, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", credFile, ErrReadFileFailed, err)
	}

	conf, err := google.ConfigFromJSON(data, "https://mail.google.com/")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", credFile, err)
	}

	if conf.RedirectURL == "" || strings.HasPrefix(conf.RedirectURL, "urn:ietf:wg:oauth:2.0:oob") {
		conf.RedirectURL = googleLoopbackRedirect
	}

	// Google returns a refresh token to web clients only with offline access,
	// and again on a later grant only after the consent prompt.
	return runOAuth2Flow(ctx, conf, "smtp.gmail.com", lines, output, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}
