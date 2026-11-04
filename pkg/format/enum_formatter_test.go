package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// otherEnumFormatter wraps a formatter as an implementation of
// [types.EnumFormatter] other than the one this package creates.
type otherEnumFormatter struct {
	types.EnumFormatter
}

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

// TestEnumFormatterParse verifies that names and aliases match in any case,
// that aliases map to their index, and that anything else is invalid.
func TestEnumFormatterParse(t *testing.T) {
	t.Parallel()

	formatter := CreateEnumFormatter([]string{"None", "Foo", "Bar"}, map[string]int{"f": 1, "2": 2, "Bar_Alias": 2})

	// tests lists raw values with the index each should parse to.
	tests := []struct {
		raw  string
		want int
	}{
		{raw: "None", want: 0},
		{raw: "foo", want: 1},
		{raw: "BAR", want: 2},
		{raw: "f", want: 1},
		{raw: "F", want: 1},
		{raw: "2", want: 2},
		{raw: "bar_alias", want: 2},
		{raw: "BAR_ALIAS", want: 2},
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

// TestEnumFormatterParseAliasCollision verifies that aliases differing only
// in case each match their own spelling exactly, and that any other spelling
// resolves to the same one every time, the first in sorted order.
func TestEnumFormatterParseAliasCollision(t *testing.T) {
	t.Parallel()

	for range 50 {
		formatter := CreateEnumFormatter([]string{"Closed", "Open", "Pending"}, map[string]int{"op": 1, "OP": 2})

		require.Equal(t, 1, formatter.Parse("op"))
		require.Equal(t, 2, formatter.Parse("OP"))
		require.Equal(t, 2, formatter.Parse("Op"))
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

// TestCheckEnumValue verifies that a value passes when a URL can hold it and
// fails with ErrInvalidEnumValue otherwise.
//
// A URL holds a value as its name, so a value with a name passes. Value 0 also
// passes for an enum whose names start empty, since an empty value reads back
// as 0. A negative value fails, and so does one past the last name or one whose
// name is empty. So does a value past the last name of an enum with a name of
// Invalid, which Print returns for it, and a name that an earlier name matches
// in any case. Each case runs with the formatter this package creates and with
// another implementation of the formatter interface, which is judged by parsing
// the names it prints.
func TestCheckEnumValue(t *testing.T) {
	t.Parallel()

	// leadingEmpty has no name for 0 or 1, as an enum whose values start at 2.
	leadingEmpty := CreateEnumFormatter([]string{"", "", "Low", "High"})
	// invalidNamed has a name of Invalid, which Print also returns past the
	// last value. That value prints a listed name but reads back as 0.
	invalidNamed := CreateEnumFormatter([]string{"Invalid", "Valid"})
	// caseTwins has two names that match in any case, so the second reads
	// back as the first.
	caseTwins := CreateEnumFormatter([]string{"Low", "LOW", "High"})

	// tests lists formatters and values with whether each should pass.
	tests := []struct {
		formatter types.EnumFormatter
		name      string
		value     int
		valid     bool
	}{
		{name: "first name", formatter: testEnum, value: 0, valid: true},
		{name: "last name", formatter: testEnum, value: 2, valid: true},
		{name: "past the last name", formatter: testEnum, value: 3},
		{name: "negative", formatter: testEnum, value: EnumInvalid},
		{name: "zero without a name", formatter: leadingEmpty, value: 0, valid: true},
		{name: "empty name past zero", formatter: leadingEmpty, value: 1},
		{name: "name after empty names", formatter: leadingEmpty, value: 3, valid: true},
		{name: "name of Invalid", formatter: invalidNamed, value: 0, valid: true},
		{name: "past a name of Invalid", formatter: invalidNamed, value: 2},
		{name: "first of names matching in any case", formatter: caseTwins, value: 0, valid: true},
		{name: "later of names matching in any case", formatter: caseTwins, value: 1},
		{name: "name after names matching in any case", formatter: caseTwins, value: 2, valid: true},
	}

	for _, tt := range tests {
		// formatters holds the formatter itself and the same enum behind
		// another implementation of the interface.
		formatters := map[string]types.EnumFormatter{
			"created": tt.formatter,
			"other":   otherEnumFormatter{EnumFormatter: tt.formatter},
		}

		for kind, formatter := range formatters {
			t.Run(tt.name+"/"+kind, func(t *testing.T) {
				t.Parallel()

				err := CheckEnumValue(formatter, tt.value)
				if tt.valid {
					require.NoError(t, err)

					return
				}

				require.ErrorIs(t, err, ErrInvalidEnumValue)
				assert.Contains(t, err.Error(), "accepted values are")
			})
		}
	}
}
