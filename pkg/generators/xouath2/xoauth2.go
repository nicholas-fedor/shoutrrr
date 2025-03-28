//go:generate stringer -type=URLPart -trimprefix URL

package xouath2

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/smtp"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// SMTP port constants.
const (
	DefaultSMTPPort       uint16 = 25  // Standard SMTP port without encryption
	GmailSMTPPortStartTLS uint16 = 587 // Gmail SMTP port with STARTTLS
)

// Errors.
var (
	ErrReadFileFailed      = errors.New("failed to read file")
	ErrUnmarshalFailed     = errors.New("failed to unmarshal JSON")
	ErrScanFailed          = errors.New("failed to scan input")
	ErrTokenExchangeFailed = errors.New("failed to exchange token")
)

// Generator is the XOAuth2 Generator implementation.
type Generator struct{}

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

func oauth2GeneratorFile(file string) (*smtp.Config, error) {
	jsonData, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, ErrReadFileFailed)
	}

	var providerConfig struct {
		ClientID     string   `json:"client_id"`
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

func oauth2Generator() (*smtp.Config, error) {
	scanner := bufio.NewScanner(os.Stdin)

	var clientID string

	fmt.Fprint(os.Stdout, "ClientID: ")

	if scanner.Scan() {
		clientID = scanner.Text()
	} else {
		return nil, fmt.Errorf("clientID: %w", ErrScanFailed)
	}

	var clientSecret string

	fmt.Fprint(os.Stdout, "ClientSecret: ")

	if scanner.Scan() {
		clientSecret = scanner.Text()
	} else {
		return nil, fmt.Errorf("clientSecret: %w", ErrScanFailed)
	}

	var authURL string

	fmt.Fprint(os.Stdout, "AuthURL: ")

	if scanner.Scan() {
		authURL = scanner.Text()
	} else {
		return nil, fmt.Errorf("authURL: %w", ErrScanFailed)
	}

	var tokenURL string

	fmt.Fprint(os.Stdout, "TokenURL: ")

	if scanner.Scan() {
		tokenURL = scanner.Text()
	} else {
		return nil, fmt.Errorf("tokenURL: %w", ErrScanFailed)
	}

	var redirectURL string

	fmt.Fprint(os.Stdout, "RedirectURL: ")

	if scanner.Scan() {
		redirectURL = scanner.Text()
	} else {
		return nil, fmt.Errorf("redirectURL: %w", ErrScanFailed)
	}

	var scopes string

	fmt.Fprint(os.Stdout, "Scopes: ")

	if scanner.Scan() {
		scopes = scanner.Text()
	} else {
		return nil, fmt.Errorf("scopes: %w", ErrScanFailed)
	}

	var hostname string

	fmt.Fprint(os.Stdout, "SMTP Hostname: ")

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

func oauth2GeneratorGmail(credFile string) (*smtp.Config, error) {
	data, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", credFile, ErrReadFileFailed)
	}

	conf, err := google.ConfigFromJSON(data, "https://mail.google.com/")
	if err != nil {
		return nil, fmt.Errorf(
			"%s: %w",
			credFile,
			err,
		) // google.ConfigFromJSON error doesn't need custom wrapping
	}

	return generateOauth2Config(conf, "smtp.gmail.com")
}

func generateOauth2Config(conf *oauth2.Config, host string) (*smtp.Config, error) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Fprintf(os.Stdout, "Visit the following URL to authenticate:\n%s\n\n", conf.AuthCodeURL(""))

	var verCode string

	fmt.Fprint(os.Stdout, "Enter verification code: ")

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

	fmt.Fprint(os.Stdout, "Enter sender e-mail: ")

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
