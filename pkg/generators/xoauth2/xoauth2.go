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
type Generator struct{}

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
//     or [ErrUnmarshalFailed], or an input or token exchange error.
func (g *Generator) Generate(
	_ types.Service,
	props map[string]string,
	args []string,
) (types.ServiceConfig, error) {
	if provider, found := props["provider"]; found {
		if provider == "gmail" {
			if len(args) == 0 {
				return nil, ErrCredentialsFileRequired
			}

			return oauth2GeneratorGmail(args[0])
		}
	}

	if len(args) > 0 {
		return oauth2GeneratorFile(args[0])
	}

	return oauth2Generator()
}

// generateOauth2Config completes the OAuth2 flow on the terminal and generates
// the SMTP configuration.
//
// Parameters:
//   - conf: the provider's OAuth2 settings.
//   - host: the SMTP server host.
//   - opts: extra parameters for the authorization URL, such as Google's
//     offline access.
//
// Returns:
//   - *smtp.Config: the SMTP configuration.
//   - error: an input or token exchange error.
func generateOauth2Config(conf *oauth2.Config, host string, opts ...oauth2.AuthCodeOption) (*smtp.Config, error) {
	return runOAuth2Flow(context.Background(), conf, host, os.Stdin, os.Stdout, opts...)
}

// runOAuth2Flow asks the user to authorize Shoutrrr, exchanges the code for
// tokens, and generates the SMTP configuration.
//
// The configuration holds the refresh token and the client settings, so the
// SMTP service can get a current access token for each send. A provider that
// returns no refresh token leaves only the access token, used as the password
// until it expires, and the user is warned.
//
// Parameters:
//   - ctx: bounds the token exchange.
//   - conf: the provider's OAuth2 settings.
//   - host: the SMTP server host.
//   - input: the user's answers: the verification code, then the sender.
//   - output: receives the prompts and the warning.
//   - opts: extra parameters for the authorization URL.
//
// Returns:
//   - *smtp.Config: the SMTP configuration.
//   - error: an input error wrapping [ErrScanFailed], or an error wrapping
//     [ErrTokenExchangeFailed] and the provider's error.
func runOAuth2Flow(
	ctx context.Context,
	conf *oauth2.Config,
	host string,
	input io.Reader,
	output io.Writer,
	opts ...oauth2.AuthCodeOption,
) (*smtp.Config, error) {
	scanner := bufio.NewScanner(input)

	// Generate a random state value
	stateBytes := make([]byte, StateLength)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("generating random state: %w", err)
	}

	state := base64.URLEncoding.EncodeToString(stateBytes)

	if _, err := fmt.Fprintf(
		output,
		"Visit the following URL to authenticate:\n%s\n\n",
		conf.AuthCodeURL(state, opts...),
	); err != nil {
		return nil, fmt.Errorf("writing to output: %w", err)
	}

	var verCode string

	if _, err := fmt.Fprint(output, "Enter verification code: "); err != nil {
		return nil, fmt.Errorf("writing to output: %w", err)
	}

	if scanner.Scan() {
		verCode = scanner.Text()
	} else {
		return nil, fmt.Errorf("verification code: %w", ErrScanFailed)
	}

	token, err := conf.Exchange(ctx, verCode)
	if err != nil {
		// The verification code is a credential, so the error leaves it out.
		return nil, fmt.Errorf("%w: %w", ErrTokenExchangeFailed, err)
	}

	var sender string

	if _, err := fmt.Fprint(output, "Enter sender e-mail: "); err != nil {
		return nil, fmt.Errorf("writing to output: %w", err)
	}

	if scanner.Scan() {
		sender = scanner.Text()
	} else {
		return nil, fmt.Errorf("sender email: %w", ErrScanFailed)
	}

	if token.RefreshToken == "" {
		if _, err := fmt.Fprintln(output, noRefreshTokenWarning(token)); err != nil {
			return nil, fmt.Errorf("writing to output: %w", err)
		}
	}

	return newSMTPConfig(conf, token, host, sender), nil
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
// Returns:
//   - *smtp.Config: the SMTP configuration.
//   - error: an output error, an error wrapping [ErrScanFailed] when input
//     ends early, or a token exchange error.
func oauth2Generator() (*smtp.Config, error) {
	scanner := bufio.NewScanner(os.Stdin)

	var clientID string

	if _, err := fmt.Fprint(os.Stdout, "ClientID: "); err != nil {
		return nil, fmt.Errorf("writing to stdout: %w", err)
	}

	if scanner.Scan() {
		clientID = scanner.Text()
	} else {
		return nil, fmt.Errorf("clientID: %w", ErrScanFailed)
	}

	var clientSecret string

	if _, err := fmt.Fprint(os.Stdout, "ClientSecret: "); err != nil {
		return nil, fmt.Errorf("writing to stdout: %w", err)
	}

	if scanner.Scan() {
		clientSecret = scanner.Text()
	} else {
		return nil, fmt.Errorf("clientSecret: %w", ErrScanFailed)
	}

	var authURL string

	if _, err := fmt.Fprint(os.Stdout, "AuthURL: "); err != nil {
		return nil, fmt.Errorf("writing to stdout: %w", err)
	}

	if scanner.Scan() {
		authURL = scanner.Text()
	} else {
		return nil, fmt.Errorf("authURL: %w", ErrScanFailed)
	}

	var tokenURL string

	if _, err := fmt.Fprint(os.Stdout, "TokenURL: "); err != nil {
		return nil, fmt.Errorf("writing to stdout: %w", err)
	}

	if scanner.Scan() {
		tokenURL = scanner.Text()
	} else {
		return nil, fmt.Errorf("tokenURL: %w", ErrScanFailed)
	}

	var redirectURL string

	if _, err := fmt.Fprint(os.Stdout, "RedirectURL: "); err != nil {
		return nil, fmt.Errorf("writing to stdout: %w", err)
	}

	if scanner.Scan() {
		redirectURL = scanner.Text()
	} else {
		return nil, fmt.Errorf("redirectURL: %w", ErrScanFailed)
	}

	var scopes string

	if _, err := fmt.Fprint(os.Stdout, "Scopes: "); err != nil {
		return nil, fmt.Errorf("writing to stdout: %w", err)
	}

	if scanner.Scan() {
		scopes = scanner.Text()
	} else {
		return nil, fmt.Errorf("scopes: %w", ErrScanFailed)
	}

	var hostname string

	if _, err := fmt.Fprint(os.Stdout, "SMTP Hostname: "); err != nil {
		return nil, fmt.Errorf("writing to stdout: %w", err)
	}

	if scanner.Scan() {
		hostname = scanner.Text()
	} else {
		return nil, fmt.Errorf("hostname: %w", ErrScanFailed)
	}

	conf := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,

		Endpoint: oauth2.Endpoint{
			AuthURL:   authURL,
			TokenURL:  tokenURL,
			AuthStyle: oauth2.AuthStyleAutoDetect,
		},
		RedirectURL: redirectURL,
		Scopes:      strings.Split(scopes, ","),
	}

	return generateOauth2Config(&conf, hostname)
}

// oauth2GeneratorFile generates OAuth2 configuration from a JSON file.
//
// Parameters:
//   - file: the path of the JSON file with the provider's OAuth2 settings.
//
// Returns:
//   - *smtp.Config: the SMTP configuration.
//   - error: a file error wrapping [ErrReadFileFailed] or [ErrUnmarshalFailed] and
//     the underlying error, or an input or token exchange error.
func oauth2GeneratorFile(file string) (*smtp.Config, error) {
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

	return generateOauth2Config(&conf, providerConfig.Hostname)
}

// oauth2GeneratorGmail generates OAuth2 configuration for Gmail using credentials file.
//
// Parameters:
//   - credFile: the path of the Google credentials file.
//
// Returns:
//   - *smtp.Config: the SMTP configuration.
//   - error: a file error wrapping [ErrReadFileFailed] and the underlying error, a
//     credentials parsing error, or an input or token exchange error.
func oauth2GeneratorGmail(credFile string) (*smtp.Config, error) {
	data, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", credFile, ErrReadFileFailed, err)
	}

	conf, err := google.ConfigFromJSON(data, "https://mail.google.com/")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", credFile, err)
	}

	// Google returns a refresh token to web clients only with offline access,
	// and again on a later grant only after the consent prompt.
	return generateOauth2Config(conf, "smtp.gmail.com", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}
