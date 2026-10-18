package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCreateEnumFormatter verifies that leading empty names are left out of
// the listed names but keep their indexes.
func TestCreateEnumFormatter(t *testing.T) {
	t.Parallel()

	formatter := CreateEnumFormatter([]string{"", "", "Low", "High"})

	assert.Equal(t, []string{"Low", "High"}, formatter.Names())
	assert.Equal(t, 2, formatter.Parse("Low"))
	assert.Equal(t, "High", formatter.Print(3))
	assert.Equal(t, []string{"None", "Foo", "Bar"}, testEnum.Names())
}

// TestEnumFormatterParse verifies that names match in any case, that aliases
// map to their index, and that anything else is invalid.
func TestEnumFormatterParse(t *testing.T) {
	t.Parallel()

	formatter := CreateEnumFormatter([]string{"None", "Foo", "Bar"}, map[string]int{"f": 1, "2": 2})

	// tests lists raw values with the index each should parse to.
	tests := []struct {
		raw  string
		want int
	}{
		{raw: "None", want: 0},
		{raw: "foo", want: 1},
		{raw: "BAR", want: 2},
		{raw: "f", want: 1},
		{raw: "2", want: 2},
		{raw: "Baz", want: EnumInvalid},
		{raw: "", want: EnumInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, formatter.Parse(tt.raw))
		})
	}
}

// TestEnumFormatterPrint verifies that an index prints its name and that an
// index out of range prints Invalid.
func TestEnumFormatterPrint(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "None", testEnum.Print(0))
	assert.Equal(t, "Bar", testEnum.Print(2))
	assert.Equal(t, "Invalid", testEnum.Print(3))
	assert.Equal(t, "Invalid", testEnum.Print(EnumInvalid))
}
