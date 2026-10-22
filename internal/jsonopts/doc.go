// Package jsonopts holds the [encoding/json/v2] options that every JSON
// encoding and decoding in Shoutrrr uses.
//
// The options keep the JSON that services send and accept the same as
// encoding/json produced and read it. Request bodies keep sorted map keys and
// null for nil slices and maps. Provider responses and user-supplied JSON still
// decode when their member names differ in case, repeat a name, or carry
// invalid UTF-8.
package jsonopts
