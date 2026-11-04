package util

import "slices"

// NonEmpty returns values without its empty strings, such as the element a
// trailing comma leaves in a list setting.
//
// It never writes to values, so a config copy that shares its lists with the
// stored config can drop elements without changing the stored config. When
// values holds no empty string, NonEmpty returns values itself and allocates
// nothing.
//
// Parameters:
//   - values: the list.
//
// Returns:
//   - []string: values itself when it holds no empty string. Otherwise a new
//     slice with the other elements in order, or nil when none remain.
func NonEmpty(values []string) []string {
	first := slices.Index(values, "")
	if first < 0 {
		return values
	}

	kept := make([]string, first, len(values)-1)
	copy(kept, values[:first])

	for _, value := range values[first+1:] {
		if value != "" {
			kept = append(kept, value)
		}
	}

	if len(kept) == 0 {
		return nil
	}

	return kept
}
