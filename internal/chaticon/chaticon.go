package chaticon

import "encoding/binary"

// IsURL reports whether an icon is an image URL rather than an emoji name. An
// icon is a URL when it starts with "http://" or "https://", with each scheme
// letter in either ASCII case.
//
// Parameters:
//   - icon: the icon from a service's config.
//
// Returns:
//   - bool: true when the icon is an http or https URL.
func IsURL(icon string) bool {
	const (
		// caseBit is set in a lower-case ASCII letter and clear in its upper
		// case. Setting it in any other byte never gives h, t, p, or s, so the
		// scheme compares in ASCII case only.
		caseBit = 0x20
		// caseBits sets caseBit in each byte of a little-endian word.
		caseBits = caseBit | caseBit<<8 | caseBit<<16 | caseBit<<24
		// httpWord is "http" read as a little-endian word.
		httpWord = 'h' | 't'<<8 | 't'<<16 | 'p'<<24
	)

	if len(icon) < len("http://") {
		return false
	}

	if binary.LittleEndian.Uint32([]byte(icon[:4]))|caseBits != httpWord {
		return false
	}

	rest := icon[len("http"):]
	if rest[0]|caseBit == 's' {
		rest = rest[1:]
	}

	// A length check and slice compare cost less inlining budget than
	// strings.HasPrefix, which keeps each SetIcon that calls IsURL small enough
	// to inline into its payload builder.
	return len(rest) >= len("://") && rest[:len("://")] == "://"
}
