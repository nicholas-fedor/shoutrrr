package smtp

import (
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// authType is an SMTP authentication method.
type authType int

// authTypeVals holds named authentication methods and their enum formatter.
type authTypeVals struct {
	// Enum formats and parses authentication method values.
	Enum types.EnumFormatter
	// None disables SMTP authentication.
	None authType
	// Plain uses AUTH PLAIN.
	Plain authType
	// CRAMMD5 uses AUTH CRAM-MD5.
	CRAMMD5 authType
	// Unknown means the method has not been set or could not be parsed.
	Unknown authType
	// OAuth2 uses SASL XOAUTH2. See [newOAuth2Auth].
	OAuth2 authType
	// Login uses SASL LOGIN. See [newLoginAuth].
	Login authType
}

// Authentication methods, in [AuthTypes] order.
//
// AuthUnknown is the zero value, so a config built in code that leaves Auth
// unset infers its method as a URL without the auth key does.
const (
	// AuthUnknown means the method has not been set or could not be parsed.
	AuthUnknown authType = iota
	// AuthNone disables SMTP authentication.
	AuthNone
	// AuthPlain uses AUTH PLAIN.
	AuthPlain
	// AuthCRAMMD5 uses AUTH CRAM-MD5.
	AuthCRAMMD5
	// AuthOAuth2 uses SASL XOAUTH2.
	AuthOAuth2
	// AuthLogin uses SASL LOGIN.
	AuthLogin
)

// AuthTypes is the enum helper for populating the [Config.Auth] field.
var AuthTypes = &authTypeVals{
	None:    AuthNone,
	Plain:   AuthPlain,
	CRAMMD5: AuthCRAMMD5,
	Unknown: AuthUnknown,
	OAuth2:  AuthOAuth2,
	Login:   AuthLogin,
	Enum: format.CreateEnumFormatter(
		[]string{
			"Unknown",
			"None",
			"Plain",
			"CRAMMD5",
			"OAuth2",
			"Login",
		},
	),
}

// String returns the authentication method name.
//
// Returns:
//   - The canonical name of the authentication method, such as "Plain" or "OAuth2".
func (at authType) String() string {
	return AuthTypes.Enum.Print(int(at))
}
