package jsonopts

import (
	"encoding/json/jsontext"
	"encoding/json/v2"

	jsonv1 "encoding/json"
)

// Default is the option set passed to every [json.Marshal] and [json.Unmarshal]
// call, and to their reader and writer variants.
//
// Each option keeps one encoding/json behavior that the v2 defaults change:
//   - [json.Deterministic] sorts map keys.
//   - [json.FormatNilSliceAsNull] and [json.FormatNilMapAsNull] encode nil as null.
//   - [json.MatchCaseInsensitiveNames] matches member names to fields in any case.
//   - [jsonv1.MatchCaseSensitiveDelimiter] keeps underscores and dashes significant in that match.
//   - [jsontext.AllowDuplicateNames] accepts a repeated member name, and the last one wins.
//   - [jsonv1.MergeWithLegacySemantics] replaces map values and keeps non-pointer values on null when decoding into a non-zero value.
//   - [jsontext.AllowInvalidUTF8] replaces invalid UTF-8 with U+FFFD instead of failing.
var Default = json.JoinOptions(
	json.Deterministic(true),
	json.FormatNilSliceAsNull(true),
	json.FormatNilMapAsNull(true),
	json.MatchCaseInsensitiveNames(true),
	jsonv1.MatchCaseSensitiveDelimiter(true),
	jsontext.AllowDuplicateNames(true),
	jsonv1.MergeWithLegacySemantics(true),
	jsontext.AllowInvalidUTF8(true),
)
