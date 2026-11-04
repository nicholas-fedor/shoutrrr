package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNonEmpty verifies that NonEmpty drops every empty string and keeps the
// other elements in order.
//
// A list of only empty strings becomes nil.
func TestNonEmpty(t *testing.T) {
	t.Parallel()

	// tests lists inputs with the list each should produce.
	tests := []struct {
		name   string
		values []string
		want   []string
	}{
		{name: "nil", values: nil, want: nil},
		{name: "no elements", values: []string{}, want: []string{}},
		{name: "no empty element", values: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "trailing empty element", values: []string{"a", ""}, want: []string{"a"}},
		{name: "leading empty element", values: []string{"", "a"}, want: []string{"a"}},
		{name: "empty elements between", values: []string{"a", "", "b", "", "", "c"}, want: []string{"a", "b", "c"}},
		{name: "blank element kept", values: []string{"a", " "}, want: []string{"a", " "}},
		{name: "only empty elements", values: []string{"", ""}, want: nil},
		{name: "one empty element", values: []string{""}, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, NonEmpty(tt.values))
		})
	}
}

// TestNonEmptyLeavesInput verifies that NonEmpty never writes to its input, so
// a config copy sharing a list with the stored config cannot change it.
//
// A list without an empty string comes back without a copy.
func TestNonEmptyLeavesInput(t *testing.T) {
	t.Parallel()

	values := []string{"a", "", "b", ""}
	got := NonEmpty(values)

	assert.Equal(t, []string{"a", "", "b", ""}, values)

	got[0] = "changed"

	assert.Equal(t, "a", values[0])

	unchanged := []string{"a", "b"}
	assert.Same(t, &unchanged[0], &NonEmpty(unchanged)[0])
}
