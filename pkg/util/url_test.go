package util

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:gosmopolitan // Unicode characters used intentionally for testing user/password handling
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
