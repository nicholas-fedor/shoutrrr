package color

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_boolPtr(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		checkPtr func(*testing.T, *bool)
	}{
		{
			name:  "returns pointer to true",
			input: true,
			checkPtr: func(t *testing.T, p *bool) {
				t.Helper()
				require.NotNil(t, p)
				assert.True(t, *p)
			},
		},
		{
			name:  "returns pointer to false",
			input: false,
			checkPtr: func(t *testing.T, p *bool) {
				t.Helper()
				require.NotNil(t, p)
				assert.False(t, *p)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boolPtr(tt.input)
			tt.checkPtr(t, result)
		})
	}
}

func Test_noColorIsSet(t *testing.T) {
	// Save original environment
	originalEnv := os.Getenv("NO_COLOR")

	t.Cleanup(func() {
		if originalEnv == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", originalEnv)
		}
	})

	tests := []struct {
		name      string
		setEnv    bool   // whether to set the environment variable
		envValue  string // the value to set (only used if setEnv is true)
		expectSet bool
	}{
		{
			name:      "returns false when NO_COLOR is not set",
			setEnv:    false,
			envValue:  "",
			expectSet: false,
		},
		{
			name:      "returns true when NO_COLOR is set to empty string",
			setEnv:    true,
			envValue:  "",
			expectSet: true,
		},
		{
			name:      "returns true when NO_COLOR is set to 0",
			setEnv:    true,
			envValue:  "0",
			expectSet: true,
		},
		{
			name:      "returns true when NO_COLOR is set to false",
			setEnv:    true,
			envValue:  "false",
			expectSet: true,
		},
		{
			name:      "returns true when NO_COLOR is set to any value",
			setEnv:    true,
			envValue:  "1",
			expectSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				os.Setenv("NO_COLOR", tt.envValue)
			} else {
				os.Unsetenv("NO_COLOR")
			}

			result := noColorIsSet()
			assert.Equal(t, tt.expectSet, result)
		})
	}
}

func Test_sprintln(t *testing.T) {
	tests := []struct {
		name     string
		input    []any
		expected string
	}{
		{
			name:     "empty input returns empty string",
			input:    []any{},
			expected: "",
		},
		{
			name:     "single string removes trailing newline",
			input:    []any{"hello"},
			expected: "hello",
		},
		{
			name:     "single integer",
			input:    []any{42},
			expected: "42",
		},
		{
			name:     "multiple arguments",
			input:    []any{"hello", "world"},
			expected: "hello world",
		},
		{
			name:     "mixed types",
			input:    []any{"count:", 123, "pi:", 3.14},
			expected: "count: 123 pi: 3.14",
		},
		{
			name:     "string with spaces",
			input:    []any{"hello world"},
			expected: "hello world",
		},
		{
			name:     "empty string",
			input:    []any{""},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sprintln(tt.input...)
			assert.Equal(t, tt.expected, result)
		})
	}
}
