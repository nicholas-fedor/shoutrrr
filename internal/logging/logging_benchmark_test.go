package logging

import (
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// BenchmarkOnceLogged measures the warnings a send repeats after the first.
//
// It covers an insecure warning, a deprecation, and a warning keyed for Do
// that were already logged. It also covers the sync of a config with no
// deprecated field set, which most sends make.
func BenchmarkOnceLogged(b *testing.B) {
	var once Once

	logger := &recordingLogger{}
	keys := []format.Deprecation{{Name: "insecureskipverify", Replacement: "skiptlsverify"}}

	once.Insecure(b.Context(), logger)
	once.DeprecatedKeys(b.Context(), logger, keys)
	once.Do("warning", func() {})

	unset := &deprecatedConfig{SkipTLSVerify: true, Insecure: false}

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

	b.Run("do", func(b *testing.B) {
		for b.Loop() {
			once.Do("warning", func() {})
		}
	})

	b.Run("no deprecations", func(b *testing.B) {
		for b.Loop() {
			_ = once.SyncDeprecatedFields(b.Context(), logger, unset)
		}
	})
}
