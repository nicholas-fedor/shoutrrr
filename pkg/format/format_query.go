package format

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// KeyPrefix is the prefix used to escape custom URL query keys that conflict with service config prop keys.
// It consists of two underscore characters ("__").
const KeyPrefix = "__"

// ApplyQuery sets the config props from a service URL's query.
//
// Keys match the config keys case-insensitively. A query that gives one key in
// two spellings, such as disabletls and DisableTLS, is rejected before any
// value is applied, because neither spelling is the one the user meant. The
// keys are applied in sorted order, each with its first value. The first error
// reported then does not depend on map order. A deprecated key and its
// replacement are different keys. The replacement wins, as
// [PropKeyResolver.Set] documents.
//
// Parameters:
//   - cqr: the resolver bound to the config.
//   - query: the parsed URL query, such as the result of util.ParseURLQuery.
//
// Returns:
//   - error: an error wrapping [ErrConflictingKeys] for a key in two
//     spellings, or the first key's [ErrInvalidConfigKey] or value error,
//     naming the key.
func ApplyQuery(cqr types.ConfigQueryResolver, query url.Values) error {
	keys := slices.Sorted(maps.Keys(query))

	if err := checkSpellings(keys, cqr.QueryFields()); err != nil {
		return err
	}

	for _, key := range keys {
		if err := cqr.Set(key, query[key][0]); err != nil {
			return fmt.Errorf("setting query parameter %q: %w", key, err)
		}
	}

	return nil
}

// BuildQuery converts the fields of a config object to a delimited query string.
func BuildQuery(cqr types.ConfigQueryResolver) string {
	return BuildQueryWithCustomFields(cqr, url.Values{}).Encode()
}

// BuildQueryWithCustomFields converts the fields of a config object to a delimited query string,
// escaping any custom fields that share the same key as a config prop using a "__" prefix.
func BuildQueryWithCustomFields(cqr types.ConfigQueryResolver, query url.Values) url.Values {
	fields := cqr.QueryFields()
	skipEscape := len(query) < 1

	pkr, isPkr := cqr.(*PropKeyResolver)

	for _, key := range fields {
		if !skipEscape {
			// Escape any webhook query keys using the same name as service props
			escValues := query[key]
			if len(escValues) > 0 {
				query.Del(key)
				query[EscapeKey(key)] = escValues
			}
		}

		if isPkr && !pkr.KeyIsPrimary(key) {
			continue
		}

		value, err := cqr.Get(key)
		if err != nil || isPkr && pkr.IsDefault(key, value) {
			continue
		}

		query.Set(key, value)
	}

	return query
}

// SetConfigPropsFromQuery iterates over all the config prop keys and sets the config prop to the corresponding
// query value based on the key. It returns a non-nil url.Values query with all config prop keys removed,
// even if any of them could not be used to set a config field, and with any escaped keys unescaped.
// The error returned is the first error that occurred, subsequent errors are just discarded.
func SetConfigPropsFromQuery(cqr types.ConfigQueryResolver, query url.Values) (url.Values, error) {
	var firstError error

	if query == nil {
		return url.Values{}, nil
	}

	for _, key := range cqr.QueryFields() {
		// Retrieve the service-related prop value
		values := query[key]
		if len(values) > 0 {
			if err := cqr.Set(key, values[0]); err != nil && firstError == nil {
				firstError = err
			}
		}
		// Remove it from the query Values
		query.Del(key)

		// If an escaped version of the key exist, unescape it
		escKey := EscapeKey(key)
		escValues := query[escKey]

		if len(escValues) > 0 {
			query.Del(escKey)
			query[key] = escValues
		}
	}

	return query, firstError
}

// EscapeKey adds the KeyPrefix to custom URL query keys that conflict with service config prop keys.
func EscapeKey(key string) string {
	return KeyPrefix + key
}

// UnescapeKey removes the KeyPrefix from custom URL query keys that conflict with service config prop keys.
func UnescapeKey(key string) string {
	return strings.TrimPrefix(key, KeyPrefix)
}

// checkSpellings reports a config key that keys hold in more than one
// spelling. Keys that are not config keys are left to the caller, which
// rejects or ignores them.
//
// Parameters:
//   - keys: the query or param keys, in a fixed order.
//   - configKeys: the config keys in lower case, as QueryFields returns them.
//
// Returns:
//   - error: an error wrapping [ErrConflictingKeys] that names the first two
//     spellings of one key, or nil.
func checkSpellings(keys, configKeys []string) error {
	spellings := make(map[string]string, len(keys))

	for _, key := range keys {
		lower := strings.ToLower(key)
		if !slices.Contains(configKeys, lower) {
			continue
		}

		if first, found := spellings[lower]; found {
			return fmt.Errorf("%w: %q and %q", ErrConflictingKeys, first, key)
		}

		spellings[lower] = key
	}

	return nil
}
