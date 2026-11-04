package util

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLUserPassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		user         string
		password     string
		wantUsername string
		wantPassword string
		wantNil      bool
	}{
		{
			name:         "returns user with password when both provided",
			user:         "admin",
			password:     "secret",
			wantUsername: "admin",
			wantPassword: "secret",
			wantNil:      false,
		},
		{
			name:         "returns user without password when only user provided",
			user:         "admin",
			password:     "",
			wantUsername: "admin",
			wantPassword: "",
			wantNil:      false,
		},
		{
			name:         "returns nil when neither user nor password provided",
			user:         "",
			password:     "",
			wantUsername: "",
			wantPassword: "",
			wantNil:      true,
		},
		{
			name:         "returns user with password when user is empty but password is provided",
			user:         "",
			password:     "secret",
			wantUsername: "",
			wantPassword: "secret",
			wantNil:      false,
		},
		{
			name:         "handles special characters in credentials",
			user:         "user@domain.com",
			password:     "p@ssw0rd!#$%",
			wantUsername: "user@domain.com",
			wantPassword: "p@ssw0rd!#$%",
			wantNil:      false,
		},
		{
			name:         "handles unicode characters",
			user:         "用户",
			password:     "密码",
			wantUsername: "用户",
			wantPassword: "密码",
			wantNil:      false,
		},
		{
			name:         "handles very long credentials",
			user:         "averylongusernamethatcouldbeused",
			password:     "anextremelylongpasswordthatnobodyshouldusebutwestilltest",
			wantUsername: "averylongusernamethatcouldbeused",
			wantPassword: "anextremelylongpasswordthatnobodyshouldusebutwestilltest",
			wantNil:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := URLUserPassword(tt.user, tt.password)

			if tt.wantNil {
				assert.Nil(t, got, "URLUserPassword(%q, %q) should return nil", tt.user, tt.password)
			} else {
				assert.NotNil(t, got, "URLUserPassword(%q, %q) should not return nil", tt.user, tt.password)
				assert.Equal(t, tt.wantUsername, got.Username(), "Username mismatch")
				gotPassword, _ := got.Password()
				assert.Equal(t, tt.wantPassword, gotPassword, "Password mismatch")
			}
		})
	}
}

func TestURLUserPassword_Integration(t *testing.T) {
	t.Parallel()

	// Verify the returned Userinfo works correctly with url.URL
	t.Run("works with url.URL", func(t *testing.T) {
		t.Parallel()

		userinfo := URLUserPassword("admin", "secret")
		testURL := &url.URL{
			Scheme: "https",
			Host:   "example.com",
			User:   userinfo,
			Path:   "/path",
		}

		expected := "https://admin:secret@example.com/path"
		assert.Equal(t, expected, testURL.String(), "URL with userinfo should serialize correctly")
	})

	t.Run("nil userinfo serializes to empty string", func(t *testing.T) {
		t.Parallel()

		userinfo := URLUserPassword("", "")
		testURL := &url.URL{
			Scheme: "https",
			Host:   "example.com",
			User:   userinfo,
			Path:   "/path",
		}

		expected := "https://example.com/path"
		assert.Equal(t, expected, testURL.String(), "URL with nil userinfo should not include @")
	})
}

// TestSplitURLHost verifies that SplitURLHost returns the hostname without
// brackets and the port of a URL's authority, and rejects an unbracketed host
// with several colons and a port that is not a number, which url.Parse either
// splits at the wrong colon or only lets through in a hand-built URL.
func TestSplitURLHost(t *testing.T) {
	t.Parallel()

	// tests lists URL authorities with the hostname and port, or error,
	// SplitURLHost should return.
	tests := []struct {
		wantErr  error
		name     string
		host     string
		wantHost string
		wantPort string
	}{
		{name: "empty", host: ""},
		{name: "hostname", host: "example.com", wantHost: "example.com"},
		{name: "hostname and port", host: "example.com:8080", wantHost: "example.com", wantPort: "8080"},
		{name: "hostname and empty port", host: "example.com:", wantHost: "example.com"},
		{name: "port only", host: ":8080", wantPort: "8080"},
		{name: "IPv4 and port", host: "192.0.2.1:25", wantHost: "192.0.2.1", wantPort: "25"},
		{name: "bracketed IPv6", host: "[::1]", wantHost: "::1"},
		{name: "bracketed IPv6 and port", host: "[2001:db8::1]:8443", wantHost: "2001:db8::1", wantPort: "8443"},
		{name: "bracketed IPv6 and empty port", host: "[::1]:", wantHost: "::1"},
		{name: "bracketed IPv6 with a zone", host: "[fe80::1%en0]:80", wantHost: "fe80::1%en0", wantPort: "80"},
		{name: "unbracketed IPv6", host: "::1", wantErr: ErrInvalidHost},
		{name: "unbracketed IPv6 and port", host: "2001:db8::1:8443", wantErr: ErrInvalidHost},
		{name: "unclosed bracket", host: "[::1", wantErr: ErrInvalidHost},
		{name: "text after the bracket", host: "[::1]x", wantErr: ErrInvalidHost},
		{name: "non-numeric port", host: "example.com:abc", wantErr: ErrInvalidPort},
		{name: "non-numeric port after IPv6", host: "[::1]:abc", wantErr: ErrInvalidPort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			host, port, err := SplitURLHost(&url.URL{Host: tt.host})

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantPort, port)
		})
	}
}

// TestParseURLPort verifies that a URL port reads as its number and an empty
// port as 0. A port outside 1 to 65535 or not a number is rejected.
func TestParseURLPort(t *testing.T) {
	t.Parallel()

	// tests lists ports with the number each reads as.
	tests := []struct {
		port string
		want uint16
	}{
		{port: "", want: 0},
		{port: "1", want: 1},
		{port: "8080", want: 8080},
		{port: "65535", want: 65535},
	}

	for _, tt := range tests {
		t.Run(tt.port, func(t *testing.T) {
			t.Parallel()

			got, err := ParseURLPort(tt.port)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	for _, port := range []string{"0", "65536", "99999", "-1", "port"} {
		t.Run("invalid "+port, func(t *testing.T) {
			t.Parallel()

			_, err := ParseURLPort(port)

			require.ErrorIs(t, err, ErrInvalidPort)
		})
	}
}

// TestValidatePathSegment verifies that a value is accepted as one path
// segment unless it is empty, holds a slash, or is "." or "..". Characters that
// escaping keeps in the segment, such as a question mark, are accepted.
func TestValidatePathSegment(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"token", "a?b", "a#b", "a%zz", "...", ".a"} {
		require.NoError(t, ValidatePathSegment(value), value)
	}

	for _, value := range []string{"", ".", "..", "a/b", "/", "../x"} {
		require.ErrorIs(t, ValidatePathSegment(value), ErrInvalidPathSegment, value)
	}
}

// TestValidatePathSegments verifies that a path is accepted unless one of its
// segments is "." or "..". Empty segments and names that only start or end
// with a dot are accepted.
func TestValidatePathSegments(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"", "/", "base", "/base/path/", "a//b", ".a/b.", "...", "a?b#c"} {
		require.NoError(t, ValidatePathSegments(path), path)
	}

	for _, path := range []string{".", "..", "/..", "../x", "a/./b", "a/../b", "a/..", "/./"} {
		require.ErrorIs(t, ValidatePathSegments(path), ErrInvalidPathSegment, path)
	}
}

// TestValidateHostToken verifies that a token is accepted unless it would not
// read back unchanged from a URL host. A colon or a bracket reads back as a
// port or an IPv6 address. A slash, a space, or @ makes the URL fail to parse.
func TestValidateHostToken(t *testing.T) {
	t.Parallel()

	for _, token := range []string{"token", "abc-123_XYZ", "", "+15551234567", "o.a~b%c", "\u00e9t\u00e9"} {
		require.NoError(t, ValidateHostToken(token), token)
	}

	for _, token := range []string{"token:443", "[token]", "a[b", "a]b", ":", "a/b", "a b", "a@b", "a?b", "a#b", "a\x00b"} {
		require.ErrorIs(t, ValidateHostToken(token), ErrInvalidHost, token)
	}
}

// TestJoinURLHost verifies that JoinURLHost writes a hostname and optional
// port as a URL authority, with an IPv6 hostname in brackets whether or not a
// port follows it, and that SplitURLHost reads the authority back.
func TestJoinURLHost(t *testing.T) {
	t.Parallel()

	// tests lists hostnames and ports with the authority JoinURLHost should
	// write.
	tests := []struct {
		name string
		host string
		port string
		want string
	}{
		{name: "hostname", host: "example.com", want: "example.com"},
		{name: "hostname and port", host: "example.com", port: "8080", want: "example.com:8080"},
		{name: "port only", port: "8080", want: ":8080"},
		{name: "empty", want: ""},
		{name: "IPv6", host: "::1", want: "[::1]"},
		{name: "IPv6 and port", host: "2001:db8::1", port: "8443", want: "[2001:db8::1]:8443"},
		{name: "IPv6 with a zone", host: "fe80::1%en0", port: "80", want: "[fe80::1%en0]:80"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := JoinURLHost(tt.host, tt.port)
			assert.Equal(t, tt.want, got)

			host, port, err := SplitURLHost(&url.URL{Host: got})
			require.NoError(t, err)
			assert.Equal(t, tt.host, host)
			assert.Equal(t, tt.port, port)
		})
	}
}

// TestParseURLQuery verifies that ParseURLQuery returns the query values of a
// URL and rejects a malformed escape or a semicolon separator, which
// url.URL.Query drops without an error.
func TestParseURLQuery(t *testing.T) {
	t.Parallel()

	// tests lists raw queries with the values, or error, ParseURLQuery should
	// return.
	tests := []struct {
		want     url.Values
		wantErr  error
		name     string
		rawQuery string
	}{
		{name: "empty", rawQuery: "", want: url.Values{}},
		{name: "values", rawQuery: "a=1&b=x%20y&a=2", want: url.Values{"a": {"1", "2"}, "b": {"x y"}}},
		{name: "key without a value", rawQuery: "a", want: url.Values{"a": {""}}},
		{name: "malformed escape in a value", rawQuery: "a=1&b=%zz", wantErr: ErrInvalidQuery},
		{name: "malformed escape in a key", rawQuery: "%zz=1", wantErr: ErrInvalidQuery},
		{name: "semicolon separator", rawQuery: "a=1;b=2", wantErr: ErrInvalidQuery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			query, err := ParseURLQuery(&url.URL{RawQuery: tt.rawQuery})

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, query)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, query)
		})
	}
}
