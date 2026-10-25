package types

// ConfigProp interface is used to de-/serialize structs from/to a string representation.
type ConfigProp interface {
	// SetFromProp sets the receiver from its string representation.
	//
	// Parameters:
	//   - propValue: the string representation, such as a service URL query
	//     value.
	//
	// Returns:
	//   - error: an error when propValue is not a valid representation.
	SetFromProp(propValue string) error
	// GetPropValue returns the string representation of the receiver.
	//
	// Returns:
	//   - string: the string representation.
	//   - error: an error when the receiver has no valid representation.
	GetPropValue() (string, error)
}
