package color

import (
	"testing"
)

// BenchmarkNew benchmarks the creation of new Color instances.
func BenchmarkNew(b *testing.B) {
	for b.Loop() {
		_ = New(FgRed, Bold)
	}
}

// BenchmarkColor_Add benchmarks the Add method for chaining SGR parameters.
func BenchmarkColor_Add(b *testing.B) {
	c := New()
	for b.Loop() {
		_ = c.Add(FgRed, Bold)
	}
}

// BenchmarkColor_Sprint benchmarks the Sprint method for converting colored strings.
func BenchmarkColor_Sprint(b *testing.B) {
	cfg := &Config{
		NoColor: false,
		Output:  nil,
		Error:   nil,
	}

	c := NewWithConfig(cfg, FgRed)
	for b.Loop() {
		_ = c.Sprint("hello")
	}
}

// BenchmarkColor_Sprintf benchmarks the Sprintf method for formatted colored strings.
func BenchmarkColor_Sprintf(b *testing.B) {
	cfg := &Config{
		NoColor: false,
		Output:  nil,
		Error:   nil,
	}

	c := NewWithConfig(cfg, FgRed)
	for b.Loop() {
		_ = c.Sprintf("value: %d", 42)
	}
}

// BenchmarkColor_sequence benchmarks the sequence method for generating ANSI codes.
func BenchmarkColor_sequence(b *testing.B) {
	c := New(FgRed, Bold, Underline)
	for b.Loop() {
		_ = c.sequence()
	}
}

// BenchmarkColor_format benchmarks the format method for generating full ANSI escape sequences.
func BenchmarkColor_format(b *testing.B) {
	cfg := &Config{
		NoColor: false,
		Output:  nil,
		Error:   nil,
	}

	c := NewWithConfig(cfg, FgRed, Bold)
	for b.Loop() {
		_ = c.format()
	}
}
