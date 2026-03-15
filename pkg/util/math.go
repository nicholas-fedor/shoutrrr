package util

// Min returns the smaller of two integers.
//
// Parameters:
//   - a: The first integer to compare.
//   - b: The second integer to compare.
//
// Returns:
//   - The smaller of a and b. If they are equal, returns a.
func Min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

// Max returns the larger of two integers.
//
// Parameters:
//   - a: The first integer to compare.
//   - b: The second integer to compare.
//
// Returns:
//   - The larger of a and b. If they are equal, returns a.
func Max(a, b int) int {
	if a > b {
		return a
	}

	return b
}
