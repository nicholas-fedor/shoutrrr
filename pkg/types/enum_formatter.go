package types

// EnumFormatter translate enums between strings and numbers.
type EnumFormatter interface {
	// Print returns the name of an enum value.
	//
	// Parameters:
	//   - e: the enum value.
	//
	// Returns:
	//   - string: the name of e.
	Print(e int) string
	// Parse returns the enum value that a name stands for.
	//
	// Parameters:
	//   - s: the name to parse.
	//
	// Returns:
	//   - int: the enum value, or -1 when s matches no name.
	Parse(s string) int
	// Names returns the valid names of the enum.
	//
	// Returns:
	//   - []string: the names, in value order.
	Names() []string
}
