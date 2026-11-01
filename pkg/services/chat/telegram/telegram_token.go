package telegram

import "strings"

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
	botID, secret, found := strings.Cut(token, ":")
	if !found || botID == "" || secret == "" {
		return false
	}

	for i := range len(botID) {
		if botID[i] < '0' || botID[i] > '9' {
			return false
		}
	}

	for i := range len(secret) {
		switch c := secret[i]; {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}

	return true
}
