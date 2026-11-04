package format

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// EnumFormatter is the helper methods for enum-like types.
type EnumFormatter struct {
	aliases map[string]int
	// foldedAliases maps each lowercased alias to its value.
	foldedAliases map[string]int
	names         []string
	// defined holds, by value, whether the value's name parses back to it, or
	// the value is 0 with an empty name.
	defined     []bool
	firstOffset int
}

// EnumInvalid is the constant value that an enum gets assigned when it could not be parsed.
const EnumInvalid = -1

// CreateEnumFormatter creates a EnumFormatter struct.
//
// Aliases match in any case, as names do. An exact alias match wins, and
// aliases that differ only in case otherwise resolve to the first of them in
// sorted order. The formatter keeps its own copy of the names, so changing the
// given slice later leaves it unchanged.
//
// Parameters:
//   - names: the enum names by value, with leading empty names left out of the listed names.
//   - optAliases: an optional map of other spellings to values.
//
// Returns:
//   - types.EnumFormatter: the formatter.
func CreateEnumFormatter(names []string, optAliases ...map[string]int) types.EnumFormatter {
	// The names are copied, so the caller cannot change them out from under
	// the parse results recorded below.
	names = slices.Clone(names)
	aliases := map[string]int{}
	foldedAliases := map[string]int{}

	if len(optAliases) > 0 {
		for _, alias := range slices.Sorted(maps.Keys(optAliases[0])) {
			value := optAliases[0][alias]
			aliases[alias] = value

			if _, taken := foldedAliases[strings.ToLower(alias)]; !taken {
				foldedAliases[strings.ToLower(alias)] = value
			}
		}
	}

	firstOffset := 0

	for i, name := range names {
		if name != "" {
			firstOffset = i

			break
		}
	}

	enum := &EnumFormatter{
		names:         names,
		defined:       make([]bool, len(names)),
		firstOffset:   firstOffset,
		aliases:       aliases,
		foldedAliases: foldedAliases,
	}

	// Each name is parsed once here, so a check of a value needs no parse.
	for value, name := range names {
		enum.defined[value] = name == "" && value == 0 || name != "" && enum.Parse(name) == value
	}

	return enum
}

// CheckEnumValue checks that a value belongs to an enum, so a service URL can
// hold it.
//
// A URL holds an enum value as its name, and reading the URL parses the name
// back. A value counts when its name parses back to the same value. That
// includes 0 with an empty name, since reading an empty value gives 0. Reading
// any other value's URL fails or gives another value, so a service checks the
// enum fields of a config built in code with it before a send.
//
// Parameters:
//   - formatter: the enum's formatter.
//   - value: the value to check.
//
// Returns:
//   - error: an error wrapping [ErrInvalidEnumValue] that lists the accepted
//     names, or nil for a value of the enum.
func CheckEnumValue(formatter types.EnumFormatter, value int) error {
	// The formatter this package creates answers by index, without the
	// allocation and scan of a parse. Another formatter parses its printed
	// name, as reading a URL does.
	if enum, ok := formatter.(*EnumFormatter); ok {
		if enum.defines(value) {
			return nil
		}
	} else if value != EnumInvalid && formatter.Parse(formatter.Print(value)) == value {
		return nil
	}

	return fmt.Errorf("value %d: %w", value, invalidEnumValue(formatter))
}

// invalidEnumValue returns the error for a value outside an enum, which lists
// the enum's names.
//
// Parameters:
//   - formatter: the enum's formatter.
//
// Returns:
//   - error: an error wrapping [ErrInvalidEnumValue].
func invalidEnumValue(formatter types.EnumFormatter) error {
	return fmt.Errorf("%w: accepted values are %v", ErrInvalidEnumValue, formatter.Names())
}

// Names is the list of the valid Enum string values.
//
// It returns a copy, so changing the list leaves the enum unchanged.
//
// Returns:
//   - []string: the names, in value order, without leading empty names.
func (ef EnumFormatter) Names() []string {
	return slices.Clone(ef.names[ef.firstOffset:])
}

// Parse takes an enum mapped string and returns it's int representation or EnumInvalid (-1).
//
// Names and aliases match in any case, with an exact alias match first.
//
// Parameters:
//   - mappedString: the name or alias to parse.
//
// Returns:
//   - int: the enum value, or [EnumInvalid] when nothing matches.
func (ef EnumFormatter) Parse(mappedString string) int {
	target := strings.ToLower(mappedString)
	for index, name := range ef.names {
		if strings.EqualFold(target, name) {
			return index
		}
	}

	if index, found := ef.aliases[mappedString]; found {
		return index
	}

	if index, found := ef.foldedAliases[target]; found {
		return index
	}

	return EnumInvalid
}

// Print takes a enum mapped int and returns it's string representation or "Invalid".
func (ef EnumFormatter) Print(e int) string {
	if e >= len(ef.names) || e < 0 {
		return "Invalid"
	}

	return ef.names[e]
}

// defines reports whether the enum has a value whose name parses back to it,
// as [CheckEnumValue] describes.
//
// Parameters:
//   - value: the value to check.
//
// Returns:
//   - bool: whether the enum has the value.
func (ef EnumFormatter) defines(value int) bool {
	return value >= 0 && value < len(ef.defined) && ef.defined[value]
}
