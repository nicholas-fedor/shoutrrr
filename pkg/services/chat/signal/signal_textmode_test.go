package signal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTextMode verifies each text mode's config name and its text_mode
// value, which is empty for None so the field is omitted.
func TestTextMode(t *testing.T) {
	t.Parallel()

	// tests lists text modes with their names and payload values.
	tests := []struct {
		name        string
		wantPayload string
		mode        textMode
	}{
		{mode: TextModeNone, name: "None"},
		{mode: TextModeNormal, name: "Normal", wantPayload: "normal"},
		{mode: TextModeStyled, name: "Styled", wantPayload: "styled"},
		{mode: textMode(9), name: "Invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.name, tt.mode.String())
			assert.Equal(t, tt.wantPayload, tt.mode.payloadValue())
		})
	}
}

// TestTextModesEnum verifies that the enum parses the mode names regardless
// of case.
func TestTextModesEnum(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"None", "Normal", "Styled"}, TextModes.Enum.Names())
	assert.Equal(t, int(TextModeStyled), TextModes.Enum.Parse("styled"))
	assert.Equal(t, int(TextModeNormal), TextModes.Enum.Parse("NORMAL"))
}
