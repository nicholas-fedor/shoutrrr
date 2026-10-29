// Package jsonopts holds the [encoding/json/v2] options that every JSON
// encoding and decoding in Shoutrrr uses.
//
// Request bodies keep sorted map keys, and invalid UTF-8 in them is replaced.
// Provider responses match member names exactly. JSON that users write
// decodes by the encoding/json rules, so names in another case and repeated
// names keep working.
package jsonopts
