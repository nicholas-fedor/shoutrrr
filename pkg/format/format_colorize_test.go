package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestColorizers verifies the color of each colorizer, and that every
// colorizer writes plain text when color is turned off in their shared
// config.
//
//nolint:paralleltest // Changes the color setting every colorizer reads.
func TestColorizers(t *testing.T) {
	// tests lists each colorizer with the text it should give for "value"
	// when color is on.
	tests := []struct {
		colorize func(...any) string
		name     string
		want     string
	}{
		{name: "desc", colorize: ColorizeDesc, want: "\x1b[90mvalue\x1b[0m"},
		{name: "true", colorize: ColorizeTrue, want: "\x1b[92mvalue\x1b[0m"},
		{name: "false", colorize: ColorizeFalse, want: "\x1b[91mvalue\x1b[0m"},
		{name: "number", colorize: ColorizeNumber, want: "\x1b[94mvalue\x1b[0m"},
		{name: "string", colorize: ColorizeString, want: "\x1b[93mvalue\x1b[0m"},
		{name: "enum", colorize: ColorizeEnum, want: "\x1b[96mvalue\x1b[0m"},
		{name: "prop", colorize: ColorizeProp, want: "\x1b[95mvalue\x1b[0m"},
		{name: "error", colorize: ColorizeError, want: "\x1b[91mvalue\x1b[0m"},
		{name: "container", colorize: ColorizeContainer, want: "\x1b[90mvalue\x1b[0m"},
		{name: "link", colorize: ColorizeLink, want: "\x1b[94mvalue\x1b[0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setNoColor(t, false)
			assert.Equal(t, tt.want, tt.colorize("value"))

			setNoColor(t, true)
			assert.Equal(t, "value", tt.colorize("value"))
		})
	}
}

// TestColorizeValue verifies that a value is colored as an enum when it is
// one, otherwise as a true or false bool, a number, or a string.
//
//nolint:paralleltest // Changes the color setting every colorizer reads.
func TestColorizeValue(t *testing.T) {
	setNoColor(t, false)

	// tests lists values with the colored text each should give.
	tests := []struct {
		value  string
		want   string
		isEnum bool
	}{
		{value: "Foo", isEnum: true, want: "\x1b[96mFoo\x1b[0m"},
		{value: "yes", isEnum: true, want: "\x1b[96myes\x1b[0m"},
		{value: "yes", want: "\x1b[92myes\x1b[0m"},
		{value: "No", want: "\x1b[91mNo\x1b[0m"},
		{value: "1.5", want: "\x1b[94m1.5\x1b[0m"},
		{value: "text", want: "\x1b[93mtext\x1b[0m"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, tt.want, ColorizeValue(tt.value, tt.isEnum))
		})
	}
}

// TestColorizeToken verifies that each token type is colored with its color,
// and that an unknown token is left as it is.
//
//nolint:paralleltest // Changes the color setting every colorizer reads.
func TestColorizeToken(t *testing.T) {
	setNoColor(t, false)

	// tests lists token types with the colored text each should give for
	// "value".
	tests := []struct {
		name  string
		want  string
		token NodeTokenType
	}{
		{name: "number", token: NumberToken, want: "\x1b[94mvalue\x1b[0m"},
		{name: "enum", token: EnumToken, want: "\x1b[96mvalue\x1b[0m"},
		{name: "true", token: TrueToken, want: "\x1b[92mvalue\x1b[0m"},
		{name: "false", token: FalseToken, want: "\x1b[91mvalue\x1b[0m"},
		{name: "prop", token: PropToken, want: "\x1b[95mvalue\x1b[0m"},
		{name: "error", token: ErrorToken, want: "\x1b[91mvalue\x1b[0m"},
		{name: "container", token: ContainerToken, want: "\x1b[90mvalue\x1b[0m"},
		{name: "string", token: StringToken, want: "\x1b[93mvalue\x1b[0m"},
		{name: "unknown", token: UnknownToken, want: "value"},
		{name: "undefined", token: NodeTokenType(99), want: "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ColorizeToken("value", tt.token))
		})
	}
}

// setNoColor turns the colorizers' color off or on for the rest of the test
// and restores the setting when the test ends. A test that calls it must not
// run in parallel, because every colorizer reads the setting.
//
// Parameters:
//   - t: the test.
//   - noColor: true to write plain text, false to color.
func setNoColor(t *testing.T, noColor bool) {
	t.Helper()

	previous := colorConfig.NoColor
	colorConfig.NoColor = noColor

	t.Cleanup(func() { colorConfig.NoColor = previous })
}
