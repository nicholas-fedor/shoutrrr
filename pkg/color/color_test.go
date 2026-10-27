package color

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ensure Color implements io.Writer interface.
var _ io.Writer = (*bytes.Buffer)(nil)

// newTestColor creates a Color with a test configuration for thread-safe testing.
func newTestColor(output *bytes.Buffer, noColor bool, attrs ...Attribute) *Color {
	cfg := &Config{
		NoColor: noColor,
		Output:  output,
	}

	return NewWithConfig(cfg, attrs...)
}

// TestColor_Add tests the Add method which chains SGR parameters.
func TestColor_Add(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		initialAttrs   []Attribute
		addAttrs       []Attribute
		wantParamCount int
	}{
		{
			name:           "adds single attribute",
			initialAttrs:   []Attribute{FgRed},
			addAttrs:       []Attribute{Bold},
			wantParamCount: 2,
		},
		{
			name:           "adds multiple attributes",
			initialAttrs:   []Attribute{FgGreen},
			addAttrs:       []Attribute{Bold, Underline},
			wantParamCount: 3,
		},
		{
			name:           "adds to empty color",
			initialAttrs:   []Attribute{},
			addAttrs:       []Attribute{FgBlue},
			wantParamCount: 1,
		},
		{
			name:           "adds background color",
			initialAttrs:   []Attribute{FgRed},
			addAttrs:       []Attribute{BgYellow},
			wantParamCount: 2,
		},
		{
			name:           "returns same color instance for chaining",
			initialAttrs:   []Attribute{FgRed},
			addAttrs:       []Attribute{Bold},
			wantParamCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, false, tt.initialAttrs...)
			originalColor := c

			result := c.Add(tt.addAttrs...)

			assert.Len(t, c.params, tt.wantParamCount, "param count should match")
			assert.Same(t, originalColor, result, "should return same color instance for chaining")
		})
	}
}

// TestColor_AddBgRGB tests the AddBgRGB method for background RGB colors.
func TestColor_AddBgRGB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		r          int
		g          int
		b          int
		wantParams int
	}{
		{
			name:       "adds RGB background color",
			r:          255,
			g:          128,
			b:          0,
			wantParams: 5,
		},
		{
			name:       "adds zero RGB background",
			r:          0,
			g:          0,
			b:          0,
			wantParams: 5,
		},
		{
			name:       "adds max RGB background",
			r:          255,
			g:          255,
			b:          255,
			wantParams: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, false)
			result := c.AddBgRGB(tt.r, tt.g, tt.b)

			assert.Len(t, c.params, tt.wantParams, "should have correct param count")
			assert.Same(t, c, result, "should return same color for chaining")
		})
	}
}

// TestColor_AddRGB tests the AddRGB method for foreground RGB colors.
func TestColor_AddRGB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		r          int
		g          int
		b          int
		wantParams int
	}{
		{
			name:       "adds RGB foreground color",
			r:          255,
			g:          128,
			b:          0,
			wantParams: 5,
		},
		{
			name:       "adds zero RGB foreground",
			r:          0,
			g:          0,
			b:          0,
			wantParams: 5,
		},
		{
			name:       "adds max RGB foreground",
			r:          255,
			g:          255,
			b:          255,
			wantParams: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, false)
			result := c.AddRGB(tt.r, tt.g, tt.b)

			assert.Len(t, c.params, tt.wantParams, "should have correct param count")
			assert.Same(t, c, result, "should return same color for chaining")
		})
	}
}

// TestColor_DisableColor tests the DisableColor method.
func TestColor_DisableColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		wantNil bool
	}{
		{
			name:    "disables color on new color",
			attrs:   []Attribute{FgRed},
			wantNil: false,
		},
		{
			name:    "disables color on empty color",
			attrs:   []Attribute{},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, false, tt.attrs...)
			c.DisableColor()

			require.NotNil(t, c.noColor, "noColor should be set")
			assert.True(t, *c.noColor, "noColor should be true")
		})
	}
}

// TestColor_EnableColor tests the EnableColor method.
func TestColor_EnableColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		wantNil bool
	}{
		{
			name:    "enables color on new color",
			attrs:   []Attribute{FgRed},
			wantNil: false,
		},
		{
			name:    "enables color on empty color",
			attrs:   []Attribute{},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, true, tt.attrs...)
			c.EnableColor()

			require.NotNil(t, c.noColor, "noColor should be set")
			assert.False(t, *c.noColor, "noColor should be false")
		})
	}
}

// TestColor_Equals tests the Equals method.
func TestColor_Equals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		c              *Color
		colorToCompare *Color
		want           bool
	}{
		{
			name:           "equal colors",
			c:              New(FgRed, Bold),
			colorToCompare: New(FgRed, Bold),
			want:           true,
		},
		{
			name:           "different colors",
			c:              New(FgRed),
			colorToCompare: New(FgGreen),
			want:           false,
		},
		{
			name:           "different param counts",
			c:              New(FgRed),
			colorToCompare: New(FgRed, Bold),
			want:           false,
		},
		{
			name:           "both nil",
			c:              nil,
			colorToCompare: nil,
			want:           true,
		},
		{
			name:           "one nil",
			c:              New(FgRed),
			colorToCompare: nil,
			want:           false,
		},
		{
			name:           "same instance",
			c:              New(FgRed),
			colorToCompare: New(FgRed),
			want:           true,
		},
		{
			name:           "empty colors equal",
			c:              New(),
			colorToCompare: New(),
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.c.Equals(tt.colorToCompare)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestColor_Fprint tests the Fprint method.
func TestColor_Fprint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		args    []any
		noColor bool
		check   func(*testing.T, string)
	}{
		{
			name:    "prints with color",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "hello", "should contain the text")
				assert.Contains(t, output, "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name:    "prints without color when NoColor set",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: true,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Equal(t, "hello", output, "should be plain text")
			},
		},
		{
			name:    "prints multiple args",
			attrs:   []Attribute{FgGreen},
			args:    []any{"hello", "world"},
			noColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "hello", "should contain first arg")
				assert.Contains(t, output, "world", "should contain second arg")
			},
		},
		{
			name:    "prints empty",
			attrs:   []Attribute{FgBlue},
			args:    []any{},
			noColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				// When colors are enabled, even empty args produce the escape codes
				assert.NotEmpty(t, output, "should have escape codes even for empty")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)
			_, err := c.Fprint(buf, tt.args...)

			require.NoError(t, err)
			tt.check(t, buf.String())
		})
	}
}

// TestColor_FprintFunc tests the FprintFunc method.
func TestColor_FprintFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "returns function with color enabled",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "returns function with NoColor set",
			attrs:   []Attribute{FgGreen},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			fn := c.FprintFunc()
			require.NotNil(t, fn)

			fn(buf, "test")

			if tt.noColor {
				assert.Equal(t, "test", buf.String())
			} else {
				assert.NotEmpty(t, buf.String())
			}
		})
	}
}

// TestColor_Fprintf tests the Fprintf method.
func TestColor_Fprintf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		format  string
		args    []any
		noColor bool
		check   func(*testing.T, string)
	}{
		{
			name:    "prints formatted with color",
			attrs:   []Attribute{FgRed},
			format:  "value: %d",
			args:    []any{42},
			noColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "value: 42", "should contain formatted text")
				assert.Contains(t, output, "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name:    "prints formatted without color",
			attrs:   []Attribute{FgRed},
			format:  "value: %d",
			args:    []any{42},
			noColor: true,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Equal(t, "value: 42", output, "should be plain text")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)
			_, err := c.Fprintf(buf, tt.format, tt.args...)

			require.NoError(t, err)
			tt.check(t, buf.String())
		})
	}
}

// TestColor_FprintfFunc tests the FprintfFunc method.
func TestColor_FprintfFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "returns function with color enabled",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "returns function with NoColor set",
			attrs:   []Attribute{FgGreen},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			fn := c.FprintfFunc()
			require.NotNil(t, fn)

			fn(buf, "value: %d", 42)

			if tt.noColor {
				assert.Equal(t, "value: 42", buf.String())
			} else {
				assert.NotEmpty(t, buf.String())
			}
		})
	}
}

// TestColor_Fprintln tests the Fprintln method.
func TestColor_Fprintln(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		args    []any
		noColor bool
		check   func(*testing.T, string)
	}{
		{
			name:    "prints with newline with color",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "hello", "should contain the text")
				assert.Contains(t, output, "\n", "should contain newline")
			},
		},
		{
			name:    "prints with newline without color",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: true,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Equal(t, "hello\n", output, "should be plain text with newline")
			},
		},
		{
			name:    "prints multiple args with newline",
			attrs:   []Attribute{FgGreen},
			args:    []any{"hello", "world"},
			noColor: true,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Equal(t, "hello world\n", output, "should be plain text with newline")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)
			_, err := c.Fprintln(buf, tt.args...)

			require.NoError(t, err)
			tt.check(t, buf.String())
		})
	}
}

// TestColor_FprintlnFunc tests the FprintlnFunc method.
func TestColor_FprintlnFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "returns function with color enabled",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "returns function with NoColor set",
			attrs:   []Attribute{FgGreen},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			fn := c.FprintlnFunc()
			require.NotNil(t, fn)

			fn(buf, "test")

			assert.NotEmpty(t, buf.String())
		})
	}
}

// TestColor_Print tests the Print method.
func TestColor_Print(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		args    []any
		noColor bool
	}{
		{
			name:    "prints with color",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: false,
		},
		{
			name:    "prints without color when NoColor set",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			got, err := c.Print(tt.args...)

			require.NoError(t, err)
			assert.Positive(t, got)
		})
	}
}

// TestColor_PrintFunc tests the PrintFunc method.
func TestColor_PrintFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "returns function with color enabled",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "returns function with NoColor set",
			attrs:   []Attribute{FgGreen},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			fn := c.PrintFunc()
			require.NotNil(t, fn)

			fn("test")

			assert.NotEmpty(t, buf.String())
		})
	}
}

// TestColor_Printf tests the Printf method.
func TestColor_Printf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		format  string
		args    []any
		noColor bool
	}{
		{
			name:    "prints formatted with color",
			attrs:   []Attribute{FgRed},
			format:  "value: %d",
			args:    []any{42},
			noColor: false,
		},
		{
			name:    "prints formatted without color",
			attrs:   []Attribute{FgRed},
			format:  "value: %d",
			args:    []any{42},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			got, err := c.Printf(tt.format, tt.args...)

			require.NoError(t, err)
			assert.Positive(t, got)
		})
	}
}

// TestColor_PrintfFunc tests the PrintfFunc method.
func TestColor_PrintfFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "returns function with color enabled",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "returns function with NoColor set",
			attrs:   []Attribute{FgGreen},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			fn := c.PrintfFunc()
			require.NotNil(t, fn)

			fn("value: %d", 42)

			assert.NotEmpty(t, buf.String())
		})
	}
}

// TestColor_Println tests the Println method.
func TestColor_Println(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		args    []any
		noColor bool
		check   func(*testing.T, string)
	}{
		{
			name:    "prints with newline with color",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "hello", "should contain the text")
			},
		},
		{
			name:    "prints with newline without color",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: true,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Equal(t, "hello\n", output, "should be plain text with newline")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			_, err := c.Println(tt.args...)

			require.NoError(t, err)
			tt.check(t, buf.String())
		})
	}
}

// TestColor_PrintlnFunc tests the PrintlnFunc method.
func TestColor_PrintlnFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "returns function with color enabled",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "returns function with NoColor set",
			attrs:   []Attribute{FgGreen},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			fn := c.PrintlnFunc()
			require.NotNil(t, fn)

			fn("test")

			assert.NotEmpty(t, buf.String())
		})
	}
}

// TestColor_Set tests the Set method.
func TestColor_Set(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "sets color",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "skips when NoColor set",
			attrs:   []Attribute{FgRed},
			noColor: true,
		},
		{
			name:    "sets empty color",
			attrs:   []Attribute{},
			noColor: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			result := c.Set()

			assert.Same(t, c, result)
		})
	}
}

// TestColor_SetWriter tests the SetWriter method.
func TestColor_SetWriter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "sets writer with color",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "skips when NoColor set",
			attrs:   []Attribute{FgRed},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			result := c.SetWriter(buf)

			assert.Same(t, c, result)
		})
	}
}

// TestColor_Sprint tests the Sprint method.
func TestColor_Sprint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		args    []any
		noColor bool
		check   func(*testing.T, string)
	}{
		{
			name:    "returns colored string",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Contains(t, result, "hello", "should contain the text")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name:    "returns plain string when NoColor set",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "hello", result)
			},
		},
		{
			name:    "handles multiple args",
			attrs:   []Attribute{FgGreen},
			args:    []any{"hello", "world"},
			noColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "helloworld", result)
			},
		},
		{
			name:    "handles empty args",
			attrs:   []Attribute{FgBlue},
			args:    []any{},
			noColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Empty(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			got := c.Sprint(tt.args...)
			tt.check(t, got)
		})
	}
}

// TestColor_SprintFunc tests the SprintFunc method.
func TestColor_SprintFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "returns function with color enabled",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "returns function with NoColor set",
			attrs:   []Attribute{FgGreen},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			fn := c.SprintFunc()
			require.NotNil(t, fn)

			result := fn("test")

			if tt.noColor {
				assert.Equal(t, "test", result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestColor_Sprintf tests the Sprintf method.
func TestColor_Sprintf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		format  string
		args    []any
		noColor bool
		check   func(*testing.T, string)
	}{
		{
			name:    "returns formatted colored string",
			attrs:   []Attribute{FgRed},
			format:  "value: %d",
			args:    []any{42},
			noColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Contains(t, result, "value: 42", "should contain formatted text")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name:    "returns formatted plain string when NoColor set",
			attrs:   []Attribute{FgRed},
			format:  "value: %d",
			args:    []any{42},
			noColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "value: 42", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			got := c.Sprintf(tt.format, tt.args...)
			tt.check(t, got)
		})
	}
}

// TestColor_SprintfFunc tests the SprintfFunc method.
func TestColor_SprintfFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "returns function with color enabled",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "returns function with NoColor set",
			attrs:   []Attribute{FgGreen},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			fn := c.SprintfFunc()
			require.NotNil(t, fn)

			result := fn("value: %d", 42)

			if tt.noColor {
				assert.Equal(t, "value: 42", result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestColor_Sprintln tests the Sprintln method.
func TestColor_Sprintln(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		args    []any
		noColor bool
		check   func(*testing.T, string)
	}{
		{
			name:    "returns colored string with newline",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Contains(t, result, "hello", "should contain the text")
				assert.Contains(t, result, "\n", "should contain newline")
			},
		},
		{
			name:    "returns plain string with newline when NoColor set",
			attrs:   []Attribute{FgRed},
			args:    []any{"hello"},
			noColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "hello\n", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			got := c.Sprintln(tt.args...)
			tt.check(t, got)
		})
	}
}

// TestColor_SprintlnFunc tests the SprintlnFunc method.
func TestColor_SprintlnFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "returns function with color enabled",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "returns function with NoColor set",
			attrs:   []Attribute{FgGreen},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			fn := c.SprintlnFunc()
			require.NotNil(t, fn)

			result := fn("test")

			if tt.noColor {
				assert.Equal(t, "test\n", result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestColor_UnsetWriter tests the UnsetWriter method.
func TestColor_UnsetWriter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "unsets writer with color",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "skips when NoColor set",
			attrs:   []Attribute{FgRed},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			c.UnsetWriter(buf)

			// Just verify it doesn't panic
		})
	}
}

// TestColor_attrExists tests the attrExists method.
func TestColor_attrExists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		c    *Color
		attr Attribute
		want bool
	}{
		{
			name: "attribute exists",
			c:    New(FgRed, Bold),
			attr: FgRed,
			want: true,
		},
		{
			name: "attribute does not exist",
			c:    New(FgRed),
			attr: FgGreen,
			want: false,
		},
		{
			name: "empty color has no attributes",
			c:    New(),
			attr: FgRed,
			want: false,
		},
		{
			name: "multiple attributes check one",
			c:    New(FgRed, FgGreen, FgBlue),
			attr: FgGreen,
			want: true,
		},
		{
			name: "check for style attribute",
			c:    New(FgRed, Bold, Underline),
			attr: Bold,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.c.attrExists(tt.attr)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestColor_format tests the format method.
func TestColor_format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		c       *Color
		noColor bool
		check   func(*testing.T, string)
	}{
		{
			name:    "formats with color",
			c:       New(FgRed),
			noColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result)
				assert.Contains(t, result, "\x1b[")
			},
		},
		{
			name:    "formats empty color",
			c:       New(),
			noColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				// Empty color with no params returns escape + [ + m (short reset form)
				assert.Equal(t, "\x1b[m", result)
			},
		},
		{
			name: "formats with NoColor set on color",
			c: func() *Color {
				c := New(FgRed)
				c.DisableColor()

				return c
			}(),
			noColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				// format() does not check NoColor - it always generates the ANSI sequence
				assert.Equal(t, "\x1b[31m", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.c.format()
			tt.check(t, got)
		})
	}
}

// TestColor_isNoColorSet tests the isNoColorSet method.
func TestColor_isNoColorSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		c       *Color
		noColor bool
		want    bool
	}{
		{
			name: "returns true when noColor is explicitly set",
			c: func() *Color {
				c := New(FgRed)
				c.DisableColor()

				return c
			}(),
			noColor: false,
			want:    true,
		},
		{
			name: "returns false when noColor is explicitly enabled",
			c: func() *Color {
				c := New(FgRed)
				c.EnableColor()

				return c
			}(),
			noColor: false,
			want:    false,
		},
		{
			name: "returns config NoColor when noColor is nil (color enabled)",
			c: func() *Color {
				buf := &bytes.Buffer{}
				cfg := &Config{NoColor: false, Output: buf}

				return NewWithConfig(cfg)
			}(),
			noColor: false,
			want:    false,
		},
		{
			name: "returns config NoColor when noColor is nil (color disabled)",
			c: func() *Color {
				buf := &bytes.Buffer{}
				cfg := &Config{NoColor: true, Output: buf}

				return NewWithConfig(cfg)
			}(),
			noColor: true,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.c.isNoColorSet()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestColor_sequence tests the sequence method.
func TestColor_sequence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		c    *Color
		want string
	}{
		{
			name: "single attribute",
			c:    New(FgRed),
			want: "31",
		},
		{
			name: "multiple attributes",
			c:    New(FgRed, Bold),
			want: "31;1",
		},
		{
			name: "empty color",
			c:    New(),
			want: "",
		},
		{
			name: "foreground colors",
			c:    New(FgBlack, FgRed, FgGreen),
			want: "30;31;32",
		},
		{
			name: "background colors",
			c:    New(BgBlack, BgRed),
			want: "40;41",
		},
		{
			name: "style attributes",
			c:    New(Bold, Underline, Italic),
			want: "1;4;3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.c.sequence()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestColor_unformat tests the unformat method.
func TestColor_unformat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		c    *Color
		want string
	}{
		{
			name: "single attribute reset",
			c:    New(FgRed),
			want: "\x1b[0m",
		},
		{
			name: "multiple attributes reset",
			c:    New(FgRed, Bold),
			want: "\x1b[0;22m",
		},
		{
			name: "empty color",
			c:    New(),
			want: "",
		},
		{
			name: "bold reset",
			c:    New(Bold),
			want: "\x1b[22m",
		},
		{
			name: "underline reset",
			c:    New(Underline),
			want: "\x1b[24m",
		},
		{
			name: "italic reset",
			c:    New(Italic),
			want: "\x1b[23m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.c.unformat()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestColor_unset tests the unset method.
func TestColor_unset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{
			name:    "unsets with color",
			attrs:   []Attribute{FgRed},
			noColor: false,
		},
		{
			name:    "skips when NoColor set",
			attrs:   []Attribute{FgRed},
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			c.Unset()

			// Just verify it doesn't panic
		})
	}
}

// TestColor_wrap tests the wrap method.
func TestColor_wrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   []Attribute
		s       string
		noColor bool
		check   func(*testing.T, string)
	}{
		{
			name:    "wraps string with color",
			attrs:   []Attribute{FgRed},
			s:       "hello",
			noColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Contains(t, result, "hello", "should contain the text")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name:    "returns plain string when NoColor set",
			attrs:   []Attribute{FgRed},
			s:       "hello",
			noColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "hello", result)
			},
		},
		{
			name:    "wraps empty string",
			attrs:   []Attribute{FgRed},
			s:       "",
			noColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result, "should have escape codes even for empty string")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			c := newTestColor(buf, tt.noColor, tt.attrs...)

			got := c.wrap(tt.s)
			tt.check(t, got)
		})
	}
}

// TestNew tests the New function.
func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      []Attribute
		wantParams int
	}{
		{
			name:       "creates color with single attribute",
			value:      []Attribute{FgRed},
			wantParams: 1,
		},
		{
			name:       "creates color with multiple attributes",
			value:      []Attribute{FgRed, Bold, Underline},
			wantParams: 3,
		},
		{
			name:       "creates empty color",
			value:      []Attribute{},
			wantParams: 0,
		},
		{
			name:       "creates color with no arguments",
			value:      nil,
			wantParams: 0,
		},
		{
			name:       "creates color with background",
			value:      []Attribute{BgRed},
			wantParams: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := New(tt.value...)

			require.NotNil(t, got)
			assert.Len(t, got.params, tt.wantParams)
		})
	}
}

// TestNewWithConfig verifies that NewWithConfig keeps the config and sets no
// override of its own, so the color reports the config's NoColor setting.
func TestNewWithConfig(t *testing.T) {
	t.Parallel()

	// tests lists the config settings and attributes for each new color.
	tests := []struct {
		name    string
		attrs   []Attribute
		noColor bool
	}{
		{name: "creates color with NoColor=true", noColor: true, attrs: []Attribute{FgRed}},
		{name: "creates color with NoColor=false", noColor: false, attrs: []Attribute{FgRed}},
		{name: "creates empty color with config", noColor: false, attrs: []Attribute{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{NoColor: tt.noColor, Output: &bytes.Buffer{}}

			got := NewWithConfig(cfg, tt.attrs...)

			require.NotNil(t, got)
			assert.Same(t, cfg, got.config)
			assert.Nil(t, got.noColor)
			assert.Equal(t, tt.noColor, got.isNoColorSet())
		})
	}
}

// TestNilWriters verifies that a config without an Output writer writes to
// standard output instead of panicking. It covers a nil config and an empty
// Config for every method that writes through the config.
//
// The nil config takes its NoColor setting from the environment, so the test
// sets NO_COLOR to get plain output from it. The empty Config keeps color
// enabled, so its cases check the escape codes too.
func TestNilWriters(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	// configs lists the configs that leave the writer unset, and whether each
	// writes escape codes.
	configs := []struct {
		cfg     *Config
		name    string
		colored bool
	}{
		{name: "nil config", cfg: nil, colored: false},
		{name: "config without an Output writer", cfg: &Config{Output: nil, NoColor: false}, colored: true},
	}

	// writes lists the methods that write through the config, with the output
	// each writes without and with color.
	writes := []struct {
		write   func(c *Color)
		name    string
		plain   string
		colored string
	}{
		{
			name:    "Print",
			plain:   "text",
			colored: "\x1b[31mtext\x1b[0m",
			write:   func(c *Color) { _, _ = c.Print("text") },
		},
		{
			name:    "Printf",
			plain:   "text 1",
			colored: "\x1b[31mtext 1\x1b[0m",
			write:   func(c *Color) { _, _ = c.Printf("text %d", 1) },
		},
		{
			name:    "Println",
			plain:   "text\n",
			colored: "\x1b[31mtext\x1b[0m\n",
			write:   func(c *Color) { _, _ = c.Println("text") },
		},
		{
			name:    "Set and Unset",
			plain:   "",
			colored: "\x1b[31m\x1b[0m",
			write:   func(c *Color) { c.Set().Unset() },
		},
	}

	for _, config := range configs {
		for _, tt := range writes {
			t.Run(config.name+"/"+tt.name, func(t *testing.T) {
				stdout := captureStdout(t, func() {
					assert.NotPanics(t, func() { tt.write(NewWithConfig(config.cfg, FgRed)) })
				})

				want := tt.plain
				if config.colored {
					want = tt.colored
				}

				assert.Equal(t, want, stdout)
			})
		}
	}
}

// TestColorKeepsItsStdoutWriter verifies that a color without an Output writer
// keeps the standard output it saw when it was created. Set, the text, and
// Unset of a later Print all reach that writer, even after os.Stdout changes.
//
//nolint:paralleltest // Redirects os.Stdout.
func TestColorKeepsItsStdoutWriter(t *testing.T) {
	var c *Color

	created := captureStdout(t, func() {
		c = NewWithConfig(&Config{Output: nil, NoColor: false}, FgRed)

		later := captureStdout(t, func() { _, _ = c.Print("text") })
		assert.Empty(t, later)
	})

	assert.Equal(t, "\x1b[31mtext\x1b[0m", created)
}

// TestColorWithClearedOutput verifies that a color whose config loses its
// Output writer after the color was created writes to standard output. The
// first write creates the color's writer, and later writes reuse it even after
// os.Stdout changes.
//
//nolint:paralleltest // Redirects os.Stdout.
func TestColorWithClearedOutput(t *testing.T) {
	cfg := &Config{Output: &bytes.Buffer{}, NoColor: false}
	c := NewWithConfig(cfg, FgRed)
	cfg.Output = nil

	stdout := captureStdout(t, func() {
		_, _ = c.Print("first")

		later := captureStdout(t, func() { _, _ = c.Print("second") })
		assert.Empty(t, later)
	})

	assert.Equal(t, "\x1b[31mfirst\x1b[0m\x1b[31msecond\x1b[0m", stdout)
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what fn
// wrote to it.
//
// Parameters:
//   - t: the test.
//   - fn: the function whose standard output is captured.
//
// Returns:
//   - string: the captured output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	defer func() { _ = reader.Close() }()

	original := os.Stdout
	os.Stdout = writer

	defer func() { os.Stdout = original }()

	fn()

	require.NoError(t, writer.Close())

	output, err := io.ReadAll(reader)
	require.NoError(t, err)

	return string(output)
}

// TestColorOverrideWinsOverConfig verifies that a color's own EnableColor or
// DisableColor decides its output for every writing and string method,
// whatever the config's NoColor setting, and that a color without an override
// follows the config.
func TestColorOverrideWinsOverConfig(t *testing.T) {
	t.Parallel()

	// tests lists config settings and overrides with whether the output is
	// colored.
	tests := []struct {
		override    func(c *Color)
		name        string
		configNoClr bool
		wantColored bool
	}{
		{name: "EnableColor on a disabled config", configNoClr: true, override: (*Color).EnableColor, wantColored: true},
		{name: "DisableColor on an enabled config", configNoClr: false, override: (*Color).DisableColor, wantColored: false},
		{name: "disabled config without an override", configNoClr: true, override: func(*Color) {}, wantColored: false},
		{name: "enabled config without an override", configNoClr: false, override: func(*Color) {}, wantColored: true},
	}

	// methods lists the methods that write or return colored text, with the
	// output each produces through the given color and buffer.
	methods := []struct {
		run   func(c *Color, buf *bytes.Buffer) string
		name  string
		plain string
	}{
		{name: "Print", plain: "text", run: func(c *Color, buf *bytes.Buffer) string {
			_, _ = c.Print("text")

			return buf.String()
		}},
		{name: "Printf", plain: "text", run: func(c *Color, buf *bytes.Buffer) string {
			_, _ = c.Printf("%s", "text")

			return buf.String()
		}},
		{name: "Println", plain: "text\n", run: func(c *Color, buf *bytes.Buffer) string {
			_, _ = c.Println("text")

			return buf.String()
		}},
		{name: "Fprint", plain: "text", run: func(c *Color, buf *bytes.Buffer) string {
			_, _ = c.Fprint(buf, "text")

			return buf.String()
		}},
		{name: "Fprintln", plain: "text\n", run: func(c *Color, buf *bytes.Buffer) string {
			_, _ = c.Fprintln(buf, "text")

			return buf.String()
		}},
		{name: "Set and Unset", plain: "", run: func(c *Color, buf *bytes.Buffer) string {
			c.Set().Unset()

			return buf.String()
		}},
		{name: "SetWriter and UnsetWriter", plain: "", run: func(c *Color, buf *bytes.Buffer) string {
			c.SetWriter(buf).UnsetWriter(buf)

			return buf.String()
		}},
		{name: "Sprint", plain: "text", run: func(c *Color, _ *bytes.Buffer) string { return c.Sprint("text") }},
		{name: "SprintFunc", plain: "text", run: func(c *Color, _ *bytes.Buffer) string { return c.SprintFunc()("text") }},
	}

	for _, tt := range tests {
		for _, method := range methods {
			t.Run(tt.name+"/"+method.name, func(t *testing.T) {
				t.Parallel()

				buf := &bytes.Buffer{}
				c := NewWithConfig(&Config{Output: buf, NoColor: tt.configNoClr}, FgRed)
				tt.override(c)

				got := method.run(c, buf)

				if tt.wantColored {
					assert.Contains(t, got, "\x1b[31m")
				} else {
					assert.Equal(t, method.plain, got)
				}
			})
		}
	}
}

// TestColorFollowsConfigChanges verifies that a color without an override
// reads its config's NoColor setting when it writes, so changing the setting
// after the color was created takes effect in either direction.
func TestColorFollowsConfigChanges(t *testing.T) {
	t.Parallel()

	// tests lists the NoColor setting at creation and the setting it changes to.
	tests := []struct {
		name        string
		initial     bool
		changed     bool
		wantColored bool
	}{
		{name: "color turned on after creation", initial: true, changed: false, wantColored: true},
		{name: "color turned off after creation", initial: false, changed: true, wantColored: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{Output: &bytes.Buffer{}, NoColor: tt.initial}
			c := NewWithConfig(cfg, FgRed)
			cfg.NoColor = tt.changed

			assert.Equal(t, !tt.wantColored, c.isNoColorSet())

			if tt.wantColored {
				assert.Equal(t, "\x1b[31mtext\x1b[0m", c.Sprint("text"))
			} else {
				assert.Equal(t, "text", c.Sprint("text"))
			}
		})
	}
}

// TestSprintFuncsFollowChanges verifies that the functions SprintfFunc and
// SprintlnFunc return decide on color each time they are called. A change to
// the config's NoColor setting, or a later EnableColor or DisableColor, takes
// effect in a function created before it.
func TestSprintFuncsFollowChanges(t *testing.T) {
	t.Parallel()

	// changes lists the starting NoColor setting, the change made after the
	// functions are created, and whether the output is then colored.
	changes := []struct {
		change      func(cfg *Config, c *Color)
		name        string
		initial     bool
		wantColored bool
	}{
		{name: "config turns color on", initial: true, wantColored: true, change: func(cfg *Config, _ *Color) { cfg.NoColor = false }},
		{name: "config turns color off", initial: false, wantColored: false, change: func(cfg *Config, _ *Color) { cfg.NoColor = true }},
		{name: "EnableColor", initial: true, wantColored: true, change: func(_ *Config, c *Color) { c.EnableColor() }},
		{name: "DisableColor", initial: false, wantColored: false, change: func(_ *Config, c *Color) { c.DisableColor() }},
	}

	for _, tt := range changes {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{Output: &bytes.Buffer{}, NoColor: tt.initial}
			c := NewWithConfig(cfg, FgRed)
			sprintf := c.SprintfFunc()
			sprintln := c.SprintlnFunc()

			tt.change(cfg, c)

			if tt.wantColored {
				assert.Equal(t, "\x1b[31mtext 1\x1b[0m", sprintf("text %d", 1))
				assert.Equal(t, "\x1b[31mtext\x1b[0m\n", sprintln("text"))
			} else {
				assert.Equal(t, "text 1", sprintf("text %d", 1))
				assert.Equal(t, "text\n", sprintln("text"))
			}
		})
	}
}

// TestDefaultConfig tests the DefaultConfig function.
func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	require.NotNil(t, cfg)
	assert.NotNil(t, cfg.Output)
	// NoColor depends on environment, so we just verify it's set
}

// TestBgRGB tests the BgRGB function.
func TestBgRGB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		r          int
		g          int
		b          int
		wantParams int
	}{
		{
			name:       "creates background RGB color",
			r:          255,
			g:          128,
			b:          0,
			wantParams: 5,
		},
		{
			name:       "creates black background",
			r:          0,
			g:          0,
			b:          0,
			wantParams: 5,
		},
		{
			name:       "creates white background",
			r:          255,
			g:          255,
			b:          255,
			wantParams: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := BgRGB(tt.r, tt.g, tt.b)

			require.NotNil(t, got)
			assert.Len(t, got.params, tt.wantParams)
		})
	}
}

// TestRGB tests the RGB function.
func TestRGB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		r          int
		g          int
		b          int
		wantParams int
	}{
		{
			name:       "creates foreground RGB color",
			r:          255,
			g:          128,
			b:          0,
			wantParams: 5,
		},
		{
			name:       "creates black foreground",
			r:          0,
			g:          0,
			b:          0,
			wantParams: 5,
		},
		{
			name:       "creates white foreground",
			r:          255,
			g:          255,
			b:          255,
			wantParams: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := RGB(tt.r, tt.g, tt.b)

			require.NotNil(t, got)
			assert.Len(t, got.params, tt.wantParams)
		})
	}
}
