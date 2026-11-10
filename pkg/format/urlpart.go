//go:generate stringer -type=URLPart -trimprefix URL

package format

import (
	"strconv"
	"strings"
)

// URLPart is an indicator as to what part of an URL a field is serialized to.
type URLPart int

// The parts of a URL that a field can be serialized to.
//
// URLQuery comes first as the zero value, and the parts after it follow URL
// order from the user to the path.
const (
	// URLQuery is the query, where every field without a URL part goes.
	URLQuery URLPart = iota
	// URLUser is the user name of the userinfo.
	URLUser
	// URLPassword is the password of the userinfo.
	URLPassword
	// URLHost is the host name.
	URLHost
	// URLPort is the port.
	URLPort
	// URLPath is the first path segment. Each later segment is URLPath plus
	// its offset.
	URLPath
)

// Suffix returns the separator between the URLPart and its subsequent part.
//
// Returns:
//   - rune: ':' after the user and the host, '@' after the password, and '/'
//     after any other part.
func (u URLPart) Suffix() rune {
	switch u {
	case URLQuery:
		return '/'
	case URLUser:
		return ':'
	case URLPassword:
		return '@'
	case URLHost:
		return ':'
	case URLPort:
		return '/'
	case URLPath:
		return '/'
	default:
		return '/'
	}
}

// ParseURLPart returns the URLPart that matches the supplied string, ignoring
// case.
//
// A name that matches no part of a URL is the query, as are "query" and the
// empty string.
//
// Parameters:
//   - inputString: the part name, such as "host", "path", or "path2".
//
// Returns:
//   - URLPart: the matching part, or [URLQuery] for any other name.
func ParseURLPart(inputString string) URLPart {
	lowerString := strings.ToLower(inputString)
	switch lowerString {
	case "user":
		return URLUser
	case "pass", "password":
		return URLPassword
	case "host":
		return URLHost
	case "port":
		return URLPort
	case "path", "path1":
		return URLPath
	case "query", "":
		return URLQuery
	}

	// Handle dynamic path segments (e.g., "path2", "path3", etc.).
	if strings.HasPrefix(lowerString, "path") && len(lowerString) > 4 {
		if num, err := strconv.Atoi(lowerString[4:]); err == nil && num >= 2 {
			return URLPath + URLPart(num-1) // Offset from URLPath; "path2" -> URLPath+1
		}
	}

	return URLQuery
}

// ParseURLParts returns the URLParts that a comma-separated list names.
//
// Parameters:
//   - s: the part names, separated by commas, as in a `url` struct tag.
//
// Returns:
//   - []URLPart: one part for each name, as [ParseURLPart] reads it.
func ParseURLParts(s string) []URLPart {
	rawParts := strings.Split(s, ",")
	urlParts := make([]URLPart, len(rawParts))

	for i, raw := range rawParts {
		urlParts[i] = ParseURLPart(raw)
	}

	return urlParts
}
