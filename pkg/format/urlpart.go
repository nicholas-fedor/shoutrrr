//go:generate stringer -type=URLPart -trimprefix URL

package format

import (
	"log"
	"strconv"
	"strings"
)

// URLPart is an indicator as to what part of an URL a field is serialized to.
type URLPart int

// indicator as to what part of an URL a field is serialized to.
const (
	URLQuery URLPart = iota
	URLUser
	URLPassword
	URLHost
	URLPort
	URLPath // Base path; additional paths are URLPath + N
)

// Suffix returns the separator between the URLPart and its subsequent part.
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

// ParseURLPart returns the URLPart that matches the supplied string.
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

	log.Printf("invalid URLPart: %s, defaulting to URLQuery", lowerString)

	return URLQuery
}

// ParseURLParts returns the URLParts that matches the supplied string.
func ParseURLParts(s string) []URLPart {
	rawParts := strings.Split(s, ",")
	urlParts := make([]URLPart, len(rawParts))

	for i, raw := range rawParts {
		urlParts[i] = ParseURLPart(raw)
	}

	return urlParts
}
