package color_test

import (
	"fmt"
	"os"

	"github.com/nicholas-fedor/shoutrrr/pkg/color"
)

// Example demonstrates basic color printing using helper functions.
// The helper functions use the default configuration internally.
func Example() {
	// Disable color via environment variable for predictable test output
	os.Setenv("NO_COLOR", "true")

	defer os.Unsetenv("NO_COLOR")

	fmt.Println(color.RedString("This is red text"))
	fmt.Println(color.GreenString("This is green text"))
	fmt.Println(color.BlueString("This is blue text"))
	fmt.Println(color.YellowString("This is yellow text"))

	// Output:
	// This is red text
	// This is green text
	// This is blue text
	// This is yellow text
}

// ExampleRGB demonstrates using RGB colors for foreground and background.
// Uses the default configuration via helper functions.
func ExampleRGB() {
	// Disable color via environment variable for predictable test output
	os.Setenv("NO_COLOR", "true")

	defer os.Unsetenv("NO_COLOR")

	orange := color.RGB(255, 128, 0)
	fmt.Println(orange.Sprint("Orange foreground text"))

	blueBg := color.BgRGB(0, 0, 255)
	fmt.Println(blueBg.Sprint("Text with blue background"))

	custom := color.RGB(128, 64, 255).AddBgRGB(255, 255, 0)
	fmt.Println(custom.Sprint("Purple text on yellow background"))

	// Output:
	// Orange foreground text
	// Text with blue background
	// Purple text on yellow background
}

// ExampleNew demonstrates creating and mixing custom colors.
// Uses the default configuration via the New() constructor.
func ExampleNew() {
	// Disable color via environment variable for predictable test output
	os.Setenv("NO_COLOR", "true")

	defer os.Unsetenv("NO_COLOR")

	// Create a bold red color
	boldRed := color.New(color.FgRed, color.Bold)
	fmt.Println(boldRed.Sprint("Bold red text"))

	// Mix colors by adding attributes
	underlinedGreen := color.New(color.FgGreen).Add(color.Underline)
	fmt.Println(underlinedGreen.Sprint("Underlined green text"))

	// Combine multiple attributes
	fancy := color.New(color.FgCyan, color.BgBlack, color.Bold, color.Underline)
	fmt.Println(fancy.Sprint("Fancy cyan text"))

	// Output:
	// Bold red text
	// Underlined green text
	// Fancy cyan text
}

// ExampleColor_PrintFunc demonstrates using custom print functions.
// Uses the default configuration via the New() constructor.
func ExampleColor_PrintFunc() {
	// Disable color via environment variable for predictable test output
	os.Setenv("NO_COLOR", "true")

	defer os.Unsetenv("NO_COLOR")

	// Create custom print functions
	warn := color.New(color.FgYellow).PrintlnFunc()
	err := color.New(color.FgRed, color.Bold).PrintfFunc()

	warn("This is a warning")
	err("Error code: %d\n", 404)

	// Custom string function
	info := color.New(color.FgBlue).SprintFunc()
	message := "Info: " + info("system ready")
	fmt.Println(message)

	// Output:
	// This is a warning
	// Error code: 404
	// Info: system ready
}

// ExampleColor_SprintFunc demonstrates string functions for mixing with non-colored text.
// Uses the default configuration via the New() constructor.
func ExampleColor_SprintFunc() {
	// Disable color via environment variable for predictable test output
	os.Setenv("NO_COLOR", "true")

	defer os.Unsetenv("NO_COLOR")

	// Create color string functions
	red := color.New(color.FgRed).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	blue := color.New(color.FgBlue).SprintFunc()

	// Mix colored strings with regular text
	status := fmt.Sprintf("Status: %s | %s | %s",
		red("ERROR"),
		green("SUCCESS"),
		blue("INFO"))

	fmt.Println(status)

	// Using SprintfFunc for formatted colored strings
	coloredPrintf := color.New(color.FgMagenta).SprintfFunc()
	details := "Details: " + coloredPrintf("Value is %d", 42)
	fmt.Println(details)

	// Output:
	// Status: ERROR | SUCCESS | INFO
	// Details: Value is 42
}

// ExampleNewWithConfig demonstrates creating a Color with a custom Config
// for thread-safe, instance-based color management.
func ExampleNewWithConfig() {
	// Create a custom configuration with color disabled for predictable output
	cfg := &color.Config{
		NoColor: true,
		Output:  os.Stdout,
	}

	// Create color instances with the custom config
	red := color.NewWithConfig(cfg, color.FgRed)
	green := color.NewWithConfig(cfg, color.FgGreen)
	blue := color.NewWithConfig(cfg, color.FgBlue)

	fmt.Println(red.Sprint("Red text (disabled)"))
	fmt.Println(green.Sprint("Green text (disabled)"))
	fmt.Println(blue.Sprint("Blue text (disabled)"))

	// Output:
	// Red text (disabled)
	// Green text (disabled)
	// Blue text (disabled)
}

// ExampleConfig demonstrates using the Config struct for thread-safe
// color configuration that can be passed to color constructors.
func ExampleConfig() {
	// Create a custom configuration
	cfg := &color.Config{
		NoColor: false,
		Output:  os.Stdout,
	}

	// The configuration provides safe defaults
	fmt.Fprintf(cfg.Output, "Output is stdout: %v\n", cfg.Output == os.Stdout)
	fmt.Fprintf(cfg.Output, "NoColor value: %v\n", cfg.NoColor)

	// Output:
	// Output is stdout: true
	// NoColor value: false
}

// ExampleColor_setUnset demonstrates using Set and Unset methods on a Color instance
// for temporary color changes in a thread-safe manner.
func ExampleColor_setUnset() {
	// Create a configuration with color enabled but output to stdout
	cfg := &color.Config{
		NoColor: true, // Disabled for predictable test output
		Output:  os.Stdout,
	}

	// Create a color instance with the config
	c := color.NewWithConfig(cfg, color.FgRed)

	fmt.Print("Normal text, ")
	c.Set()
	fmt.Fprint(cfg.Output, "red text")
	c.Unset()
	fmt.Println(", back to normal")

	// Output:
	// Normal text, red text, back to normal
}

// ExampleColor_DisableColor demonstrates how to disable colors on individual
// Color instances using the thread-safe DisableColor method.
func ExampleColor_DisableColor() {
	// Disable color globally via environment variable
	os.Setenv("NO_COLOR", "true")

	defer os.Unsetenv("NO_COLOR")

	// Create color with default config (which respects NO_COLOR)
	red := color.New(color.FgRed)
	fmt.Println(red.Sprint("This should not be red"))

	// Create a color with explicit config
	cfg := &color.Config{
		NoColor: true,
		Output:  os.Stdout,
	}
	yellow := color.NewWithConfig(cfg, color.FgYellow)
	fmt.Println(yellow.Sprint("This yellow text is disabled"))

	// Output:
	// This should not be red
	// This yellow text is disabled
}
