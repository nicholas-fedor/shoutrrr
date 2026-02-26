package color

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_colorPrint(t *testing.T) {
	// Save original NoColor state and restore after test
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name        string
		format      string
		attribute   Attribute
		args        []any
		setNoColor  bool
		checkOutput func(*testing.T, string)
	}{
		{
			name:       "prints with foreground color red",
			format:     "hello",
			attribute:  FgRed,
			args:       nil,
			setNoColor: false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				require.NotEmpty(t, output)
				assert.Contains(t, output, "hello")
			},
		},
		{
			name:       "prints with foreground color green",
			format:     "world",
			attribute:  FgGreen,
			args:       nil,
			setNoColor: false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				require.NotEmpty(t, output)
				assert.Contains(t, output, "world")
			},
		},
		{
			name:       "prints with format and args",
			format:     "value: %d",
			attribute:  FgBlue,
			args:       []any{42},
			setNoColor: false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				require.NotEmpty(t, output)
				assert.Contains(t, output, "value: 42")
			},
		},
		{
			name:       "prints nothing when NoColor is set",
			format:     "test",
			attribute:  FgRed,
			args:       nil,
			setNoColor: true,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				// When NoColor is set, the color codes should not be present
				assert.NotContains(t, output, "\x1b[")
			},
		},
		{
			name:       "adds newline if not present",
			format:     "test",
			attribute:  FgRed,
			args:       nil,
			setNoColor: false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.NotEmpty(t, output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			buf := &bytes.Buffer{}
			oldOutput := Output
			Output = buf

			t.Cleanup(func() {
				Output = oldOutput
			})

			colorPrint(tt.format, tt.attribute, tt.args...)

			output := buf.String()

			tt.checkOutput(t, output)
		})
	}
}

func Test_colorString(t *testing.T) {
	// Save original NoColor state and restore after test
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		format     string
		attribute  Attribute
		args       []any
		setNoColor bool
		want       string
	}{
		{
			name:       "returns colored string for red",
			format:     "hello",
			attribute:  FgRed,
			args:       nil,
			setNoColor: false,
			want:       "",
		},
		{
			name:       "returns colored string for green",
			format:     "world",
			attribute:  FgGreen,
			args:       nil,
			setNoColor: false,
			want:       "",
		},
		{
			name:       "returns colored string with format args",
			format:     "value: %d",
			attribute:  FgBlue,
			args:       []any{42},
			setNoColor: false,
			want:       "",
		},
		{
			name:       "returns plain string when NoColor is set",
			format:     "test",
			attribute:  FgRed,
			args:       nil,
			setNoColor: true,
			want:       "test",
		},
		{
			name:       "returns plain string with args when NoColor is set",
			format:     "value: %d",
			attribute:  FgRed,
			args:       []any{123},
			setNoColor: true,
			want:       "value: 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			result := colorString(tt.format, tt.attribute, tt.args...)

			if tt.setNoColor {
				// When NoColor is set, result should be plain
				assert.Equal(t, tt.want, result)
			} else {
				// When color is enabled, result should contain escape codes
				require.NotEmpty(t, result)

				if tt.args == nil {
					assert.Contains(t, result, tt.format)
				} else {
					assert.Contains(t, result, "value: 42")
				}
			}
		})
	}
}

// Test helper functions with various color attributes.
func Test_colorPrint_variousAttributes(t *testing.T) {
	// Save and restore
	originalNoColor := NoColor
	originalOutput := Output

	t.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	NoColor = false
	buf := &bytes.Buffer{}
	Output = buf

	attributes := []Attribute{
		FgBlack, FgRed, FgGreen, FgYellow, FgBlue, FgMagenta, FgCyan, FgWhite,
		FgHiBlack, FgHiRed, FgHiGreen, FgHiYellow, FgHiBlue, FgHiMagenta, FgHiCyan, FgHiWhite,
		BgBlack, BgRed, BgGreen, BgYellow, BgBlue, BgMagenta, BgCyan, BgWhite,
		Bold, Faint, Italic, Underline, BlinkSlow, ReverseVideo, Concealed, CrossedOut,
	}

	for _, attr := range attributes {
		buf.Reset()
		colorPrint("test", attr)

		output := buf.String()
		assert.NotEmpty(t, output, "expected output for attribute %v", attr)
	}
}

func Test_colorString_variousAttributes(t *testing.T) {
	// Save and restore
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	NoColor = false

	attributes := []Attribute{
		FgBlack, FgRed, FgGreen, FgYellow, FgBlue, FgMagenta, FgCyan, FgWhite,
		Bold, Italic, Underline,
	}

	for _, attr := range attributes {
		result := colorString("test", attr)
		assert.NotEmpty(t, result, "expected output for attribute %v", attr)
		assert.Contains(t, result, "test", "result should contain the input string for attribute %v", attr)
	}
}
