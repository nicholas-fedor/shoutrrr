package types

// StdLogger is an interface for outputting log information from services that are non-fatal.
type StdLogger interface {
	// Print writes args in the manner of [fmt.Print].
	//
	// Parameters:
	//   - args: the values to write.
	Print(args ...any)
	// Printf writes args formatted in the manner of [fmt.Printf].
	//
	// Parameters:
	//   - format: the format string.
	//   - args: the values to format.
	Printf(format string, args ...any)
	// Println writes args in the manner of [fmt.Println].
	//
	// Parameters:
	//   - args: the values to write.
	Println(args ...any)
}
