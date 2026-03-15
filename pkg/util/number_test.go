package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripNumberPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantNum  string
		wantBase int
	}{
		{
			name:     "strips hash prefix and returns base 16",
			input:    "#FF",
			wantNum:  "FF",
			wantBase: 16,
		},
		{
			name:     "strips lowercase 0x prefix and returns base 16",
			input:    "0xABCD",
			wantNum:  "ABCD",
			wantBase: 16,
		},
		{
			name:     "strips uppercase 0X prefix and returns base 16",
			input:    "0X1234",
			wantNum:  "1234",
			wantBase: 16,
		},
		{
			name:     "returns input unchanged and base 0 when no prefix",
			input:    "12345",
			wantNum:  "12345",
			wantBase: 0,
		},
		{
			name:     "handles empty string",
			input:    "",
			wantNum:  "",
			wantBase: 0,
		},
		{
			name:     "handles single hash character",
			input:    "#",
			wantNum:  "",
			wantBase: 16,
		},
		{
			name:     "handles single 0x prefix",
			input:    "0x",
			wantNum:  "",
			wantBase: 16,
		},
		{
			name:     "handles only 0 without x",
			input:    "0",
			wantNum:  "0",
			wantBase: 0,
		},
		{
			name:     "handles decimal number starting with 0",
			input:    "0123",
			wantNum:  "0123",
			wantBase: 0,
		},
		{
			name:     "handles lowercase hex with 0x",
			input:    "0xdeadbeef",
			wantNum:  "deadbeef",
			wantBase: 16,
		},
		{
			name:     "handles uppercase hex with hash",
			input:    "#DEADBEEF",
			wantNum:  "DEADBEEF",
			wantBase: 16,
		},
		{
			name:     "handles hex with mixed case",
			input:    "0xDeAdBeEf",
			wantNum:  "DeAdBeEf",
			wantBase: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotNum, gotBase := StripNumberPrefix(tt.input)
			assert.Equal(t, tt.wantNum, gotNum, "StripNumberPrefix(%q) returned number", tt.input)
			assert.Equal(t, tt.wantBase, gotBase, "StripNumberPrefix(%q) returned base", tt.input)
		})
	}
}
