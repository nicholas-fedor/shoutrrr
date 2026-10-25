package types

import "sort"

// Field is a Key/Value pair used for extra data in log messages.
type Field struct {
	// Key names the field.
	Key string
	// Value is the value of the field.
	Value string
}

// FieldsFromMap creates a Fields slice from a map, optionally sorting keys.
//
// Parameters:
//   - fieldMap: the keys and values of the fields.
//   - sorted: whether to sort the fields by key. Otherwise their order is
//     unspecified.
//
// Returns:
//   - []Field: the fields, one per map entry.
func FieldsFromMap(fieldMap map[string]string, sorted bool) []Field {
	keys := make([]string, 0, len(fieldMap))
	fields := make([]Field, len(fieldMap))

	for key := range fieldMap {
		keys = append(keys, key)
	}

	if sorted {
		sort.Strings(keys)
	}

	for i, key := range keys {
		fields[i].Key = key
		fields[i].Value = fieldMap[key]
	}

	return fields
}
