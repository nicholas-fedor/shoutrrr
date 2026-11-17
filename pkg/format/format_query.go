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
// A custom field whose key matches a config key in any case is moved to the
// key with the [KeyPrefix] prefix first, so reading the query back keeps it a
// custom field. A [PropKeyResolver] writes only primary keys, and leaves out
// props that hold their default value.
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

	// The keys are collected first, because the loop adds escaped keys.
	for _, key := range slices.Collect(maps.Keys(query)) {
		if isConfigKey(key, fields) {
			query[EscapeKey(key)] = query[key]
			delete(query, key)
		}
	}

	pkr, isPkr := cqr.(*PropKeyResolver)

	for _, key := range fields {
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
// Config keys match and apply as in [ApplyQuery], with two differences. Each
// config key is tried and removed from the query even after a value fails.
// A key that is not a config key is kept, not rejected. A key with the
// [KeyPrefix] prefix whose rest matches a config key in any case then takes
// the place of that rest, as a custom key.
//
// Parameters:
//   - cqr: the resolver bound to the config.
//   - query: the query to read. The function changes and returns this query.
//
// Returns:
//   - url.Values: the remaining custom fields, unescaped, or an empty query
//     when query is nil. A query with conflicting keys comes back unchanged.
//   - error: an error wrapping [ErrConflictingKeys] for one key in two
//     spellings, or the first error from setting a config prop in key order.
//     Later errors are discarded.
func SetConfigPropsFromQuery(cqr types.ConfigQueryResolver, query url.Values) (url.Values, error) {
	if query == nil {
		return url.Values{}, nil
	}

	keys := slices.Sorted(maps.Keys(query))
	fields := cqr.QueryFields()

	if err := checkSpellings(keys, fields); err != nil {
		return query, err
	}

	var firstError error

	for _, key := range keys {
		if !isConfigKey(key, fields) {
			continue
		}

		// A key built in code may hold no values, so there is nothing to set.
		if values := query[key]; len(values) > 0 {
			if err := cqr.Set(key, values[0]); err != nil && firstError == nil {
				firstError = err
			}
		}

		delete(query, key)
	}

	// Escaped keys are restored once every config key is gone, so a restored
	// key never meets the config key it was escaped from.
	for _, key := range keys {
		custom, escaped := strings.CutPrefix(key, KeyPrefix)
		if escaped && isConfigKey(custom, fields) {
			query[custom] = query[key]
			delete(query, key)
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
		// The key is lowercased once, since params run this check on every send.
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

// isConfigKey reports whether a query or param key names a config key, in
// any case.
//
// Parameters:
//   - key: the key as written.
//   - configKeys: the config keys in lower case, as QueryFields returns them.
//
// Returns:
//   - bool: true when the key's lower-case form is one of configKeys.
func isConfigKey(key string, configKeys []string) bool {
	return slices.Contains(configKeys, strings.ToLower(key))
}
