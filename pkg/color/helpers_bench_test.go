package color

import (
	"testing"
)

// BenchmarkColorPrint benchmarks the Color.Print method for printing colored output.
func BenchmarkColorPrint(b *testing.B) {
	cfg := &Config{
		NoColor: false,
		Output:  nil,
		Error:   nil,
	}

	c := NewWithConfig(cfg, FgRed)

	b.ResetTimer()

	for b.Loop() {
		_, _ = c.Print("test message")
	}
}

// BenchmarkColorSprint benchmarks the Color.Sprint method for returning colored strings.
func BenchmarkColorSprint(b *testing.B) {
	cfg := &Config{
		NoColor: false,
		Output:  nil,
		Error:   nil,
	}

	c := NewWithConfig(cfg, FgRed)

	b.ResetTimer()

	for b.Loop() {
		_ = c.Sprint("test message")
	}
}

// BenchmarkColorSprintNoColor benchmarks the Color.Sprint method with NoColor enabled.
func BenchmarkColorSprintNoColor(b *testing.B) {
	cfg := &Config{
		NoColor: true,
		Output:  nil,
		Error:   nil,
	}

	c := NewWithConfig(cfg, FgRed)

	b.ResetTimer()

	for b.Loop() {
		_ = c.Sprint("test message")
	}
}
