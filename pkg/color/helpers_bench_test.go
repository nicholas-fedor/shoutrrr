package color

import (
	"bytes"
	"testing"
)

// Benchmark_colorPrint benchmarks the colorPrint helper function with color enabled.
func Benchmark_colorPrint(b *testing.B) {
	// Save and restore
	originalNoColor := NoColor
	originalOutput := Output

	b.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	NoColor = false
	buf := &bytes.Buffer{}
	Output = buf

	b.ResetTimer()

	for i := range b.N {
		buf.Reset()
		colorPrint("test %d", FgRed, i)
	}
}

// Benchmark_colorString benchmarks the colorString helper function with color enabled.
func Benchmark_colorString(b *testing.B) {
	// Save and restore
	originalNoColor := NoColor

	b.Cleanup(func() {
		NoColor = originalNoColor
	})

	NoColor = false

	b.ResetTimer()

	for i := range b.N {
		_ = colorString("test %d", FgRed, i)
	}
}

// Benchmark_colorString_NoColor benchmarks the colorString helper function with NoColor enabled.
func Benchmark_colorString_NoColor(b *testing.B) {
	// Save and restore
	originalNoColor := NoColor

	b.Cleanup(func() {
		NoColor = originalNoColor
	})

	NoColor = true

	b.ResetTimer()

	for i := range b.N {
		_ = colorString("test %d", FgRed, i)
	}
}
