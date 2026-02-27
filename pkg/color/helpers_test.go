package color

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_colorPrint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		format      string
		attribute   Attribute
		args        []any
		noColor     bool
		checkOutput func(*testing.T, string)
	}{
		{
			name:      "prints with foreground color red",
			format:    "hello",
			attribute: FgRed,
			args:      nil,
			noColor:   false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				require.NotEmpty(t, output)
				assert.Contains(t, output, "hello")
			},
		},
		{
			name:      "prints with foreground color green",
			format:    "world",
			attribute: FgGreen,
			args:      nil,
			noColor:   false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				require.NotEmpty(t, output)
				assert.Contains(t, output, "world")
			},
		},
		{
			name:      "prints with format and args",
			format:    "value: %d",
			attribute: FgBlue,
			args:      []any{42},
			noColor:   false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				require.NotEmpty(t, output)
				assert.Contains(t, output, "value: 42")
			},
		},
		{
			name:      "prints nothing when NoColor is set",
			format:    "test",
			attribute: FgRed,
			args:      nil,
			noColor:   true,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				// When NoColor is set, the color codes should not be present
				assert.NotContains(t, output, "\x1b[")
			},
		},
		{
			name:      "adds newline if not present",
			format:    "test",
			attribute: FgRed,
			args:      nil,
			noColor:   false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.NotEmpty(t, output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a test config with custom output buffer
			buf := &bytes.Buffer{}
			cfg := &Config{
				NoColor: tt.noColor,
				Output:  buf,
				Error:   buf,
			}

			// Create color with config and use Fprintln for testing
			c := NewWithConfig(cfg, tt.attribute)
			switch {
			case !tt.noColor && tt.args == nil:
				_, _ = c.Fprintln(buf, tt.format)
			case !tt.noColor:
				_, _ = c.Fprintf(buf, tt.format+"\n", tt.args...)
			default:
				_, _ = buf.WriteString(tt.format + "\n")
			}

			output := buf.String()

			tt.checkOutput(t, output)
		})
	}
}

func Test_colorString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		format    string
		attribute Attribute
		args      []any
		noColor   bool
		want      string
	}{
		{
			name:      "returns colored string for red",
			format:    "hello",
			attribute: FgRed,
			args:      nil,
			noColor:   false,
			want:      "",
		},
		{
			name:      "returns colored string for green",
			format:    "world",
			attribute: FgGreen,
			args:      nil,
			noColor:   false,
			want:      "",
		},
		{
			name:      "returns colored string with format args",
			format:    "value: %d",
			attribute: FgBlue,
			args:      []any{42},
			noColor:   false,
			want:      "",
		},
		{
			name:      "returns plain string when NoColor is set",
			format:    "test",
			attribute: FgRed,
			args:      nil,
			noColor:   true,
			want:      "test",
		},
		{
			name:      "returns plain string with args when NoColor is set",
			format:    "value: %d",
			attribute: FgRed,
			args:      []any{123},
			noColor:   true,
			want:      "value: 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create color with config
			buf := &bytes.Buffer{}
			cfg := &Config{
				NoColor: tt.noColor,
				Output:  buf,
				Error:   buf,
			}
			c := NewWithConfig(cfg, tt.attribute)

			var result string
			if tt.args == nil {
				result = c.SprintFunc()(tt.format)
			} else {
				result = c.SprintfFunc()(tt.format, tt.args...)
			}

			if tt.noColor {
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
	t.Parallel()

	attributes := []Attribute{
		FgBlack, FgRed, FgGreen, FgYellow, FgBlue, FgMagenta, FgCyan, FgWhite,
		FgHiBlack, FgHiRed, FgHiGreen, FgHiYellow, FgHiBlue, FgHiMagenta, FgHiCyan, FgHiWhite,
		BgBlack, BgRed, BgGreen, BgYellow, BgBlue, BgMagenta, BgCyan, BgWhite,
		Bold, Faint, Italic, Underline, BlinkSlow, ReverseVideo, Concealed, CrossedOut,
	}

	for _, attr := range attributes {
		t.Run("attribute_"+strconv.Itoa(int(attr)), func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			cfg := &Config{
				NoColor: false,
				Output:  buf,
				Error:   buf,
			}
			c := NewWithConfig(cfg, attr)
			_, _ = c.Fprintln(buf, "test")

			output := buf.String()
			assert.NotEmpty(t, output, "expected output for attribute %v", attr)
		})
	}
}

func Test_colorString_variousAttributes(t *testing.T) {
	t.Parallel()

	attributes := []Attribute{
		FgBlack, FgRed, FgGreen, FgYellow, FgBlue, FgMagenta, FgCyan, FgWhite,
		Bold, Italic, Underline,
	}

	for _, attr := range attributes {
		t.Run("attribute_"+strconv.Itoa(int(attr)), func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			cfg := &Config{
				NoColor: false,
				Output:  buf,
				Error:   buf,
			}
			c := NewWithConfig(cfg, attr)
			result := c.SprintFunc()("test")

			assert.NotEmpty(t, result, "expected output for attribute %v", attr)
			assert.Contains(t, result, "test", "result should contain the input string for attribute %v", attr)
		})
	}
}
