package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestColorizeValue verifies that a value is colored as an enum when it is
// one, otherwise as a true or false bool, a number, or a string, and that the
// text itself is kept. The colorizers fix their color setting at init, so when
// stdout is not a terminal the comparison checks the text only.
func TestColorizeValue(t *testing.T) {
	t.Parallel()

	// tests lists values with how each should be colored.
	tests := []struct {
		colorize func(...any) string
		value    string
		isEnum   bool
	}{
		{value: "Foo", isEnum: true, colorize: ColorizeEnum},
		{value: "yes", isEnum: true, colorize: ColorizeEnum},
		{value: "yes", colorize: ColorizeTrue},
		{value: "No", colorize: ColorizeFalse},
		{value: "1.5", colorize: ColorizeNumber},
		{value: "text", colorize: ColorizeString},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()

			colored := ColorizeValue(tt.value, tt.isEnum)

			assert.Equal(t, tt.colorize(tt.value), colored)
			assert.Equal(t, tt.value, ansiEscape.ReplaceAllString(colored, ""))
		})
	}
}

// TestColorizeToken verifies that each token type is colored with its color,
// and that an unknown token is left as it is. As in [TestColorizeValue], the
// comparison checks the text only when stdout is not a terminal.
func TestColorizeToken(t *testing.T) {
	t.Parallel()

	// tests lists token types with how each should be colored.
	tests := []struct {
		colorize func(...any) string
		name     string
		token    NodeTokenType
	}{
		{name: "number", token: NumberToken, colorize: ColorizeNumber},
		{name: "enum", token: EnumToken, colorize: ColorizeEnum},
		{name: "true", token: TrueToken, colorize: ColorizeTrue},
		{name: "false", token: FalseToken, colorize: ColorizeFalse},
		{name: "prop", token: PropToken, colorize: ColorizeProp},
		{name: "error", token: ErrorToken, colorize: ColorizeError},
		{name: "container", token: ContainerToken, colorize: ColorizeContainer},
		{name: "string", token: StringToken, colorize: ColorizeString},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.colorize("value"), ColorizeToken("value", tt.token))
		})
	}

	assert.Equal(t, "value", ColorizeToken("value", UnknownToken))
	assert.Equal(t, "value", ColorizeToken("value", NodeTokenType(99)))
}
