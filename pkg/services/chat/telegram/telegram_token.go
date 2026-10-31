package telegram

import "regexp"

// tokenPattern matches the format of a Telegram bot token, which
// [IsTokenValid] describes.
var tokenPattern = regexp.MustCompile(`^\d+:[a-zA-Z0-9_-]+$`)

// IsTokenValid reports whether a bot token has the format Telegram issues. A
// token is the numeric bot ID and a secret joined by a colon. The secret holds
// letters, digits, underscores, and dashes.
//
// Parameters:
//   - token: the bot token.
//
// Returns:
//   - bool: whether the token has a valid format.
func IsTokenValid(token string) bool {
	return tokenPattern.MatchString(token)
}
