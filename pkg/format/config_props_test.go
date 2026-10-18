package format

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingProp is a config prop that cannot write its value.
type failingProp struct{}

// errGetProp is the error failingProp returns.
var errGetProp = errors.New("no value")

// TestGetConfigPropFromString verifies that a config prop type is read from
// its string, that its read error is wrapped, and that a type that is not a
// config prop fails with [ErrNotConfigProp].
func TestGetConfigPropFromString(t *testing.T) {
	t.Parallel()

	value, err := GetConfigPropFromString(reflect.TypeFor[subPropStruct](), "@alice")
	require.NoError(t, err)
	assert.Equal(t, &subPropStruct{Value: "alice"}, value.Interface())

	_, err = GetConfigPropFromString(reflect.TypeFor[subPropStruct](), "alice")
	require.ErrorIs(t, err, errInvalidProp)

	_, err = GetConfigPropFromString(reflect.TypeFor[subStruct](), "@alice")
	require.ErrorIs(t, err, ErrNotConfigProp)
}

// TestGetConfigPropString verifies that a config prop writes its string from
// a pointer or a value, that its write error is wrapped, and that a value that
// is not a config prop or cannot be read fails with [ErrNotConfigProp].
func TestGetConfigPropString(t *testing.T) {
	t.Parallel()

	prop := subPropStruct{Value: "alice"}

	value, err := GetConfigPropString(reflect.ValueOf(&prop))
	require.NoError(t, err)
	assert.Equal(t, "@alice", value)

	value, err = GetConfigPropString(reflect.ValueOf(prop))
	require.NoError(t, err)
	assert.Equal(t, "@alice", value)

	_, err = GetConfigPropString(reflect.ValueOf(&failingProp{}))
	require.ErrorIs(t, err, errGetProp)

	_, err = GetConfigPropString(reflect.ValueOf(subStruct{}))
	require.ErrorIs(t, err, ErrNotConfigProp)

	unexported := reflect.ValueOf(struct{ prop *subPropStruct }{prop: &prop}).Field(0)
	_, err = GetConfigPropString(unexported)
	require.ErrorIs(t, err, ErrNotConfigProp)
}

// GetPropValue fails with errGetProp.
//
// Returns:
//   - string: always "".
//   - error: always errGetProp.
func (*failingProp) GetPropValue() (string, error) {
	return "", errGetProp
}

// SetFromProp accepts any value.
//
// Parameters:
//   - string: unused.
//
// Returns:
//   - error: always nil.
func (*failingProp) SetFromProp(string) error {
	return nil
}
