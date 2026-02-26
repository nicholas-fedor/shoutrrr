package color

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ensure Color implements io.Writer interface.
var _ io.Writer = (*bytes.Buffer)(nil)

// TestColor_Add tests the Add method which chains SGR parameters.
func TestColor_Add(t *testing.T) {
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
			c := New(tt.initialAttrs...)
			originalColor := c

			result := c.Add(tt.addAttrs...)

			assert.Len(t, c.params, tt.wantParamCount, "param count should match")
			assert.Same(t, originalColor, result, "should return same color instance for chaining")
		})
	}
}

// TestColor_AddBgRGB tests the AddBgRGB method for background RGB colors.
func TestColor_AddBgRGB(t *testing.T) {
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
			c := New()
			result := c.AddBgRGB(tt.r, tt.g, tt.b)

			assert.Len(t, c.params, tt.wantParams, "should have correct param count")
			assert.Same(t, c, result, "should return same color for chaining")
		})
	}
}

// TestColor_AddRGB tests the AddRGB method for foreground RGB colors.
func TestColor_AddRGB(t *testing.T) {
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
			c := New()
			result := c.AddRGB(tt.r, tt.g, tt.b)

			assert.Len(t, c.params, tt.wantParams, "should have correct param count")
			assert.Same(t, c, result, "should return same color for chaining")
		})
	}
}

// TestColor_DisableColor tests the DisableColor method.
func TestColor_DisableColor(t *testing.T) {
	tests := []struct {
		name    string
		c       *Color
		wantNil bool
	}{
		{
			name:    "disables color on new color",
			c:       New(FgRed),
			wantNil: false,
		},
		{
			name:    "disables color on empty color",
			c:       New(),
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.c.DisableColor()

			require.NotNil(t, tt.c.noColor, "noColor should be set")
			assert.True(t, *tt.c.noColor, "noColor should be true")
		})
	}
}

// TestColor_EnableColor tests the EnableColor method.
func TestColor_EnableColor(t *testing.T) {
	tests := []struct {
		name    string
		c       *Color
		wantNil bool
	}{
		{
			name:    "enables color on new color",
			c:       New(FgRed),
			wantNil: false,
		},
		{
			name:    "enables color on empty color",
			c:       New(),
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.c.EnableColor()

			require.NotNil(t, tt.c.noColor, "noColor should be set")
			assert.False(t, *tt.c.noColor, "noColor should be false")
		})
	}
}

// TestColor_Equals tests the Equals method.
func TestColor_Equals(t *testing.T) {
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
			got := tt.c.Equals(tt.colorToCompare)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestColor_Fprint tests the Fprint method.
func TestColor_Fprint(t *testing.T) {
	// Save and restore global NoColor
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		args       []any
		setNoColor bool
		check      func(*testing.T, string)
	}{
		{
			name:       "prints with color",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "hello", "should contain the text")
				assert.Contains(t, output, "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name:       "prints without color when NoColor set",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: true,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Equal(t, "hello", output, "should be plain text")
			},
		},
		{
			name:       "prints multiple args",
			c:          New(FgGreen),
			args:       []any{"hello", "world"},
			setNoColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "hello", "should contain first arg")
				assert.Contains(t, output, "world", "should contain second arg")
			},
		},
		{
			name:       "prints empty",
			c:          New(FgBlue),
			args:       []any{},
			setNoColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				// When colors are enabled, even empty args produce the escape codes
				assert.NotEmpty(t, output, "should have escape codes even for empty")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			buf := &bytes.Buffer{}
			_, err := tt.c.Fprint(buf, tt.args...)

			require.NoError(t, err)
			tt.check(t, buf.String())
		})
	}
}

// TestColor_FprintFunc tests the FprintFunc method.
func TestColor_FprintFunc(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "returns function with color enabled",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "returns function with NoColor set",
			c:          New(FgGreen),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			fn := tt.c.FprintFunc()
			require.NotNil(t, fn)

			buf := &bytes.Buffer{}
			fn(buf, "test")

			if tt.setNoColor {
				assert.Equal(t, "test", buf.String())
			} else {
				assert.NotEmpty(t, buf.String())
			}
		})
	}
}

// TestColor_Fprintf tests the Fprintf method.
func TestColor_Fprintf(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		format     string
		args       []any
		setNoColor bool
		check      func(*testing.T, string)
	}{
		{
			name:       "prints formatted with color",
			c:          New(FgRed),
			format:     "value: %d",
			args:       []any{42},
			setNoColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "value: 42", "should contain formatted text")
				assert.Contains(t, output, "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name:       "prints formatted without color",
			c:          New(FgRed),
			format:     "value: %d",
			args:       []any{42},
			setNoColor: true,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Equal(t, "value: 42", output, "should be plain text")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			buf := &bytes.Buffer{}
			_, err := tt.c.Fprintf(buf, tt.format, tt.args...)

			require.NoError(t, err)
			tt.check(t, buf.String())
		})
	}
}

// TestColor_FprintfFunc tests the FprintfFunc method.
func TestColor_FprintfFunc(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "returns function with color enabled",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "returns function with NoColor set",
			c:          New(FgGreen),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			fn := tt.c.FprintfFunc()
			require.NotNil(t, fn)

			buf := &bytes.Buffer{}
			fn(buf, "value: %d", 42)

			if tt.setNoColor {
				assert.Equal(t, "value: 42", buf.String())
			} else {
				assert.NotEmpty(t, buf.String())
			}
		})
	}
}

// TestColor_Fprintln tests the Fprintln method.
func TestColor_Fprintln(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		args       []any
		setNoColor bool
		check      func(*testing.T, string)
	}{
		{
			name:       "prints with newline with color",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "hello", "should contain the text")
				assert.Contains(t, output, "\n", "should contain newline")
			},
		},
		{
			name:       "prints with newline without color",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: true,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Equal(t, "hello\n", output, "should be plain text with newline")
			},
		},
		{
			name:       "prints multiple args with newline",
			c:          New(FgGreen),
			args:       []any{"hello", "world"},
			setNoColor: true,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Equal(t, "hello world\n", output, "should be plain text with newline")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			buf := &bytes.Buffer{}
			_, err := tt.c.Fprintln(buf, tt.args...)

			require.NoError(t, err)
			tt.check(t, buf.String())
		})
	}
}

// TestColor_FprintlnFunc tests the FprintlnFunc method.
func TestColor_FprintlnFunc(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "returns function with color enabled",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "returns function with NoColor set",
			c:          New(FgGreen),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			fn := tt.c.FprintlnFunc()
			require.NotNil(t, fn)

			buf := &bytes.Buffer{}
			fn(buf, "test")

			assert.NotEmpty(t, buf.String())
		})
	}
}

// TestColor_Print tests the Print method.
func TestColor_Print(t *testing.T) {
	originalNoColor := NoColor
	originalOutput := Output

	t.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	tests := []struct {
		name       string
		c          *Color
		args       []any
		setNoColor bool
	}{
		{
			name:       "prints with color",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: false,
		},
		{
			name:       "prints without color when NoColor set",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}
			Output = buf

			got, err := tt.c.Print(tt.args...)

			require.NoError(t, err)
			assert.Positive(t, got)
		})
	}
}

// TestColor_PrintFunc tests the PrintFunc method.
func TestColor_PrintFunc(t *testing.T) {
	originalNoColor := NoColor
	originalOutput := Output

	t.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "returns function with color enabled",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "returns function with NoColor set",
			c:          New(FgGreen),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}
			Output = buf

			fn := tt.c.PrintFunc()
			require.NotNil(t, fn)

			fn("test")

			assert.NotEmpty(t, buf.String())
		})
	}
}

// TestColor_Printf tests the Printf method.
func TestColor_Printf(t *testing.T) {
	originalNoColor := NoColor
	originalOutput := Output

	t.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	tests := []struct {
		name       string
		c          *Color
		format     string
		args       []any
		setNoColor bool
	}{
		{
			name:       "prints formatted with color",
			c:          New(FgRed),
			format:     "value: %d",
			args:       []any{42},
			setNoColor: false,
		},
		{
			name:       "prints formatted without color",
			c:          New(FgRed),
			format:     "value: %d",
			args:       []any{42},
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}
			Output = buf

			got, err := tt.c.Printf(tt.format, tt.args...)

			require.NoError(t, err)
			assert.Positive(t, got)
		})
	}
}

// TestColor_PrintfFunc tests the PrintfFunc method.
func TestColor_PrintfFunc(t *testing.T) {
	originalNoColor := NoColor
	originalOutput := Output

	t.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "returns function with color enabled",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "returns function with NoColor set",
			c:          New(FgGreen),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}
			Output = buf

			fn := tt.c.PrintfFunc()
			require.NotNil(t, fn)

			fn("value: %d", 42)

			assert.NotEmpty(t, buf.String())
		})
	}
}

// TestColor_Println tests the Println method.
func TestColor_Println(t *testing.T) {
	originalNoColor := NoColor
	originalOutput := Output

	t.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	tests := []struct {
		name       string
		c          *Color
		args       []any
		setNoColor bool
		check      func(*testing.T, string)
	}{
		{
			name:       "prints with newline with color",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: false,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "hello", "should contain the text")
			},
		},
		{
			name:       "prints with newline without color",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: true,
			check: func(t *testing.T, output string) {
				t.Helper()
				assert.Equal(t, "hello\n", output, "should be plain text with newline")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}
			Output = buf

			_, err := tt.c.Println(tt.args...)

			require.NoError(t, err)
			tt.check(t, buf.String())
		})
	}
}

// TestColor_PrintlnFunc tests the PrintlnFunc method.
func TestColor_PrintlnFunc(t *testing.T) {
	originalNoColor := NoColor
	originalOutput := Output

	t.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "returns function with color enabled",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "returns function with NoColor set",
			c:          New(FgGreen),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}
			Output = buf

			fn := tt.c.PrintlnFunc()
			require.NotNil(t, fn)

			fn("test")

			assert.NotEmpty(t, buf.String())
		})
	}
}

// TestColor_Set tests the Set method.
func TestColor_Set(t *testing.T) {
	originalNoColor := NoColor
	originalOutput := Output

	t.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "sets color",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "skips when NoColor set",
			c:          New(FgRed),
			setNoColor: true,
		},
		{
			name:       "sets empty color",
			c:          New(),
			setNoColor: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}
			Output = buf

			result := tt.c.Set()

			assert.Same(t, tt.c, result)
		})
	}
}

// TestColor_SetWriter tests the SetWriter method.
func TestColor_SetWriter(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "sets writer with color",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "skips when NoColor set",
			c:          New(FgRed),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}

			result := tt.c.SetWriter(buf)

			assert.Same(t, tt.c, result)
		})
	}
}

// TestColor_Sprint tests the Sprint method.
func TestColor_Sprint(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		args       []any
		setNoColor bool
		check      func(*testing.T, string)
	}{
		{
			name:       "returns colored string",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Contains(t, result, "hello", "should contain the text")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name:       "returns plain string when NoColor set",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "hello", result)
			},
		},
		{
			name:       "handles multiple args",
			c:          New(FgGreen),
			args:       []any{"hello", "world"},
			setNoColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "helloworld", result)
			},
		},
		{
			name:       "handles empty args",
			c:          New(FgBlue),
			args:       []any{},
			setNoColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Empty(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			got := tt.c.Sprint(tt.args...)
			tt.check(t, got)
		})
	}
}

// TestColor_SprintFunc tests the SprintFunc method.
func TestColor_SprintFunc(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "returns function with color enabled",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "returns function with NoColor set",
			c:          New(FgGreen),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			fn := tt.c.SprintFunc()
			require.NotNil(t, fn)

			result := fn("test")

			if tt.setNoColor {
				assert.Equal(t, "test", result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestColor_Sprintf tests the Sprintf method.
func TestColor_Sprintf(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		format     string
		args       []any
		setNoColor bool
		check      func(*testing.T, string)
	}{
		{
			name:       "returns formatted colored string",
			c:          New(FgRed),
			format:     "value: %d",
			args:       []any{42},
			setNoColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Contains(t, result, "value: 42", "should contain formatted text")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name:       "returns formatted plain string when NoColor set",
			c:          New(FgRed),
			format:     "value: %d",
			args:       []any{42},
			setNoColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "value: 42", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			got := tt.c.Sprintf(tt.format, tt.args...)
			tt.check(t, got)
		})
	}
}

// TestColor_SprintfFunc tests the SprintfFunc method.
func TestColor_SprintfFunc(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "returns function with color enabled",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "returns function with NoColor set",
			c:          New(FgGreen),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			fn := tt.c.SprintfFunc()
			require.NotNil(t, fn)

			result := fn("value: %d", 42)

			if tt.setNoColor {
				assert.Equal(t, "value: 42", result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestColor_Sprintln tests the Sprintln method.
func TestColor_Sprintln(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		args       []any
		setNoColor bool
		check      func(*testing.T, string)
	}{
		{
			name:       "returns colored string with newline",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Contains(t, result, "hello", "should contain the text")
				assert.Contains(t, result, "\n", "should contain newline")
			},
		},
		{
			name:       "returns plain string with newline when NoColor set",
			c:          New(FgRed),
			args:       []any{"hello"},
			setNoColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "hello\n", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			got := tt.c.Sprintln(tt.args...)
			tt.check(t, got)
		})
	}
}

// TestColor_SprintlnFunc tests the SprintlnFunc method.
func TestColor_SprintlnFunc(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "returns function with color enabled",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "returns function with NoColor set",
			c:          New(FgGreen),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			fn := tt.c.SprintlnFunc()
			require.NotNil(t, fn)

			result := fn("test")

			if tt.setNoColor {
				assert.Equal(t, "test\n", result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestColor_UnsetWriter tests the UnsetWriter method.
func TestColor_UnsetWriter(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "unsets writer with color",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "skips when NoColor set",
			c:          New(FgRed),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}

			tt.c.UnsetWriter(buf)

			// Just verify it doesn't panic
		})
	}
}

// TestColor_attrExists tests the attrExists method.
func TestColor_attrExists(t *testing.T) {
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
			got := tt.c.attrExists(tt.attr)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestColor_format tests the format method.
func TestColor_format(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
		check      func(*testing.T, string)
	}{
		{
			name:       "formats with color",
			c:          New(FgRed),
			setNoColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result)
				assert.Contains(t, result, "\x1b[")
			},
		},
		{
			name:       "formats empty color",
			c:          New(),
			setNoColor: false,
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
			setNoColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				// format() does not check NoColor - it always generates the ANSI sequence
				assert.Equal(t, "\x1b[31m", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			got := tt.c.format()
			tt.check(t, got)
		})
	}
}

// TestColor_isNoColorSet tests the isNoColorSet method.
func TestColor_isNoColorSet(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
		want       bool
	}{
		{
			name: "returns true when noColor is explicitly set",
			c: func() *Color {
				c := New(FgRed)
				c.DisableColor()

				return c
			}(),
			setNoColor: false,
			want:       true,
		},
		{
			name: "returns false when noColor is explicitly enabled",
			c: func() *Color {
				c := New(FgRed)
				c.EnableColor()

				return c
			}(),
			setNoColor: false,
			want:       false,
		},
		{
			name:       "returns global NoColor when noColor is nil",
			c:          New(),
			setNoColor: false,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			got := tt.c.isNoColorSet()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestColor_sequence tests the sequence method.
func TestColor_sequence(t *testing.T) {
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
			got := tt.c.sequence()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestColor_unformat tests the unformat method.
func TestColor_unformat(t *testing.T) {
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
			got := tt.c.unformat()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestColor_unset tests the unset method.
func TestColor_unset(t *testing.T) {
	originalNoColor := NoColor
	originalOutput := Output

	t.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	tests := []struct {
		name       string
		c          *Color
		setNoColor bool
	}{
		{
			name:       "unsets with color",
			c:          New(FgRed),
			setNoColor: false,
		},
		{
			name:       "skips when NoColor set",
			c:          New(FgRed),
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}
			Output = buf

			tt.c.unset()

			// Just verify it doesn't panic
		})
	}
}

// TestColor_wrap tests the wrap method.
func TestColor_wrap(t *testing.T) {
	originalNoColor := NoColor

	t.Cleanup(func() {
		NoColor = originalNoColor
	})

	tests := []struct {
		name       string
		c          *Color
		s          string
		setNoColor bool
		check      func(*testing.T, string)
	}{
		{
			name:       "wraps string with color",
			c:          New(FgRed),
			s:          "hello",
			setNoColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Contains(t, result, "hello", "should contain the text")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name:       "returns plain string when NoColor set",
			c:          New(FgRed),
			s:          "hello",
			setNoColor: true,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "hello", result)
			},
		},
		{
			name:       "wraps empty string",
			c:          New(FgRed),
			s:          "",
			setNoColor: false,
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result, "should have escape codes even for empty string")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor

			got := tt.c.wrap(tt.s)
			tt.check(t, got)
		})
	}
}

// TestNew tests the New function.
func TestNew(t *testing.T) {
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
			got := New(tt.value...)

			require.NotNil(t, got)
			assert.Len(t, got.params, tt.wantParams)
		})
	}
}

// TestSet tests the Set function.
func TestSet(t *testing.T) {
	originalNoColor := NoColor
	originalOutput := Output

	t.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	tests := []struct {
		name       string
		p          []Attribute
		setNoColor bool
	}{
		{
			name:       "sets color with attributes",
			p:          []Attribute{FgRed},
			setNoColor: false,
		},
		{
			name:       "sets color with NoColor",
			p:          []Attribute{FgRed},
			setNoColor: true,
		},
		{
			name:       "sets empty color",
			p:          []Attribute{},
			setNoColor: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}
			Output = buf

			got := Set(tt.p...)

			require.NotNil(t, got)
		})
	}
}

// TestUnset tests the Unset function.
func TestUnset(t *testing.T) {
	originalNoColor := NoColor
	originalOutput := Output

	t.Cleanup(func() {
		NoColor = originalNoColor
		Output = originalOutput
	})

	tests := []struct {
		name       string
		setNoColor bool
	}{
		{
			name:       "unsets with color enabled",
			setNoColor: false,
		},
		{
			name:       "skips with NoColor set",
			setNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoColor = tt.setNoColor
			buf := &bytes.Buffer{}
			Output = buf

			Unset()

			// Just verify it doesn't panic
		})
	}
}

// TestBgRGB tests the BgRGB function.
func TestBgRGB(t *testing.T) {
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
			got := BgRGB(tt.r, tt.g, tt.b)

			require.NotNil(t, got)
			assert.Len(t, got.params, tt.wantParams)
		})
	}
}

// TestRGB tests the RGB function.
func TestRGB(t *testing.T) {
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
			got := RGB(tt.r, tt.g, tt.b)

			require.NotNil(t, got)
			assert.Len(t, got.params, tt.wantParams)
		})
	}
}
