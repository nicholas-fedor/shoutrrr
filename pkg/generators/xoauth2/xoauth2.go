// Package xoauth2 provides OAuth2 authentication for email services.
package xoauth2

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/email/smtp"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Generator is the XOAuth2 Generator implementation.
type Generator struct{}

// SMTP port constants.
const (
	DefaultSMTPPort       uint16 = 25  // Standard SMTP port without encryption
	GmailSMTPPortStartTLS uint16 = 587 // Gmail SMTP port with STARTTLS
)

// StateLength is the length in bytes for OAuth 2.0 state randomness (128 bits).
const StateLength int = 16

// Errors.
var (
	ErrReadFileFailed      = errors.New("failed to read file")
	ErrUnmarshalFailed     = errors.New("failed to unmarshal JSON")
	ErrScanFailed          = errors.New("failed to scan input")
	ErrTokenExchangeFailed = errors.New("failed to exchange token")
)

// Generate generates a service URL from a set of user questions/answers.
func (g *Generator) Generate(
	_ types.Service,
	props map[string]string,
	args []string,
) (types.ServiceConfig, error) {
	if provider, found := props["provider"]; found {
		if provider == "gmail" {
			return oauth2GeneratorGmail(args[0])
		}
	}

	if len(args) > 0 {
		return oauth2GeneratorFile(args[0])
	}

	return oauth2Generator()
}

// generateOauth2Config completes the OAuth2 flow and generates SMTP configuration.
func generateOauth2Config(conf *oauth2.Config, host string) (*smtp.Config, error) {
	scanner := bufio.NewScanner(os.Stdin)

	// Generate a random state value
	stateBytes := make([]byte, StateLength)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("generating random state: %w", err)
	}

	state := base64.URLEncoding.EncodeToString(stateBytes)

	if _, err := fmt.Fprintf(
		os.Stdout,
		"Visit the following URL to authenticate:\n%s\n\n",
		conf.AuthCodeURL(state),
	); err != nil {
		return nil, fmt.Errorf("writing to stdout: %w", err)
	}

	var verCode string

	if _, err := fmt.Fprint(os.Stdout, "Enter verification code: "); err != nil {
		return nil, fmt.Errorf("writing to stdout: %w", err)
	}

	if scanner.Scan() {
		verCode = scanner.Text()
	} else {
		return nil, fmt.Errorf("verification code: %w", ErrScanFailed)
	}

	ctx := context.Background()

	token, err := conf.Exchange(ctx, verCode)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", verCode, ErrTokenExchangeFailed)
	}

	var sender string

	if _, err := fmt.Fprint(os.Stdout, "Enter sender e-mail: "); err != nil {
		return nil, fmt.Errorf("writing to stdout: %w", err)
	}

	if scanner.Scan() {
		sender = scanner.Text()
	} else {
		return nil, fmt.Errorf("sender email: %w", ErrScanFailed)
	}

	// Determine the appropriate port based on the host
	port := DefaultSMTPPort
	if host == "smtp.gmail.com" {
		port = GmailSMTPPortStartTLS // Use 587 for Gmail with STARTTLS
	}

	//nolint:exhaustruct // Missing fields have sensible defaults
	svcConf := &smtp.Config{
		Host:        host,
		Port:        port,
		Username:    sender,
		Password:    token.AccessToken,
		FromAddress: sender,
		FromName:    "Shoutrrr",
		ToAddresses: []string{sender},
		Auth:        smtp.AuthTypes.OAuth2,
		UseStartTLS: true,
		UseHTML:     true,
	}

	return svcConf, nil
}

// oauth2Generator generates OAuth2 configuration from user input.
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
		//nolint:exhaustruct // DeviceAuthURL is optional for this OAuth2 flow
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
func oauth2GeneratorFile(file string) (*smtp.Config, error) {
	jsonData, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, ErrReadFileFailed)
	}

	var providerConfig struct {
		ClientID string `json:"client_id"`
		//nolint:gosec // ClientSecret is standard OAuth2 field name, not a hardcoded credential
		ClientSecret string   `json:"client_secret"`
		RedirectURL  string   `json:"redirect_url"`
		AuthURL      string   `json:"auth_url"`
		TokenURL     string   `json:"token_url"`
		Hostname     string   `json:"smtp_hostname"`
		Scopes       []string `json:"scopes"`
	}

	if err := json.Unmarshal(jsonData, &providerConfig); err != nil {
		return nil, fmt.Errorf("%s: %w", file, ErrUnmarshalFailed)
	}

	conf := oauth2.Config{
		ClientID:     providerConfig.ClientID,
		ClientSecret: providerConfig.ClientSecret,
		//nolint:exhaustruct // DeviceAuthURL is optional for this OAuth2 flow
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
func oauth2GeneratorGmail(credFile string) (*smtp.Config, error) {
	data, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", credFile, ErrReadFileFailed)
	}

	conf, err := google.ConfigFromJSON(data, "https://mail.google.com/")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", credFile, err)
	}

	return generateOauth2Config(conf, "smtp.gmail.com")
}
