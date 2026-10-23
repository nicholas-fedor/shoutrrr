package telegram

import "regexp"

// IsTokenValid reports whether a bot token has the format Telegram issues: the
// numeric bot ID, a colon, and a secret of letters, digits, underscores, and
// dashes.
//
// Parameters:
//   - token: the bot token.
//
// Returns:
//   - bool: whether the token has a valid format.
func IsTokenValid(token string) bool {
	matched, err := regexp.MatchString("^[0-9]+:[a-zA-Z0-9_-]+$", token)

	return matched && err == nil
}
