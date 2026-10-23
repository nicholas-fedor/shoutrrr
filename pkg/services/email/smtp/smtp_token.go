package smtp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// tokenKey identifies the access token cached for one set of refresh settings,
// so a send with other settings never uses a token refreshed with other
// credentials.
type tokenKey struct {
	// tokenURL is the provider's token endpoint.
	tokenURL string
	// clientID is the OAuth2 client ID.
	clientID string
	// clientSecret is the OAuth2 client secret.
	clientSecret string
	// refreshToken is the refresh token from the configuration.
	refreshToken string
}

// clientTransport adapts a [types.HTTPClient] to an [http.RoundTripper], so
// the oauth2 package can send token requests through an injected client.
type clientTransport struct {
	// client sends the requests.
	client types.HTTPClient
}

// tokenRefresh is a token request in flight, which concurrent sends with the
// same refresh settings wait for.
type tokenRefresh struct {
	// token is the new token, set before done is closed when err is nil.
	token *oauth2.Token
	// err is the request's error, set before done is closed.
	err error
	// done is closed when the request ends.
	done chan struct{}
	// canceled reports that the request failed because the context of the
	// send that made it ended, so a waiting send makes its own request.
	canceled bool
}

// RoundTrip sends the request through the wrapped client.
//
// Parameters:
//   - req: the token request.
//
// Returns:
//   - *http.Response: the response.
//   - error: the client's error.
func (t clientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.client.Do(req)
}

// tokenKey returns the key of the access token cached for c's refresh settings.
//
// Returns:
//   - tokenKey: the cache key.
func (c *Config) tokenKey() tokenKey {
	return tokenKey{
		tokenURL:     c.TokenURL,
		clientID:     c.ClientID,
		clientSecret: c.ClientSecret,
		refreshToken: c.RefreshToken,
	}
}

// usesTokenRefresh reports whether a send gets its access token by refreshing.
//
// Returns:
//   - bool: true when a refresh token is set.
func (c *Config) usesTokenRefresh() bool {
	return c.RefreshToken != ""
}

// validateOAuth2Refresh checks the OAuth2 refresh settings of a configuration.
//
// The settings are optional, but when any is set they need OAuth2
// authentication, a refresh token, a client ID, and a token URL. The refresh
// token and client secret travel in the token request, so the token URL must
// be HTTPS, or HTTP to a loopback host, as SMTP credentials need TLS or
// localhost.
//
// Parameters:
//   - config: the configuration to check.
//
// Returns:
//   - error: [ErrInvalidOAuth2Refresh] with the reason, or nil.
func validateOAuth2Refresh(config *Config) error {
	if config.RefreshToken == "" && config.ClientID == "" && config.ClientSecret == "" && config.TokenURL == "" {
		return nil
	}

	switch {
	case config.Auth != AuthTypes.OAuth2:
		return fmt.Errorf("%w: the refresh settings need auth=OAuth2", ErrInvalidOAuth2Refresh)
	case config.RefreshToken == "":
		return fmt.Errorf("%w: the refresh token is missing", ErrInvalidOAuth2Refresh)
	case config.ClientID == "":
		return fmt.Errorf("%w: the client ID is missing", ErrInvalidOAuth2Refresh)
	case config.TokenURL == "":
		return fmt.Errorf("%w: the token URL is missing", ErrInvalidOAuth2Refresh)
	}

	tokenURL, err := url.Parse(config.TokenURL)

	switch {
	case err != nil:
		return fmt.Errorf("%w: the token URL does not parse", ErrInvalidOAuth2Refresh)
	case tokenURL.Hostname() == "":
		return fmt.Errorf("%w: the token URL has no host", ErrInvalidOAuth2Refresh)
	case tokenURL.Scheme == "https":
		return nil
	case tokenURL.Scheme == "http" && isLocalhost(tokenURL.Hostname()):
		return nil
	default:
		return fmt.Errorf("%w: the token URL must be HTTPS, or HTTP to localhost", ErrInvalidOAuth2Refresh)
	}
}

// SetHTTPClient sets the HTTP client for OAuth2 token requests. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for token requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// accessToken returns a current OAuth2 access token for config.
//
// A token cached for the same refresh settings is reused until it expires.
// Otherwise a new one is requested from the token URL through the HTTP client,
// bounded by ctx. When the provider returns a new refresh token, later
// refreshes use it.
//
// Concurrent sends with the same refresh settings share one token request. A
// waiting send returns when its own ctx ends, and retries when the request
// failed only because the sender that made it was canceled. Sends with other
// refresh settings, or with a cached token, never wait for the request.
//
// Parameters:
//   - ctx: bounds the token request, or the wait for one in flight.
//   - config: the configuration for this send, with valid refresh settings.
//
// Returns:
//   - string: the access token.
//   - error: a [FailTokenRefresh] failure wrapping the token request's error,
//     or ctx's error while waiting.
func (s *Service) accessToken(ctx context.Context, config *Config) (string, error) {
	key := config.tokenKey()

	for {
		s.tokenMu.Lock()

		cached := s.tokens[key]
		if cached.Valid() {
			s.tokenMu.Unlock()

			return cached.AccessToken, nil
		}

		if inFlight := s.refreshes[key]; inFlight != nil {
			s.tokenMu.Unlock()

			select {
			case <-inFlight.done:
			case <-ctx.Done():
				return "", fail(FailTokenRefresh, ctx.Err())
			}

			if inFlight.canceled {
				continue
			}

			if inFlight.err != nil {
				return "", inFlight.err
			}

			return inFlight.token.AccessToken, nil
		}

		refreshToken := config.RefreshToken
		if cached != nil && cached.RefreshToken != "" {
			refreshToken = cached.RefreshToken
		}

		refresh := &tokenRefresh{token: nil, err: nil, done: make(chan struct{}), canceled: false}

		if s.refreshes == nil {
			s.refreshes = map[tokenKey]*tokenRefresh{}
		}

		s.refreshes[key] = refresh
		client := s.tokenHTTPClient()

		s.tokenMu.Unlock()

		token, err := requestToken(ctx, client, config, refreshToken)

		s.tokenMu.Lock()

		delete(s.refreshes, key)

		if err == nil {
			if s.tokens == nil {
				s.tokens = map[tokenKey]*oauth2.Token{}
			}

			s.tokens[key] = token
		}

		refresh.token = token
		refresh.err = err
		refresh.canceled = err != nil && ctx.Err() != nil
		close(refresh.done)

		s.tokenMu.Unlock()

		if err != nil {
			return "", err
		}

		return token.AccessToken, nil
	}
}

// forgetToken drops the rejected access token cached for config's refresh
// settings, so the next send refreshes it. A send calls it when the server
// rejects the token, such as one revoked early or cached without an expiry.
//
// Only the token the send used, held in config's password, is dropped. A newer
// token that another send cached since is kept. The cached refresh token is
// kept for the next refresh, since a provider that rotates refresh tokens and
// detects reuse may revoke the original.
//
// Parameters:
//   - config: the configuration of the rejected send, with the access token
//     it used as its password.
func (s *Service) forgetToken(config *Config) {
	key := config.tokenKey()

	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	cached := s.tokens[key]
	if cached == nil || cached.AccessToken != config.Password {
		return
	}

	s.tokens[key] = &oauth2.Token{
		AccessToken:  "",
		TokenType:    "",
		RefreshToken: cached.RefreshToken,
		Expiry:       time.Time{},
		ExpiresIn:    0,
	}
}

// tokenHTTPClient returns the *http.Client for token requests.
//
// It sends through the injected client, or the default client, by way of
// [clientTransport]. The send's context bounds every request.
//
// Returns:
//   - *http.Client: the client for token requests.
func (s *Service) tokenHTTPClient() *http.Client {
	return &http.Client{Transport: clientTransport{client: s.client.Client()}}
}

// requestToken requests an access token from config's token URL with
// refreshToken.
//
// Parameters:
//   - ctx: bounds the request.
//   - client: sends the request.
//   - config: the configuration with the client ID, client secret, and token URL.
//   - refreshToken: the refresh token to redeem.
//
// Returns:
//   - *oauth2.Token: the new token. It keeps refreshToken when the provider
//     returns no new one.
//   - error: a [FailTokenRefresh] failure wrapping the request's error, with
//     the token URL reduced to its scheme and host.
func requestToken(
	ctx context.Context,
	client *http.Client,
	config *Config,
	refreshToken string,
) (*oauth2.Token, error) {
	oauthConfig := oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL:  config.TokenURL,
			AuthStyle: oauth2.AuthStyleAutoDetect,
		},
	}

	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)

	token, err := oauthConfig.TokenSource(ctx, &oauth2.Token{
		AccessToken:  "",
		TokenType:    "",
		RefreshToken: refreshToken,
		Expiry:       time.Time{},
		ExpiresIn:    0,
	}).Token()
	if err != nil {
		// The client's transport error names the token URL, whose path may
		// identify the account, such as a Microsoft tenant.
		return nil, fail(FailTokenRefresh, redact.URLError(err))
	}

	return token, nil
}
