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

// BuildQuery writes the config props of a config as a URL query string.
//
// A [PropKeyResolver] writes only primary keys, and leaves out props that hold
// their default value.
//
// Parameters:
//   - cqr: the resolver bound to the config.
//
// Returns:
//   - string: the encoded query, sorted by key.
func BuildQuery(cqr types.ConfigQueryResolver) string {
	return BuildQueryWithCustomFields(cqr, url.Values{}).Encode()
}

// BuildQueryWithCustomFields adds the config props of a config to a query that
// holds custom fields.
//
// A custom field whose key is also a config key is moved to the key with the
// [KeyPrefix] prefix first, so the prop does not replace it. A
// [PropKeyResolver] writes only primary keys, and leaves out props that hold
// their default value.
//
// Parameters:
//   - cqr: the resolver bound to the config.
//   - query: the custom fields. The function changes and returns this query.
//
// Returns:
//   - url.Values: the query with the escaped custom fields and the config
//     props.
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

// SetConfigPropsFromQuery sets the config props from a query and returns the
// keys that are not config props.
//
// Each config key is matched exactly, and its first value is applied. Every
// config key is removed from the query, even one whose value fails. A key with
// the [KeyPrefix] prefix then takes the place of the config key it escapes.
// Unlike [ApplyQuery], a key that is not a config key is kept, not rejected.
//
// Parameters:
//   - cqr: the resolver bound to the config.
//   - query: the query to read. The function changes and returns this query.
//
// Returns:
//   - url.Values: the remaining custom fields, unescaped, or an empty query
//     when query is nil.
//   - error: the first error from setting a config prop, or nil. Later errors
//     are discarded.
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

// EscapeKey adds the [KeyPrefix] to a custom URL query key that a config prop
// key also uses.
//
// Parameters:
//   - key: the custom key.
//
// Returns:
//   - string: the escaped key.
func EscapeKey(key string) string {
	return KeyPrefix + key
}

// UnescapeKey removes the [KeyPrefix] from an escaped custom URL query key.
//
// Parameters:
//   - key: the key, escaped or not.
//
// Returns:
//   - string: the key without its prefix.
func UnescapeKey(key string) string {
	return strings.TrimPrefix(key, KeyPrefix)
}

// checkSpellings reports a config key that keys hold in more than one
// spelling.
//
// Keys that are not config keys are left to the caller, which rejects or
// ignores them.
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
