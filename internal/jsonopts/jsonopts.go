package jsonopts

import (
	"encoding/json/jsontext"
	"encoding/json/v2"

	jsonv1 "encoding/json"
)

// Option sets for encoding and decoding.
var (
	// Default is the option set for every [json.Marshal] and [json.Unmarshal]
	// call and their reader and writer variants. Decoding JSON that users
	// write uses [UserInput] instead.
	//
	// Each option keeps one encoding/json behavior that Shoutrrr's JSON needs:
	//   - [json.Deterministic] sorts map keys, so the same value always encodes to the same bytes.
	//   - [jsontext.AllowInvalidUTF8] replaces invalid UTF-8 with U+FFFD instead of failing,
	//     so a message with a stray byte still sends.
	//
	// Everything else follows the v2 defaults. A nil slice or map encodes as an
	// empty array or object. Member names match field names exactly, and a
	// repeated member name fails the decode.
	Default = json.JoinOptions(
		json.Deterministic(true),
		jsontext.AllowInvalidUTF8(true),
	)

	// UserInput is the option set for decoding JSON that users write, such as
	// a param or a provider file. It extends [Default] with the encoding/json
	// rules for reading input, so JSON that worked before keeps working:
	//   - [json.MatchCaseInsensitiveNames] matches member names to fields in any case.
	//   - [jsonv1.MatchCaseSensitiveDelimiter] keeps underscores and dashes significant in that match.
	//   - [jsontext.AllowDuplicateNames] accepts a repeated member name, and the last one wins.
	//   - [jsonv1.MergeWithLegacySemantics] makes a repeated name replace an earlier map value
	//     instead of merging into it, so the last value wins even when its type differs.
	//     A null then keeps a string, number, bool, or struct value decoded earlier.
	UserInput = json.JoinOptions(
		Default,
		json.MatchCaseInsensitiveNames(true),
		jsonv1.MatchCaseSensitiveDelimiter(true),
		jsontext.AllowDuplicateNames(true),
		jsonv1.MergeWithLegacySemantics(true),
	)
)
