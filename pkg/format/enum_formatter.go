package format

import (
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
	firstOffset   int
}

// EnumInvalid is the constant value that an enum gets assigned when it could not be parsed.
const EnumInvalid = -1

// CreateEnumFormatter creates a EnumFormatter struct.
//
// Aliases match in any case, as names do. An exact alias match wins, and
// aliases that differ only in case otherwise resolve to the first of them in
// sorted order.
//
// Parameters:
//   - names: the enum names by value, with leading empty names left out of the listed names.
//   - optAliases: an optional map of other spellings to values.
//
// Returns:
//   - types.EnumFormatter: the formatter.
func CreateEnumFormatter(names []string, optAliases ...map[string]int) types.EnumFormatter {
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

	return &EnumFormatter{
		names:         names,
		firstOffset:   firstOffset,
		aliases:       aliases,
		foldedAliases: foldedAliases,
	}
}

// Names is the list of the valid Enum string values.
func (ef EnumFormatter) Names() []string {
	return ef.names[ef.firstOffset:]
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
