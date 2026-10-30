package logging

import (
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// BenchmarkOnceLogged measures the warnings a send repeats after the first.
// It covers an insecure warning and a deprecation that were already logged,
// and the empty deprecation list that most sends pass.
func BenchmarkOnceLogged(b *testing.B) {
	var once Once

	logger := &recordingLogger{}
	keys := []format.Deprecation{{Name: "insecureskipverify", Replacement: "skiptlsverify"}}

	once.Insecure(b.Context(), logger)
	once.DeprecatedKeys(b.Context(), logger, keys)

	b.Run("insecure", func(b *testing.B) {
		for b.Loop() {
			once.Insecure(b.Context(), logger)
		}
	})

	b.Run("deprecated key", func(b *testing.B) {
		for b.Loop() {
			once.DeprecatedKeys(b.Context(), logger, keys)
		}
	})

	b.Run("no deprecations", func(b *testing.B) {
		for b.Loop() {
			once.DeprecatedFields(b.Context(), logger, nil)
		}
	})
}
