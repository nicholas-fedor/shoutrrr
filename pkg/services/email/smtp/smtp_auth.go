package smtp

import (
	"net/smtp"
)

// newAuth returns the SMTP authentication mechanism for config.
//
// Auth [AuthNone] returns a nil mechanism and a nil error so the session
// continues without AUTH.
//
// Parameters:
//   - config: SMTP configuration providing Auth, Username, Password, and Host.
//
// Returns:
//   - An [smtp.Auth] implementation for the selected method, or nil when Auth is [AuthNone].
//   - An error if Auth is [AuthUnknown] or otherwise unsupported.
//
//nolint:exhaustive // false positive: switch covers all AuthTypes, linter confuses local authType with net/smtp.authType
func newAuth(config *Config) (smtp.Auth, error) {
	switch config.Auth {
	case AuthTypes.None:
		return nil, nil //nolint:nilnil // AuthNone means skip AUTH; a nil mechanism is the success path.
	case AuthTypes.Plain:
		return smtp.PlainAuth("", config.Username, config.Password, config.Host), nil
	case AuthTypes.CRAMMD5:
		return smtp.CRAMMD5Auth(config.Username, config.Password), nil
	case AuthTypes.OAuth2:
		return newOAuth2Auth(config.Username, config.Password, config.Host), nil
	case AuthTypes.Login:
		return newLoginAuth(config.Username, config.Password, config.Host), nil
	case AuthTypes.Unknown:
		return nil, fail(FailAuthType, nil, config.Auth.String())
	default:
		return nil, fail(FailAuthType, nil, config.Auth.String())
	}
}
