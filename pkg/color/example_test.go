package color_test

import (
	"fmt"

	"github.com/nicholas-fedor/shoutrrr/pkg/color"
)

// Example demonstrates basic color printing using helper functions.
func Example() {
	color.Redf("This is red text")
	color.Greenf("This is green text")
	color.Bluef("This is blue text")
	color.Yellowf("This is yellow text")

	// Output:
	// This is red text
	// This is green text
	// This is blue text
	// This is yellow text
}

// ExampleRGB demonstrates using RGB colors for foreground and background.
func ExampleRGB() {
	orange := color.RGB(255, 128, 0)
	orange.Println("Orange foreground text")

	blueBg := color.BgRGB(0, 0, 255)
	blueBg.Println("Text with blue background")

	custom := color.RGB(128, 64, 255).AddBgRGB(255, 255, 0)
	custom.Println("Purple text on yellow background")

	// Output:
	// Orange foreground text
	// Text with blue background
	// Purple text on yellow background
}

// ExampleNew demonstrates creating and mixing custom colors.
func ExampleNew() {
	// Create a bold red color
	boldRed := color.New(color.FgRed, color.Bold)
	boldRed.Println("Bold red text")

	// Mix colors by adding attributes
	underlinedGreen := color.New(color.FgGreen).Add(color.Underline)
	underlinedGreen.Println("Underlined green text")

	// Combine multiple attributes
	fancy := color.New(color.FgCyan, color.BgBlack, color.Bold, color.Underline)
	fancy.Println("Fancy cyan text")

	// Output:
	// Bold red text
	// Underlined green text
	// Fancy cyan text
}

// ExampleColor_PrintFunc demonstrates using custom print functions.
func ExampleColor_PrintFunc() {
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
func ExampleColor_SprintFunc() {
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

// ExampleSet demonstrates global color control using Set and Unset.
func ExampleSet() {
	fmt.Print("Normal text, ")
	color.Set(color.FgRed)
	fmt.Print("red text")
	color.Unset()
	fmt.Println(", back to normal")

	// Multiple sets
	color.Set(color.FgGreen)
	fmt.Print("Green, ")
	color.Set(color.FgBlue)
	fmt.Print("then blue")
	color.Unset()
	fmt.Println(", normal again")

	// Output:
	// Normal text, red text, back to normal
	// Green, then blue, normal again
}

// ExampleColor_DisableColor demonstrates how to disable colors.
func ExampleColor_DisableColor() {
	// Disable colors globally
	color.NoColor = true

	color.Redf("This should not be red")
	color.Greenf("This should not be green")

	// Re-enable
	color.NoColor = false

	// Disable on individual color objects
	c := color.New(color.FgYellow)
	c.DisableColor()
	c.Println("This yellow text is disabled")

	// Re-enable individual color
	c.EnableColor()
	c.Println("Now it's yellow again")

	// Output:
	// This should not be red
	// This should not be green
	// This yellow text is disabled
	// Now it's yellow again
}
