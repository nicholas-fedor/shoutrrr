package signal

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// usernamePrefix marks a Signal username recipient.
	usernamePrefix = "u:"
)

// phoneRegex validates phone number format (with or without + prefix).
var phoneRegex = regexp.MustCompile(`^\+?[0-9\s)(+-]+$`)

// groupRegex validates group ID format.
var groupRegex = regexp.MustCompile(`^group\.[a-zA-Z0-9_+/=-]+$`)

// parseRecipients parses recipient phone numbers, group IDs, and usernames from URL path segments.
// It handles group IDs that may contain "/" characters by accumulating consecutive segments.
//
// Parameters:
//   - pathParts: the URL path segments to parse
//
// Returns:
//   - []string: the parsed recipients
//   - error: if parsing fails, nil otherwise
func parseRecipients(pathParts []string) ([]string, error) {
	if len(pathParts) == 0 {
		return nil, ErrNoRecipients
	}

	var (
		recipients     []string
		currentGroupID strings.Builder
	)

	inGroupID := false

	for _, part := range pathParts {
		switch {
		case strings.HasPrefix(part, "group."):
			if inGroupID {
				recipients = append(recipients, currentGroupID.String())
			}

			currentGroupID.Reset()
			currentGroupID.WriteString(part)

			inGroupID = true

		case inGroupID && !strings.HasPrefix(part, "+") && !isValidUsername(part):
			currentGroupID.WriteString("/")
			currentGroupID.WriteString(part)

		default:
			if inGroupID {
				recipients = append(recipients, currentGroupID.String())
				inGroupID = false
			}

			recipients = append(recipients, part)
		}
	}

	if inGroupID {
		recipients = append(recipients, currentGroupID.String())
	}

	for _, recipient := range recipients {
		if !isValidRecipient(recipient) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRecipient, recipient)
		}
	}

	return recipients, nil
}

// isValidPhoneNumber checks if the string is a valid phone number.
//
// Parameters:
//   - phone: the phone number string to validate
//
// Returns:
//   - bool: true if valid, false otherwise
func isValidPhoneNumber(phone string) bool {
	return phoneRegex.MatchString(phone)
}

// isValidGroupID checks if the string is a valid group ID.
//
// Parameters:
//   - groupID: the group ID string to validate
//
// Returns:
//   - bool: true if valid, false otherwise
func isValidGroupID(groupID string) bool {
	return groupRegex.MatchString(groupID)
}

// isValidUsername checks if the string is a Signal username recipient.
//
// Parameters:
//   - username: the username string to validate
//
// Returns:
//   - bool: true if it has a u: prefix and a non-empty remainder
func isValidUsername(username string) bool {
	rest, ok := strings.CutPrefix(username, usernamePrefix)

	return ok && rest != ""
}

// isValidRecipient reports whether the string is a phone number, group ID, or username.
//
// Parameters:
//   - recipient: the recipient string to validate
//
// Returns:
//   - bool: true if valid, false otherwise
func isValidRecipient(recipient string) bool {
	return isValidPhoneNumber(recipient) || isValidGroupID(recipient) || isValidUsername(recipient)
}
