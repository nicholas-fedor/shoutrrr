package color

import (
	"os"
	"testing"
)

// Benchmark_boolPtr benchmarks the boolPtr function for creating boolean pointers.
func Benchmark_boolPtr(b *testing.B) {
	for b.Loop() {
		_ = boolPtr(true)
		_ = boolPtr(false)
	}
}

// Benchmark_sprintln benchmarks the sprintln function for joining arguments with spaces.
func Benchmark_sprintln(b *testing.B) {
	args := []any{"hello", "world", 123}

	for b.Loop() {
		_ = sprintln(args...)
	}
}

// Benchmark_noColorIsSet benchmarks the noColorIsSet function for checking NO_COLOR environment variable.
func Benchmark_noColorIsSet(b *testing.B) {
	// Save original environment
	originalEnv := os.Getenv("NO_COLOR")

	os.Unsetenv("NO_COLOR")
	b.Cleanup(func() {
		if originalEnv == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", originalEnv)
		}
	})

	for b.Loop() {
		_ = noColorIsSet()
	}
}
