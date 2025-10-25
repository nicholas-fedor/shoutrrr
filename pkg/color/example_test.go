package color_test

import (
	"fmt"
	"os"

	"github.com/nicholas-fedor/shoutrrr/pkg/color"
)

// Example demonstrates basic color printing using helper functions.
func Example() {
	originalNoColor := color.NoColor
	color.NoColor = true

	defer func() { color.NoColor = originalNoColor }()

	originalOutput := color.Output
	color.Output = os.Stdout
	fmt.Println(color.RedString("This is red text"))
	fmt.Println(color.GreenString("This is green text"))
	fmt.Println(color.BlueString("This is blue text"))
	fmt.Println(color.YellowString("This is yellow text"))
	color.Output = originalOutput

	// Output:
	// This is red text
	// This is green text
	// This is blue text
	// This is yellow text
}

// ExampleRGB demonstrates using RGB colors for foreground and background.
func ExampleRGB() {
	originalNoColor := color.NoColor
	color.NoColor = true

	defer func() { color.NoColor = originalNoColor }()

	originalOutput := color.Output
	color.Output = os.Stdout
	orange := color.RGB(255, 128, 0)
	fmt.Println(orange.Sprint("Orange foreground text"))

	blueBg := color.BgRGB(0, 0, 255)
	fmt.Println(blueBg.Sprint("Text with blue background"))

	custom := color.RGB(128, 64, 255).AddBgRGB(255, 255, 0)
	fmt.Println(custom.Sprint("Purple text on yellow background"))

	color.Output = originalOutput

	// Output:
	// Orange foreground text
	// Text with blue background
	// Purple text on yellow background
}

// ExampleNew demonstrates creating and mixing custom colors.
func ExampleNew() {
	originalNoColor := color.NoColor
	color.NoColor = true

	defer func() { color.NoColor = originalNoColor }()

	originalOutput := color.Output
	color.Output = os.Stdout

	// Create a bold red color
	boldRed := color.New(color.FgRed, color.Bold)
	fmt.Println(boldRed.Sprint("Bold red text"))

	// Mix colors by adding attributes
	underlinedGreen := color.New(color.FgGreen).Add(color.Underline)
	fmt.Println(underlinedGreen.Sprint("Underlined green text"))

	// Combine multiple attributes
	fancy := color.New(color.FgCyan, color.BgBlack, color.Bold, color.Underline)
	fmt.Println(fancy.Sprint("Fancy cyan text"))

	color.Output = originalOutput

	// Output:
	// Bold red text
	// Underlined green text
	// Fancy cyan text
}

// ExampleColor_PrintFunc demonstrates using custom print functions.
func ExampleColor_PrintFunc() {
	originalNoColor := color.NoColor
	color.NoColor = true

	defer func() { color.NoColor = originalNoColor }()

	originalOutput := color.Output
	color.Output = os.Stdout

	// Create custom print functions
	warn := color.New(color.FgYellow).PrintlnFunc()
	err := color.New(color.FgRed, color.Bold).PrintfFunc()

	warn("This is a warning")
	err("Error code: %d\n", 404)

	// Custom string function
	info := color.New(color.FgBlue).SprintFunc()
	message := "Info: " + info("system ready")
	fmt.Println(message)

	color.Output = originalOutput

	// Output:
	// This is a warning
	// Error code: 404
	// Info: system ready
}

// ExampleColor_SprintFunc demonstrates string functions for mixing with non-colored text.
func ExampleColor_SprintFunc() {
	originalNoColor := color.NoColor
	color.NoColor = true

	defer func() { color.NoColor = originalNoColor }()

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
	originalOutput := color.Output
	color.Output = os.Stdout

	originalNoColor := color.NoColor
	color.NoColor = true

	defer func() { color.NoColor = originalNoColor }()

	color.Redf("This should not be red")
	color.Greenf("This should not be green")

	// Disable on individual color objects
	c := color.New(color.FgYellow)
	c.DisableColor()
	_, _ = c.Println("This yellow text is disabled")

	_, _ = c.Println("Now it's yellow again")

	color.Output = originalOutput

	// Output:
	// This should not be red
	// This should not be green
	// This yellow text is disabled
	// Now it's yellow again
}
