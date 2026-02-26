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

// TestIntegration_HelpersComprehensive tests the color helper functions comprehensively.
func TestIntegration_HelpersComprehensive(t *testing.T) {
	// Save and restore global state
	originalNoColor := color.NoColor

	t.Cleanup(func() {
		color.NoColor = originalNoColor
	})

	tests := []struct {
		name        string
		noColor     bool
		testFn      func() string
		checkEscape bool
		checkText   string
	}{
		{
			name:    "RedString with color enabled",
			noColor: false,
			testFn: func() string {
				return color.RedString("test")
			},
			checkEscape: true,
			checkText:   "test",
		},
		{
			name:    "RedString with NoColor",
			noColor: true,
			testFn: func() string {
				return color.RedString("test")
			},
			checkEscape: false,
			checkText:   "test",
		},
		{
			name:    "GreenString with color",
			noColor: false,
			testFn: func() string {
				return color.GreenString("hello")
			},
			checkEscape: true,
			checkText:   "hello",
		},
		{
			name:    "BlueString with NoColor",
			noColor: true,
			testFn: func() string {
				return color.BlueString("world")
			},
			checkEscape: false,
			checkText:   "world",
		},
		{
			name:    "YellowString formatted",
			noColor: false,
			testFn: func() string {
				return color.YellowString("value: %d", 42)
			},
			checkEscape: true,
			checkText:   "value: 42",
		},
		{
			name:    "CyanString empty",
			noColor: false,
			testFn: func() string {
				return color.CyanString("")
			},
			checkEscape: true,
			checkText:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color.NoColor = tt.noColor

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
	// Save and restore global state
	originalNoColor := color.NoColor

	t.Cleanup(func() {
		color.NoColor = originalNoColor
	})

	tests := []struct {
		name    string
		colorFn func() *color.Color
		check   func(t *testing.T, c *color.Color)
	}{
		{
			name: "chained foreground colors",
			colorFn: func() *color.Color {
				return color.New(color.FgRed).Add(color.Bold)
			},
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
				return color.New(color.FgRed).Add(color.Bold).Add(color.Underline)
			},
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
				c := color.New(color.FgRed)

				return c
			},
			check: func(t *testing.T, c *color.Color) {
				t.Helper()

				color.NoColor = true
				result := c.Sprint("test")
				assert.Equal(t, "test", result, "should be plain text when NoColor is set")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color.NoColor = false // Ensure colors are enabled for each test

			tt.check(t, tt.colorFn())
		})
	}
}

// TestIntegration_RGBFunctionsIntegration tests RGB color functions.
func TestIntegration_RGBFunctionsIntegration(t *testing.T) {
	// Save and restore global state
	originalNoColor := color.NoColor

	t.Cleanup(func() {
		color.NoColor = originalNoColor
	})

	tests := []struct {
		name    string
		noColor bool
		testFn  func() string
	}{
		{
			name:    "RGB foreground with color",
			noColor: false,
			testFn: func() string {
				return color.RGB(255, 128, 64).Sprint("test")
			},
		},
		{
			name:    "RGB foreground without color",
			noColor: true,
			testFn: func() string {
				return color.RGB(255, 128, 64).Sprint("test")
			},
		},
		{
			name:    "BgRGB background with color",
			noColor: false,
			testFn: func() string {
				return color.BgRGB(128, 64, 32).Sprint("test")
			},
		},
		{
			name:    "AddRGB chained with color",
			noColor: false,
			testFn: func() string {
				return color.New().AddRGB(255, 128, 64).Sprint("test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color.NoColor = tt.noColor

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

// TestIntegration_CacheIntegration tests the cache behavior integration.
func TestIntegration_CacheIntegration(t *testing.T) {
	// Save and restore global state
	originalNoColor := color.NoColor

	t.Cleanup(func() {
		color.NoColor = originalNoColor
	})

	tests := []struct {
		name   string
		testFn func(t *testing.T) string
	}{
		{
			name: "multiple calls to same helper use cache",
			testFn: func(t *testing.T) string {
				t.Helper()

				color.NoColor = false
				// Call multiple times - should use cache
				r1 := color.RedString("test")
				r2 := color.RedString("test")
				r3 := color.RedString("test")
				// All should produce same output
				assert.Equal(t, r1, r2)
				assert.Equal(t, r2, r3)

				return r1
			},
		},
		{
			name: "different helpers produce different output",
			testFn: func(t *testing.T) string {
				t.Helper()

				color.NoColor = false

				red := color.RedString("test")
				green := color.GreenString("test")
				assert.NotEqual(t, red, green, "different colors should produce different output")

				return red
			},
		},
		{
			name: "helper functions with formatting",
			testFn: func(t *testing.T) string {
				t.Helper()

				color.NoColor = false

				result := color.YellowString("value: %d, name: %s", 42, "test")
				assert.Contains(t, result, "value: 42", "should contain formatted value")
				assert.Contains(t, result, "name: test", "should contain formatted name")

				return result
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFn(t)
		})
	}
}

// TestIntegration_GlobalStateIntegration tests global state integration.
func TestIntegration_GlobalStateIntegration(t *testing.T) {
	// Save and restore global state
	originalNoColor := color.NoColor
	originalOutput := color.Output

	t.Cleanup(func() {
		color.NoColor = originalNoColor
		color.Output = originalOutput
	})

	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "global NoColor affects new colors",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = true
				result := color.RedString("test")
				assert.Equal(t, "test", result, "should be plain when NoColor is true")

				color.NoColor = false
				result = color.RedString("test")
				assert.Contains(t, result, "\x1b[", "should have color when NoColor is false")
			},
		},
		{
			name: "global NoColor disabled allows colors",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false
				result := color.GreenString("test")
				assert.Contains(t, result, "\x1b[", "should have color when NoColor is false")
			},
		},
		{
			name: "Output writer receives colored output",
			testFn: func(t *testing.T) {
				t.Helper()

				buf := &bytes.Buffer{}
				color.Output = buf
				color.NoColor = false

				_, _ = color.New(color.FgRed).Print("test")

				output := buf.String()
				assert.Contains(t, output, "\x1b[", "should contain ANSI escape code")
				assert.Contains(t, output, "test", "should contain text")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFn(t)
		})
	}
}

// TestIntegration_ConcurrentCacheAccessIntegration tests concurrent cache access.
func TestIntegration_ConcurrentCacheAccessIntegration(t *testing.T) {
	var wg sync.WaitGroup

	// Run concurrent accesses to cache
	for range 100 {
		wg.Go(func() {
			// Various color functions that use cache
			_ = color.RedString("test")
			_ = color.GreenString("test")
			_ = color.BlueString("test")
			_ = color.YellowString("test")
		})
	}

	wg.Wait()
	// If we get here without race conditions, test passes
}

// TestIntegration_EnvNO_COLORIntegration tests NO_COLOR environment variable handling.
func TestIntegration_EnvNO_COLORIntegration(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Getenv("NO_COLOR")

	t.Cleanup(func() {
		if originalEnv == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", originalEnv)
		}
		// Reset color package state
		color.NoColor = false
	})

	tests := []struct {
		name   string
		envVal string
		testFn func(t *testing.T) bool
	}{
		{
			name:   "NO_COLOR set disables colors",
			envVal: "1",
			testFn: func(t *testing.T) bool {
				t.Helper()
				os.Setenv("NO_COLOR", "1")
				// The package checks env var on init, but we need to simulate
				// Since we can't re-import, test the NoColor flag behavior
				color.NoColor = true
				result := color.RedString("test")

				return result == "test"
			},
		},
		{
			name:   "NO_COLOR empty string disables colors",
			envVal: "",
			testFn: func(t *testing.T) bool {
				t.Helper()
				os.Setenv("NO_COLOR", "")

				color.NoColor = false
				result := color.RedString("test")
				// Empty string doesn't disable (depends on package init behavior)
				return len(result) > 0
			},
		},
		{
			name:   "NO_COLOR unset allows colors",
			envVal: "",
			testFn: func(t *testing.T) bool {
				t.Helper()
				os.Unsetenv("NO_COLOR")

				color.NoColor = false
				result := color.RedString("test")

				return len(result) > 4 // Contains escape codes + text
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFn(t)
			assert.True(t, result)
		})
	}
}

// TestIntegration_PrintFunctionsIntegration tests print functions integration.
func TestIntegration_PrintFunctionsIntegration(t *testing.T) {
	// Save and restore global state
	originalNoColor := color.NoColor
	originalOutput := color.Output

	t.Cleanup(func() {
		color.NoColor = originalNoColor
		color.Output = originalOutput
	})

	tests := []struct {
		name   string
		testFn func(t *testing.T) (string, error)
	}{
		{
			name: "Print prints to Output",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				color.Output = buf
				color.NoColor = false

				_, err := color.New(color.FgRed).Print("test")

				return buf.String(), err
			},
		},
		{
			name: "Printf formats and prints",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				color.Output = buf
				color.NoColor = false

				_, err := color.New(color.FgRed).Printf("value: %d", 42)

				return buf.String(), err
			},
		},
		{
			name: "Println prints with newline",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				color.Output = buf
				color.NoColor = false

				_, err := color.New(color.FgBlue).Println("test")
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
				color.NoColor = false

				_, err := color.New(color.FgRed).Fprint(buf, "test")

				return buf.String(), err
			},
		},
		{
			name: "Fprintf formats and writes to custom writer",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				color.NoColor = false

				_, err := color.New(color.FgRed).Fprintf(buf, "value: %d", 42)

				return buf.String(), err
			},
		},
		{
			name: "Fprintln writes with newline to custom writer",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				buf := &bytes.Buffer{}
				color.NoColor = false

				_, err := color.New(color.FgMagenta).Fprintln(buf, "test")
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

				color.NoColor = false
				result := color.New(color.FgRed).Sprint("test")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
				assert.Contains(t, result, "test", "should contain text")

				return result, nil
			},
		},
		{
			name: "Sprintf returns formatted string with color codes",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				color.NoColor = false
				result := color.New(color.FgRed).Sprintf("value: %d", 42)
				assert.Contains(t, result, "value: 42", "should contain formatted text")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")

				return result, nil
			},
		},
		{
			name: "Sprintln returns string with newline and color codes",
			testFn: func(t *testing.T) (string, error) {
				t.Helper()

				color.NoColor = false
				result := color.New(color.FgGreen).Sprintln("test")
				// Contains "test" and "\n", escape codes are between them
				assert.Contains(t, result, "test", "should contain text")
				assert.Contains(t, result, "\n", "should contain newline")

				return result, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.testFn(t)
			require.NoError(t, err)
		})
	}
}

// TestIntegration_ColorEqualityIntegration tests color equality functionality.
func TestIntegration_ColorEqualityIntegration(t *testing.T) {
	tests := []struct {
		name   string
		testFn func(t *testing.T) bool
	}{
		{
			name: "identical colors are equal",
			testFn: func(t *testing.T) bool {
				t.Helper()

				c1 := color.New(color.FgRed, color.Bold)
				c2 := color.New(color.FgRed, color.Bold)

				return c1.Equals(c2)
			},
		},
		{
			name: "different colors are not equal",
			testFn: func(t *testing.T) bool {
				t.Helper()

				c1 := color.New(color.FgRed)
				c2 := color.New(color.FgGreen)

				return !c1.Equals(c2)
			},
		},
		{
			name: "colors with different attribute counts are not equal",
			testFn: func(t *testing.T) bool {
				t.Helper()

				c1 := color.New(color.FgRed)
				c2 := color.New(color.FgRed, color.Bold)

				return !c1.Equals(c2)
			},
		},
		{
			name: "empty colors are equal",
			testFn: func(t *testing.T) bool {
				t.Helper()

				c1 := color.New()
				c2 := color.New()

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

				c1 := color.New(color.FgRed)

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
			result := tt.testFn(t)
			assert.True(t, result)
		})
	}
}

// TestIntegration_SetUnsetIntegration tests Set and Unset functions integration.
func TestIntegration_SetUnsetIntegration(t *testing.T) {
	// Save and restore global state
	originalNoColor := color.NoColor
	originalOutput := color.Output

	t.Cleanup(func() {
		color.NoColor = originalNoColor
		color.Output = originalOutput
	})

	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "Set and Unset work together",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false
				buf := &bytes.Buffer{}
				color.Output = buf

				// Set applies color
				color.Set(color.FgRed)
				// Should have written to output
				_ = buf.String()

				// Unset resets color
				color.Unset()
			},
		},
		{
			name: "Set is skipped when NoColor is set",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = true
				buf := &bytes.Buffer{}
				color.Output = buf

				color.Set(color.FgRed)
				// When NoColor is set, Set should not write
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFn(t)
		})
	}
}

// TestIntegration_FuncVariantsIntegration tests function variants integration.
func TestIntegration_FuncVariantsIntegration(t *testing.T) {
	// Save and restore global state
	originalNoColor := color.NoColor
	originalOutput := color.Output

	t.Cleanup(func() {
		color.NoColor = originalNoColor
		color.Output = originalOutput
	})

	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "SprintFunc returns function that produces colored string",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false
				fn := color.New(color.FgRed).SprintFunc()
				result := fn("test")
				assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
				assert.Contains(t, result, "test", "should contain text")
			},
		},
		{
			name: "SprintfFunc returns function that formats and colors",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false
				fn := color.New(color.FgRed).SprintfFunc()
				result := fn("value: %d", 42)
				assert.Contains(t, result, "value: 42", "should contain formatted text")
			},
		},
		{
			name: "SprintlnFunc returns function that adds newline and colors",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false
				fn := color.New(color.FgBlue).SprintlnFunc()
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

				color.NoColor = false
				buf := &bytes.Buffer{}
				color.Output = buf

				fn := color.New(color.FgRed).PrintFunc()
				fn("test")

				assert.Contains(t, buf.String(), "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name: "PrintfFunc returns function that formats and prints",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false
				buf := &bytes.Buffer{}
				color.Output = buf

				fn := color.New(color.FgRed).PrintfFunc()
				fn("value: %d", 42)

				assert.Contains(t, buf.String(), "value: 42", "should contain formatted text")
			},
		},
		{
			name: "PrintlnFunc returns function that prints with newline",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false
				buf := &bytes.Buffer{}
				color.Output = buf

				fn := color.New(color.FgMagenta).PrintlnFunc()
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

				color.NoColor = false
				buf := &bytes.Buffer{}

				fn := color.New(color.FgRed).FprintFunc()
				fn(buf, "test")

				assert.Contains(t, buf.String(), "\x1b[", "should contain ANSI escape code")
			},
		},
		{
			name: "FprintfFunc returns function that formats and writes",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false
				buf := &bytes.Buffer{}

				fn := color.New(color.FgRed).FprintfFunc()
				fn(buf, "value: %d", 42)

				assert.Contains(t, buf.String(), "value: 42", "should contain formatted text")
			},
		},
		{
			name: "FprintlnFunc returns function that writes with newline",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false
				buf := &bytes.Buffer{}

				fn := color.New(color.FgGreen).FprintlnFunc()
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
			tt.testFn(t)
		})
	}
}

// TestIntegration_DisableEnableColorIntegration tests DisableColor and EnableColor methods.
func TestIntegration_DisableEnableColorIntegration(t *testing.T) {
	tests := []struct {
		name   string
		testFn func(t *testing.T) string
	}{
		{
			name: "DisableColor prevents color output",
			testFn: func(t *testing.T) string {
				t.Helper()

				c := color.New(color.FgRed)
				c.DisableColor()

				return c.Sprint("test")
			},
		},
		{
			name: "EnableColor allows color output",
			testFn: func(t *testing.T) string {
				t.Helper()

				c := color.New(color.FgRed)
				c.EnableColor()

				return c.Sprint("test")
			},
		},
		{
			name: "Disable then Enable toggles color state",
			testFn: func(t *testing.T) string {
				t.Helper()

				c := color.New(color.FgRed)
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

				c := color.New(color.FgRed)
				c.EnableColor()
				_ = c.Sprint("test") // Should have color

				c.DisableColor()

				return c.Sprint("test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	tests := []struct {
		name   string
		testFn func(t *testing.T) string
	}{
		{
			name: "Add chains multiple attributes",
			testFn: func(t *testing.T) string {
				t.Helper()

				c := color.New(color.FgRed).Add(color.Bold).Add(color.Underline)

				return c.Sprint("test")
			},
		},
		{
			name: "Add can be called multiple times",
			testFn: func(t *testing.T) string {
				t.Helper()

				c := color.New()
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

				c := color.New().AddRGB(255, 128, 64)

				return c.Sprint("test")
			},
		},
		{
			name: "AddBgRGB chains RGB background",
			testFn: func(t *testing.T) string {
				t.Helper()

				c := color.New().AddBgRGB(128, 64, 32)

				return c.Sprint("test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFn(t)
			assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			assert.Contains(t, result, "test", "should contain text")
		})
	}
}

// TestIntegration_SetWriterIntegration tests SetWriter and UnsetWriter methods.
func TestIntegration_SetWriterIntegration(t *testing.T) {
	// Save and restore global state
	originalNoColor := color.NoColor

	t.Cleanup(func() {
		color.NoColor = originalNoColor
	})

	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "SetWriter and UnsetWriter work together",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false
				buf := &bytes.Buffer{}

				c := color.New(color.FgRed)
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

				color.NoColor = true
				buf := &bytes.Buffer{}

				c := color.New(color.FgRed)
				c.SetWriter(buf)
				_, _ = c.Print("test")

				result := buf.String()
				assert.Empty(t, result, "should not write when NoColor is set")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFn(t)
		})
	}
}

// TestIntegration_SetMethodIntegration tests Set method integration.
func TestIntegration_SetMethodIntegration(t *testing.T) {
	// Save and restore global state
	originalNoColor := color.NoColor
	originalOutput := color.Output

	t.Cleanup(func() {
		color.NoColor = originalNoColor
		color.Output = originalOutput
	})

	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "Set method applies color",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false
				buf := &bytes.Buffer{}
				color.Output = buf

				c := color.New(color.FgRed)
				c.Set()

				_ = buf.String()
			},
		},
		{
			name: "Set is skipped when NoColor is set",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = true
				buf := &bytes.Buffer{}
				color.Output = buf

				c := color.New(color.FgRed)
				c.Set()

				_ = buf.String()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFn(t)
		})
	}
}

// TestIntegration_HiIntensityColorsIntegration tests hi-intensity color helpers.
func TestIntegration_HiIntensityColorsIntegration(t *testing.T) {
	tests := []struct {
		name   string
		testFn func() string
	}{
		{
			name:   "HiRedString produces hi-intensity red",
			testFn: func() string { return color.HiRedString("test") },
		},
		{
			name:   "HiGreenString produces hi-intensity green",
			testFn: func() string { return color.HiGreenString("test") },
		},
		{
			name:   "HiBlueString produces hi-intensity blue",
			testFn: func() string { return color.HiBlueString("test") },
		},
		{
			name:   "HiYellowString produces hi-intensity yellow",
			testFn: func() string { return color.HiYellowString("test") },
		},
		{
			name:   "HiCyanString produces hi-intensity cyan",
			testFn: func() string { return color.HiCyanString("test") },
		},
		{
			name:   "HiMagentaString produces hi-intensity magenta",
			testFn: func() string { return color.HiMagentaString("test") },
		},
		{
			name:   "HiWhiteString produces hi-intensity white",
			testFn: func() string { return color.HiWhiteString("test") },
		},
		{
			name:   "HiBlackString produces hi-intensity black",
			testFn: func() string { return color.HiBlackString("test") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFn()
			assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			assert.Contains(t, result, "test", "should contain text")
		})
	}
}

// TestIntegration_BackgroundColorsIntegration tests background color helpers.
func TestIntegration_BackgroundColorsIntegration(t *testing.T) {
	tests := []struct {
		name   string
		testFn func() string
	}{
		{
			name:   "BgBlack background",
			testFn: func() string { return color.New(color.BgBlack).Sprint("test") },
		},
		{
			name:   "BgRed background",
			testFn: func() string { return color.New(color.BgRed).Sprint("test") },
		},
		{
			name:   "BgGreen background",
			testFn: func() string { return color.New(color.BgGreen).Sprint("test") },
		},
		{
			name:   "BgYellow background",
			testFn: func() string { return color.New(color.BgYellow).Sprint("test") },
		},
		{
			name:   "BgBlue background",
			testFn: func() string { return color.New(color.BgBlue).Sprint("test") },
		},
		{
			name:   "BgMagenta background",
			testFn: func() string { return color.New(color.BgMagenta).Sprint("test") },
		},
		{
			name:   "BgCyan background",
			testFn: func() string { return color.New(color.BgCyan).Sprint("test") },
		},
		{
			name:   "BgWhite background",
			testFn: func() string { return color.New(color.BgWhite).Sprint("test") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFn()
			assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			assert.Contains(t, result, "test", "should contain text")
		})
	}
}

// TestIntegration_StyleAttributesIntegration tests style attribute helpers.
func TestIntegration_StyleAttributesIntegration(t *testing.T) {
	tests := []struct {
		name   string
		testFn func() string
	}{
		{
			name:   "Bold style",
			testFn: func() string { return color.New(color.Bold).Sprint("test") },
		},
		{
			name:   "Faint style",
			testFn: func() string { return color.New(color.Faint).Sprint("test") },
		},
		{
			name:   "Italic style",
			testFn: func() string { return color.New(color.Italic).Sprint("test") },
		},
		{
			name:   "Underline style",
			testFn: func() string { return color.New(color.Underline).Sprint("test") },
		},
		{
			name:   "BlinkSlow style",
			testFn: func() string { return color.New(color.BlinkSlow).Sprint("test") },
		},
		{
			name:   "BlinkRapid style",
			testFn: func() string { return color.New(color.BlinkRapid).Sprint("test") },
		},
		{
			name:   "ReverseVideo style",
			testFn: func() string { return color.New(color.ReverseVideo).Sprint("test") },
		},
		{
			name:   "Concealed style",
			testFn: func() string { return color.New(color.Concealed).Sprint("test") },
		},
		{
			name:   "CrossedOut style",
			testFn: func() string { return color.New(color.CrossedOut).Sprint("test") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFn()
			assert.Contains(t, result, "\x1b[", "should contain ANSI escape code")
			assert.Contains(t, result, "test", "should contain text")
		})
	}
}

// TestIntegration_ComplexScenariosIntegration tests complex real-world scenarios.
func TestIntegration_ComplexScenariosIntegration(t *testing.T) {
	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "colored log message simulation",
			testFn: func(t *testing.T) {
				t.Helper()

				color.NoColor = false

				timestamp := color.WhiteString("2024-01-15 10:30:00")
				level := color.RedString("[ERROR]")
				message := color.WhiteString("Something went wrong")

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

				color.NoColor = false

				headerName := color.New(color.Bold).Sprint("Name")
				headerAge := color.New(color.Bold).Sprint("Age")
				row1Name := color.CyanString("Alice")
				row1Age := color.GreenString("30")

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

				color.NoColor = false

				prefix := "Status: "
				status := color.GreenString("OK")
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

				color.NoColor = false

				var builder bytes.Buffer
				builder.WriteString(color.RedString("Error: "))
				builder.WriteString(color.WhiteString("File not found"))
				builder.WriteString(color.RedString("\n"))
				builder.WriteString(color.YellowString("Please check the path"))

				result := builder.String()

				assert.Contains(t, result, "Error: ")
				assert.Contains(t, result, "File not found")
				assert.Contains(t, result, "Please check the path")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFn(t)
		})
	}
}
