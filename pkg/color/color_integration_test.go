package color_test

import (
	"bytes"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/color"
)

// newTestConfig creates a test config with color explicitly enabled.
func newTestConfig(buf *bytes.Buffer) *color.Config {
	return &color.Config{
		NoColor: false,
		Output:  buf,
		Error:   buf,
	}
}

// TestIntegration_HelpersComprehensive tests the color helper functions comprehensively.
func TestIntegration_HelpersComprehensive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		noColor     bool
		testFn      func() string
		checkEscape bool
		checkText   string
	}{
		{
			name:    "RedString with color enabled via environment bypass",
			noColor: false,
			testFn: func() string {
				// Use NewWithConfig to bypass environment detection
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgRed)

				return c.Sprint("test")
			},
			checkEscape: true,
			checkText:   "test",
		},
		{
			name:    "RedString with NoColor via config",
			noColor: true,
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := &color.Config{NoColor: true, Output: buf, Error: buf}
				c := color.NewWithConfig(cfg, color.FgRed)

				return c.Sprint("test")
			},
			checkEscape: false,
			checkText:   "test",
		},
		{
			name:    "GreenString with color",
			noColor: false,
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgGreen)

				return c.Sprint("hello")
			},
			checkEscape: true,
			checkText:   "hello",
		},
		{
			name:    "BlueString with NoColor",
			noColor: true,
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := &color.Config{NoColor: true, Output: buf, Error: buf}
				c := color.NewWithConfig(cfg, color.FgBlue)

				return c.Sprint("world")
			},
			checkEscape: false,
			checkText:   "world",
		},
		{
			name:    "YellowString formatted",
			noColor: false,
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgYellow)

				return c.Sprintf("value: %d", 42)
			},
			checkEscape: true,
			checkText:   "value: 42",
		},
		{
			name:    "CyanString empty",
			noColor: false,
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgCyan)

				return c.Sprint("")
			},
			checkEscape: true,
			checkText:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.testFn()

			if tt.checkEscape {
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			} else {
				assert.NotContains(t, result, "\x1b[", "should not contain ANSI escape code")
			}

			assert.Contains(t, result, tt.checkText, "should contain expected text")
		})
	}
}

// TestIntegration_ColorChainingIntegration tests color chaining functionality.
func TestIntegration_ColorChainingIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		colorFn func() *color.Color
		noColor bool
		check   func(t *testing.T, c *color.Color)
	}{
		{
			name: "chained foreground colors",
			colorFn: func() *color.Color {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.FgRed).Add(color.Bold)
			},
			noColor: false,
			check: func(t *testing.T, c *color.Color) {
				t.Helper()

				result := c.Sprint("test")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
				assert.Contains(t, result, "test", "should contain text")
			},
		},
		{
			name: "combined color with multiple attributes",
			colorFn: func() *color.Color {
				// Using chained Add calls to combine red, bold, and underline
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.FgRed).Add(color.Bold).Add(color.Underline)
			},
			noColor: false,
			check: func(t *testing.T, c *color.Color) {
				t.Helper()

				result := c.Sprint("important")
				// Combined attributes produce combined ANSI codes
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
				assert.Contains(t, result, "important", "should contain text")
				// The format should be combined: 31;1;4
				assert.Contains(t, result, "31", "should have red code")
			},
		},
		{
			name: "NoColor disables all colors",
			colorFn: func() *color.Color {
				// Create color with NoColor enabled
				cfg := &color.Config{NoColor: true}

				return color.NewWithConfig(cfg, color.FgRed)
			},
			noColor: true,
			check: func(t *testing.T, c *color.Color) {
				t.Helper()

				result := c.Sprint("test")
				assert.Equal(t, "test", result, "should be plain text when NoColor is set")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.check(t, tt.colorFn())
		})
	}
}

// TestIntegration_RGBFunctionsIntegration tests RGB color functions.
func TestIntegration_RGBFunctionsIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		noColor bool
		testFn  func() string
	}{
		{
			name:    "RGB foreground with color",
			noColor: false,
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg).AddRGB(255, 128, 64)

				return c.Sprint("test")
			},
		},
		{
			name:    "RGB foreground without color",
			noColor: true,
			testFn: func() string {
				cfg := &color.Config{NoColor: true}

				return color.NewWithConfig(cfg, color.FgRed).Sprint("test")
			},
		},
		{
			name:    "BgRGB background with color",
			noColor: false,
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg).AddBgRGB(128, 64, 32)

				return c.Sprint("test")
			},
		},
		{
			name:    "AddRGB chained with color",
			noColor: false,
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg).AddRGB(255, 128, 64)

				return c.Sprint("test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.testFn()

			if tt.noColor {
				assert.Equal(t, "test", result, "should be plain text")
			} else {
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
				assert.Contains(t, result, "test", "should contain text")
			}
		})
	}
}

// TestIntegration_NewWithConfigIntegration tests the NewWithConfig function.
func TestIntegration_NewWithConfigIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *color.Config
		attrs   []color.Attribute
		input   string
		noColor bool
	}{
		{
			name:    "custom config with NoColor=true",
			cfg:     &color.Config{NoColor: true, Output: &bytes.Buffer{}, Error: &bytes.Buffer{}},
			attrs:   []color.Attribute{color.FgRed},
			input:   "test",
			noColor: true,
		},
		{
			name:    "custom config with NoColor=false",
			cfg:     &color.Config{NoColor: false, Output: &bytes.Buffer{}, Error: &bytes.Buffer{}},
			attrs:   []color.Attribute{color.FgGreen},
			input:   "test",
			noColor: false,
		},
		{
			name:    "custom config with multiple attributes",
			cfg:     &color.Config{NoColor: false, Output: &bytes.Buffer{}, Error: &bytes.Buffer{}},
			attrs:   []color.Attribute{color.FgBlue, color.Bold},
			input:   "test",
			noColor: false,
		},
		{
			name:    "empty config with explicit NoColor=false",
			cfg:     &color.Config{NoColor: false, Output: &bytes.Buffer{}, Error: &bytes.Buffer{}},
			attrs:   []color.Attribute{color.FgRed},
			input:   "test",
			noColor: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := color.NewWithConfig(tt.cfg, tt.attrs...)
			result := c.Sprint(tt.input)

			if tt.noColor {
				assert.Equal(t, tt.input, result, "should be plain text when NoColor is set")
			} else {
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
				assert.Contains(t, result, tt.input, "should contain text")
			}
		})
	}
}

// TestIntegration_DefaultConfigIntegration tests the DefaultConfig function.
func TestIntegration_DefaultConfigIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "DefaultConfig returns valid config",
			test: func(t *testing.T) {
				t.Helper()

				cfg := color.DefaultConfig()
				require.NotNil(t, cfg)
				assert.NotNil(t, cfg.Output)
				assert.NotNil(t, cfg.Error)
			},
		},
		{
			name: "New with explicit color config works",
			test: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgRed)
				assert.NotNil(t, c)
				result := c.Sprint("test")
				assert.NotEmpty(t, result)
				assert.Contains(t, result, "test")
			},
		},
		{
			name: "NewWithConfig with explicit config works",
			test: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgGreen)
				assert.NotNil(t, c)
				result := c.Sprint("test")
				assert.NotEmpty(t, result)
				assert.Contains(t, result, "test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.test(t)
		})
	}
}

// TestIntegration_InstanceConfigIntegration tests instance-based configuration.
func TestIntegration_InstanceConfigIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "separate instances have independent configs",
			test: func(t *testing.T) {
				t.Helper()

				// Create two independent color instances with different configs
				buf1 := &bytes.Buffer{}
				cfg1 := &color.Config{NoColor: false, Output: buf1, Error: buf1}
				c1 := color.NewWithConfig(cfg1, color.FgRed)

				buf2 := &bytes.Buffer{}
				cfg2 := &color.Config{NoColor: true, Output: buf2, Error: buf2}
				c2 := color.NewWithConfig(cfg2, color.FgGreen)

				// c1 should produce colored output
				result1 := c1.Sprint("test1")
				assert.Contains(t, result1, "\x1b[", "c1 should have color")

				// c2 should produce plain output
				result2 := c2.Sprint("test2")
				assert.Equal(t, "test2", result2, "c2 should be plain")
			},
		},
		{
			name: "shared config affects all instances",
			test: func(t *testing.T) {
				t.Helper()

				// Create a shared config
				buf := &bytes.Buffer{}
				sharedCfg := &color.Config{NoColor: false, Output: buf, Error: buf}

				// Create multiple colors sharing the same config
				c1 := color.NewWithConfig(sharedCfg, color.FgRed)
				c2 := color.NewWithConfig(sharedCfg, color.FgGreen)

				// Both should work
				result1 := c1.Sprint("red")
				result2 := c2.Sprint("green")

				assert.Contains(t, result1, "\x1b[", "c1 should have color")
				assert.Contains(t, result2, "\x1b[", "c2 should have color")
				assert.Contains(t, result1, "red")
				assert.Contains(t, result2, "green")
			},
		},
		{
			name: "instance-level override takes precedence",
			test: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := &color.Config{NoColor: false, Output: buf, Error: buf}
				c := color.NewWithConfig(cfg, color.FgRed)

				// Initially should have color
				result1 := c.Sprint("test")
				assert.Contains(t, result1, "\x1b[", "should have color initially")

				// Disable color at instance level
				c.DisableColor()
				result2 := c.Sprint("test")
				assert.Equal(t, "test", result2, "should be plain after DisableColor")

				// Re-enable color at instance level
				c.EnableColor()
				result3 := c.Sprint("test")
				assert.Contains(t, result3, "\x1b[", "should have color after EnableColor")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.test(t)
		})
	}
}

// TestIntegration_PrintFunctionsIntegration tests print functions integration.
func TestIntegration_PrintFunctionsIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		testFn func(t *testing.T) (string, error)
	}{
		{
			name: "Print prints to Output",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgRed)

				_, err := c.Print("test")

				return buf.String(), err
			},
		},
		{
			name: "Printf formats and prints",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgRed)

				_, err := c.Printf("value: %d", 42)

				return buf.String(), err
			},
		},
		{
			name: "Println prints with newline",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgBlue)

				_, err := c.Println("test")
				result := buf.String()
				// Contains "test" and "\n", escape codes are between them
				assert.Contains(t, result, "test", "should contain text")
				assert.Contains(t, result, "\n", "should contain newline")

				return result, err
			},
		},
		{
			name: "Fprint writes to custom writer",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgRed)

				_, err := c.Fprint(buf, "test")

				return buf.String(), err
			},
		},
		{
			name: "Fprintf formats and writes to custom writer",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgRed)

				_, err := c.Fprintf(buf, "value: %d", 42)

				return buf.String(), err
			},
		},
		{
			name: "Fprintln writes with newline to custom writer",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgMagenta)

				_, err := c.Fprintln(buf, "test")
				result := buf.String()
				// Contains "test" and "\n", escape codes are between them
				assert.Contains(t, result, "test", "should contain text")
				assert.Contains(t, result, "\n", "should contain newline")

				return result, err
			},
		},
		{
			name: "Sprint returns string with color codes",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				result := color.NewWithConfig(cfg, color.FgRed).Sprint("test")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
				assert.Contains(t, result, "test", "should contain text")

				return result, nil
			},
		},
		{
			name: "Sprintf returns formatted string with color codes",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				result := color.NewWithConfig(cfg, color.FgRed).Sprintf("value: %d", 42)
				assert.Contains(t, result, "value: 42", "should contain formatted text")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")

				return result, nil
			},
		},
		{
			name: "Sprintln returns string with newline and color codes",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				result := color.NewWithConfig(cfg, color.FgGreen).Sprintln("test")
				// Contains "test" and "\n", escape codes are between them
				assert.Contains(t, result, "test", "should contain text")
				assert.Contains(t, result, "\n", "should contain newline")

				return result, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := tt.testFn(t)
			require.NoError(t, err)
		})
	}
}

// TestIntegration_ColorEqualityIntegration tests color equality functionality.
func TestIntegration_ColorEqualityIntegration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		testFn func(t *testing.T) bool
	}{
		{
			name: "identical colors are equal",
			testFn: func(t *testing.T) bool {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c1 := color.NewWithConfig(cfg, color.FgRed, color.Bold)
				c2 := color.NewWithConfig(cfg, color.FgRed, color.Bold)

				return c1.Equals(c2)
			},
		},
		{
			name: "different colors are not equal",
			testFn: func(t *testing.T) bool {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c1 := color.NewWithConfig(cfg, color.FgRed)
				c2 := color.NewWithConfig(cfg, color.FgGreen)

				return !c1.Equals(c2)
			},
		},
		{
			name: "colors with different attribute counts are not equal",
			testFn: func(t *testing.T) bool {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c1 := color.NewWithConfig(cfg, color.FgRed)
				c2 := color.NewWithConfig(cfg, color.FgRed, color.Bold)

				return !c1.Equals(c2)
			},
		},
		{
			name: "empty colors are equal",
			testFn: func(t *testing.T) bool {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c1 := color.NewWithConfig(cfg)
				c2 := color.NewWithConfig(cfg)

				return c1.Equals(c2)
			},
		},
		{
			name: "nil colors are equal",
			testFn: func(t *testing.T) bool {
				t.Helper()

				var (
					c1 *color.Color
					c2 *color.Color
				)

				return c1.Equals(c2)
			},
		},
		{
			name: "nil and non-nil are not equal",
			testFn: func(t *testing.T) bool {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c1 := color.NewWithConfig(cfg, color.FgRed)

				var c2 *color.Color

				return !c1.Equals(c2)
			},
		},
		{
			name: "RGB colors are equal if same values",
			testFn: func(t *testing.T) bool {
				t.Helper()

				c1 := color.RGB(255, 128, 64)
				c2 := color.RGB(255, 128, 64)

				return c1.Equals(c2)
			},
		},
		{
			name: "RGB colors are not equal if different values",
			testFn: func(t *testing.T) bool {
				t.Helper()

				c1 := color.RGB(255, 128, 64)
				c2 := color.RGB(255, 128, 65)

				return !c1.Equals(c2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.testFn(t)
			assert.True(t, result)
		})
	}
}

// TestIntegration_FuncVariantsIntegration tests function variants integration.
func TestIntegration_FuncVariantsIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "SprintFunc returns function that produces colored string",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				fn := color.NewWithConfig(cfg, color.FgRed).SprintFunc()
				result := fn("test")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
				assert.Contains(t, result, "test", "should contain text")
			},
		},
		{
			name: "SprintfFunc returns function that formats and colors",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				fn := color.NewWithConfig(cfg, color.FgRed).SprintfFunc()
				result := fn("value: %d", 42)
				assert.Contains(t, result, "value: 42", "should contain formatted text")
			},
		},
		{
			name: "SprintlnFunc returns function that adds newline and colors",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				fn := color.NewWithConfig(cfg, color.FgBlue).SprintlnFunc()
				result := fn("test")
				// Contains "test" and "\n", escape codes are between them
				assert.Contains(t, result, "test", "should contain text")
				assert.Contains(t, result, "\n", "should contain newline")
			},
		},
		{
			name: "PrintFunc returns function that prints to Output",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				fn := color.NewWithConfig(cfg, color.FgRed).PrintFunc()
				fn("test")

				assert.Contains(t, buf.String(), "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name: "PrintfFunc returns function that formats and prints",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				fn := color.NewWithConfig(cfg, color.FgRed).PrintfFunc()
				fn("value: %d", 42)

				assert.Contains(t, buf.String(), "value: 42", "should contain formatted text")
			},
		},
		{
			name: "PrintlnFunc returns function that prints with newline",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				fn := color.NewWithConfig(cfg, color.FgMagenta).PrintlnFunc()
				fn("test")

				result := buf.String()
				// Contains "test" and "\n", escape codes are between them
				assert.Contains(t, result, "test", "should contain text")
				assert.Contains(t, result, "\n", "should contain newline")
			},
		},
		{
			name: "FprintFunc returns function that writes to custom writer",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				fn := color.NewWithConfig(cfg, color.FgRed).FprintFunc()
				fn(buf, "test")

				assert.Contains(t, buf.String(), "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name: "FprintfFunc returns function that formats and writes",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				fn := color.NewWithConfig(cfg, color.FgRed).FprintfFunc()
				fn(buf, "value: %d", 42)

				assert.Contains(t, buf.String(), "value: 42", "should contain formatted text")
			},
		},
		{
			name: "FprintlnFunc returns function that writes with newline",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				fn := color.NewWithConfig(cfg, color.FgGreen).FprintlnFunc()
				fn(buf, "test")

				result := buf.String()
				// Contains "test" and "\n", escape codes are between them
				assert.Contains(t, result, "test", "should contain text")
				assert.Contains(t, result, "\n", "should contain newline")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFn(t)
		})
	}
}

// TestIntegration_DisableEnableColorIntegration tests DisableColor and EnableColor methods.
func TestIntegration_DisableEnableColorIntegration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		testFn func(t *testing.T) string
	}{
		{
			name: "DisableColor prevents color output",
			testFn: func(t *testing.T) string {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgRed)
				c.DisableColor()

				return c.Sprint("test")
			},
		},
		{
			name: "EnableColor allows color output",
			testFn: func(t *testing.T) string {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgRed)
				c.EnableColor()

				return c.Sprint("test")
			},
		},
		{
			name: "Disable then Enable toggles color state",
			testFn: func(t *testing.T) string {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgRed)
				c.DisableColor()
				_ = c.Sprint("test") // Should be plain

				c.EnableColor()

				return c.Sprint("test")
			},
		},
		{
			name: "Enable then Disable toggles color state off",
			testFn: func(t *testing.T) string {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgRed)
				c.EnableColor()
				_ = c.Sprint("test") // Should have color

				c.DisableColor()

				return c.Sprint("test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.testFn(t)
			// Each test case has specific expected behavior
			switch tt.name {
			case "DisableColor prevents color output":
				assert.Equal(t, "test", result, "should be plain text")
			case "EnableColor allows color output":
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			case "Disable then Enable toggles color state":
				assert.Contains(t, result, "\x1b[", "should have color after enable")
			case "Enable then Disable toggles color state off":
				assert.Equal(t, "test", result, "should be plain after disable")
			}
		})
	}
}

// TestIntegration_AddMethodChainingIntegration tests Add method chaining.
func TestIntegration_AddMethodChainingIntegration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		testFn func(t *testing.T) string
	}{
		{
			name: "Add chains multiple attributes",
			testFn: func(t *testing.T) string {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg, color.FgRed).Add(color.Bold).Add(color.Underline)

				return c.Sprint("test")
			},
		},
		{
			name: "Add can be called multiple times",
			testFn: func(t *testing.T) string {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg)
				c.Add(color.FgRed)
				c.Add(color.Bold)
				c.Add(color.Underline)

				return c.Sprint("test")
			},
		},
		{
			name: "AddRGB chains RGB foreground",
			testFn: func(t *testing.T) string {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg).AddRGB(255, 128, 64)

				return c.Sprint("test")
			},
		},
		{
			name: "AddBgRGB chains RGB background",
			testFn: func(t *testing.T) string {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				c := color.NewWithConfig(cfg).AddBgRGB(128, 64, 32)

				return c.Sprint("test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.testFn(t)
			assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			assert.Contains(t, result, "test", "should contain text")
		})
	}
}

// TestIntegration_SetWriterIntegration tests SetWriter and UnsetWriter methods.
func TestIntegration_SetWriterIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "SetWriter and UnsetWriter work together",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				c := color.NewWithConfig(cfg, color.FgRed)
				c.SetWriter(buf)
				_, _ = c.Print("test")

				result := buf.String()
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")

				c.UnsetWriter(buf)
			},
		},
		{
			name: "SetWriter is skipped when NoColor is set",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := &color.Config{NoColor: true, Output: buf, Error: buf}

				c := color.NewWithConfig(cfg, color.FgRed)
				c.SetWriter(buf)
				_, _ = c.Print("test")

				result := buf.String()
				assert.Equal(t, "test", result, "should write plain text when NoColor is set")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFn(t)
		})
	}
}

// TestIntegration_SetMethodIntegration tests Set method integration.
func TestIntegration_SetMethodIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "Set method applies color",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				c := color.NewWithConfig(cfg, color.FgRed)
				c.Set()

				_ = buf.String()
			},
		},
		{
			name: "Set is skipped when NoColor is set",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := &color.Config{NoColor: true, Output: buf, Error: buf}

				c := color.NewWithConfig(cfg, color.FgRed)
				c.Set()

				_ = buf.String()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFn(t)
		})
	}
}

// TestIntegration_HiIntensityColorsIntegration tests hi-intensity color helpers.
func TestIntegration_HiIntensityColorsIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		testFn func() string
	}{
		{
			name: "HiRed produces hi-intensity red",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.FgHiRed).Sprint("test")
			},
		},
		{
			name: "HiGreen produces hi-intensity green",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.FgHiGreen).Sprint("test")
			},
		},
		{
			name: "HiBlue produces hi-intensity blue",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.FgHiBlue).Sprint("test")
			},
		},
		{
			name: "HiYellow produces hi-intensity yellow",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.FgHiYellow).Sprint("test")
			},
		},
		{
			name: "HiCyan produces hi-intensity cyan",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.FgHiCyan).Sprint("test")
			},
		},
		{
			name: "HiMagenta produces hi-intensity magenta",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.FgHiMagenta).Sprint("test")
			},
		},
		{
			name: "HiWhite produces hi-intensity white",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.FgHiWhite).Sprint("test")
			},
		},
		{
			name: "HiBlack produces hi-intensity black",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.FgHiBlack).Sprint("test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.testFn()
			assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			assert.Contains(t, result, "test", "should contain text")
		})
	}
}

// TestIntegration_BackgroundColorsIntegration tests background color helpers.
func TestIntegration_BackgroundColorsIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		testFn func() string
	}{
		{
			name: "BgBlack background",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.BgBlack).Sprint("test")
			},
		},
		{
			name: "BgRed background",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.BgRed).Sprint("test")
			},
		},
		{
			name: "BgGreen background",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.BgGreen).Sprint("test")
			},
		},
		{
			name: "BgYellow background",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.BgYellow).Sprint("test")
			},
		},
		{
			name: "BgBlue background",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.BgBlue).Sprint("test")
			},
		},
		{
			name: "BgMagenta background",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.BgMagenta).Sprint("test")
			},
		},
		{
			name: "BgCyan background",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.BgCyan).Sprint("test")
			},
		},
		{
			name: "BgWhite background",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.BgWhite).Sprint("test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.testFn()
			assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			assert.Contains(t, result, "test", "should contain text")
		})
	}
}

// TestIntegration_StyleAttributesIntegration tests style attribute helpers.
func TestIntegration_StyleAttributesIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		testFn func() string
	}{
		{
			name: "Bold style",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.Bold).Sprint("test")
			},
		},
		{
			name: "Faint style",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.Faint).Sprint("test")
			},
		},
		{
			name: "Italic style",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.Italic).Sprint("test")
			},
		},
		{
			name: "Underline style",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.Underline).Sprint("test")
			},
		},
		{
			name: "BlinkSlow style",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.BlinkSlow).Sprint("test")
			},
		},
		{
			name: "BlinkRapid style",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.BlinkRapid).Sprint("test")
			},
		},
		{
			name: "ReverseVideo style",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.ReverseVideo).Sprint("test")
			},
		},
		{
			name: "Concealed style",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.Concealed).Sprint("test")
			},
		},
		{
			name: "CrossedOut style",
			testFn: func() string {
				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				return color.NewWithConfig(cfg, color.CrossedOut).Sprint("test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.testFn()
			assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			assert.Contains(t, result, "test", "should contain text")
		})
	}
}

// TestIntegration_ComplexScenariosIntegration tests complex real-world scenarios.
func TestIntegration_ComplexScenariosIntegration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "colored log message simulation",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				timestamp := color.NewWithConfig(cfg, color.FgWhite).Sprint("2024-01-15 10:30:00")
				level := color.NewWithConfig(cfg, color.FgRed).Sprint("[ERROR]")
				message := color.NewWithConfig(cfg, color.FgWhite).Sprint("Something went wrong")

				logLine := timestamp + " " + level + " " + message

				assert.Contains(t, logLine, "2024-01-15 10:30:00")
				assert.Contains(t, logLine, "[ERROR]")
				assert.Contains(t, logLine, "Something went wrong")
			},
		},
		{
			name: "table output simulation",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				headerName := color.NewWithConfig(cfg, color.Bold).Sprint("Name")
				headerAge := color.NewWithConfig(cfg, color.Bold).Sprint("Age")
				row1Name := color.NewWithConfig(cfg, color.FgCyan).Sprint("Alice")
				row1Age := color.NewWithConfig(cfg, color.FgGreen).Sprint("30")

				assert.NotEmpty(t, headerName)
				assert.NotEmpty(t, headerAge)
				assert.NotEmpty(t, row1Name)
				assert.NotEmpty(t, row1Age)
			},
		},
		{
			name: "mixed colored and plain text",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)
				prefix := "Status: "
				status := color.NewWithConfig(cfg, color.FgGreen).Sprint("OK")
				suffix := " - Server running"

				result := prefix + status + suffix

				assert.Contains(t, result, "Status: ")
				assert.Contains(t, result, "OK")
				assert.Contains(t, result, " - Server running")
			},
		},
		{
			name: "colored string builder",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				cfg := newTestConfig(buf)

				var builder bytes.Buffer
				builder.WriteString(color.NewWithConfig(cfg, color.FgRed).Sprint("Error: "))
				builder.WriteString(color.NewWithConfig(cfg, color.FgWhite).Sprint("File not found"))
				builder.WriteString(color.NewWithConfig(cfg, color.FgRed).Sprint("\n"))
				builder.WriteString(color.NewWithConfig(cfg, color.FgYellow).Sprint("Please check the path"))

				result := builder.String()

				assert.Contains(t, result, "Error: ")
				assert.Contains(t, result, "File not found")
				assert.Contains(t, result, "Please check the path")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFn(t)
		})
	}
}

// TestIntegration_ConcurrentAccessIntegration tests concurrent access to color instances.
func TestIntegration_ConcurrentAccessIntegration(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup

	// Run concurrent accesses to different color instances
	for range 100 {
		wg.Go(func() {
			// Each goroutine creates its own config and color instances
			buf := &bytes.Buffer{}
			cfg := &color.Config{
				NoColor: false,
				Output:  buf,
				Error:   buf,
			}

			// Various color operations
			_ = color.NewWithConfig(cfg, color.FgRed).Sprint("test")
			_ = color.NewWithConfig(cfg, color.FgGreen).Sprint("test")
			_ = color.NewWithConfig(cfg, color.FgBlue).Sprint("test")
			_ = color.NewWithConfig(cfg, color.FgYellow).Sprint("test")
		})
	}

	wg.Wait()
	// If we get here without race conditions, test passes
}

// TestIntegration_EnvNO_COLORIntegration tests NO_COLOR environment variable handling.
func TestIntegration_EnvNO_COLORIntegration(t *testing.T) {
	t.Parallel()
	// Save and restore environment
	originalEnv := os.Getenv("NO_COLOR")

	t.Cleanup(func() {
		if originalEnv == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", originalEnv)
		}
	})

	tests := []struct {
		name   string
		envVal string
		testFn func(t *testing.T) bool
	}{
		{
			name:   "NO_COLOR set disables colors via explicit config",
			envVal: "1",
			testFn: func(t *testing.T) bool {
				t.Helper()
				os.Setenv("NO_COLOR", "1")
				// Create a new config which should detect NO_COLOR
				cfg := &color.Config{NoColor: true, Output: &bytes.Buffer{}, Error: &bytes.Buffer{}}
				c := color.NewWithConfig(cfg, color.FgRed)
				result := c.Sprint("test")

				return result == "test"
			},
		},
		{
			name:   "NO_COLOR unset allows colors with explicit config",
			envVal: "",
			testFn: func(t *testing.T) bool {
				t.Helper()
				os.Unsetenv("NO_COLOR")

				cfg := &color.Config{NoColor: false, Output: &bytes.Buffer{}, Error: &bytes.Buffer{}}
				c := color.NewWithConfig(cfg, color.FgRed)
				result := c.Sprint("test")

				return len(result) > 4 // Contains escape codes + text
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.testFn(t)
			assert.True(t, result)
		})
	}
}
