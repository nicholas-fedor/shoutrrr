package zulip

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// hostNamePattern defines a host name as a regular expression, the reference
// that isHostName must agree with.
var hostNamePattern = regexp.MustCompile(
	`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`,
)

// FuzzIsHostName checks that isHostName accepts exactly the names that
// hostNamePattern matches. Seeds hold host names with one or more labels and
// labels at the length limit. Others hold empty labels, hyphens at a label's
// edge, and characters a host name cannot hold.
func FuzzIsHostName(f *testing.F) {
	label63 := "a" + strings.Repeat("b", 61) + "c"

	for _, seed := range []string{
		"", "zulip", "zulip.example.com", "chat.zulip-cloud.example.co", label63, label63 + "d", "a.", ".a", "a..b",
		"-a", "a-", "a_b", "zulip example.com", "été.example.com", "192.0.2.10",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, name string) {
		assert.Equal(t, hostNamePattern.MatchString(name), isHostName(name), "%q", name)
	})
}
