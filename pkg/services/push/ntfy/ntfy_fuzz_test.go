package ntfy

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// serverTopicPattern is the topic name rule of ntfy's server
// (server/server.go), which isTopicName must match.
var serverTopicPattern = regexp.MustCompile(`^[-_A-Za-z0-9]{1,64}$`)

// FuzzIsTopicName checks that isTopicName accepts a topic name exactly when
// ntfy's server rule does. Seeds hold valid names that cover each allowed
// character and the 64-byte maximum. Others hold a name one byte too long and
// an empty name. The rest hold a byte the rule rejects, such as a slash or a
// space. A trailing newline and a character outside ASCII are among them.
func FuzzIsTopicName(f *testing.F) {
	for _, seed := range []string{
		"my-topic_2", "A", strings.Repeat("a", 64), strings.Repeat("a", 65),
		"", "a/b", "a b", "alerts\n", "café",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, name string) {
		assert.Equal(t, serverTopicPattern.MatchString(name), isTopicName(name), "%q", name)
	})
}
