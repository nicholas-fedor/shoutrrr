package format

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// urlQueryRead reads a URL query into the config a resolver is bound to,
// keeping the config's record of the keys the URL set.
type urlQueryRead struct {
	// cqr is the resolver bound to the config.
	cqr types.ConfigQueryResolver
	// pkr is cqr when it is a [PropKeyResolver], or nil.
	pkr *PropKeyResolver
	// record is the config's record, or nil when it has none.
	record *URLKeys
	// cleared reports whether the read has emptied the record.
	cleared bool
}

// KeyPrefix is the prefix used to escape custom URL query keys that conflict with service config prop keys.
// It consists of two underscore characters ("__").
const KeyPrefix = "__"

// Names of the key sources that errors from applying keys give.
const (
	// queryKeySource names the keys of a URL query.
	queryKeySource = "query parameter"
	// paramKeySource names the keys of send params.
	paramKeySource = "param"
)

// ApplyQuery sets the config props from a service URL's query.
//
// Keys match the config keys case-insensitively. A query that gives one key in
// two spellings, such as disabletls and DisableTLS, is rejected before any
// value is applied, because neither spelling is the one the user meant. The
// keys are applied in sorted order, each with its first value. The first error
// stops the update, so the error reported does not depend on map order. A
// deprecated key and its replacement are different keys. The
// replacement wins, as [PropKeyResolver.Set] documents.
//
// A config that embeds [URLKeys] starts its record anew and records each key
// that this query sets.
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
	read := newURLQueryRead(cqr)

	err := applyKeys(slices.Sorted(maps.Keys(query)), cqr.QueryFields(), queryKeySource, nil, func(key, lower string) error {
		// A key built in code may hold no values, which reads as empty.
		return read.set(key, lower, query.Get(key))
	})
	if err == nil {
		read.clearRecord()
	}

	return err
}

// BuildQuery writes the config props of a config as a URL query string.
//
// A [PropKeyResolver] writes only primary keys, and leaves out props that hold
// their default value. It still writes a level-mapped prop that the config's
// URL set to its default, as [URLKeys] describes. It writes a value set only
// through a deprecated field under its replacement's key, as
// [SyncDeprecatedFields] would copy it. The config itself is not changed.
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
// custom field. The config props are written as [BuildQuery] writes them.
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

	if pkr, isPkr := cqr.(*PropKeyResolver); isPkr {
		pkr.writeQuery(query)

		return query
	}

	for _, key := range fields {
		if value, err := cqr.Get(key); err == nil {
			query.Set(key, value)
		}
	}

	return query
}

// SetConfigPropsFromQuery sets the config props from a query and returns the
// keys that are not config props.
//
// Config keys match and apply as in [ApplyQuery], and are removed from the
// query. A key that is not a config key is kept, not rejected. A key with the
// [KeyPrefix] prefix whose rest matches a config key in any case then takes
// the place of that rest, as a custom key. A config that embeds [URLKeys]
// starts its record anew, as in [ApplyQuery].
//
// Parameters:
//   - cqr: the resolver bound to the config.
//   - query: the query to read. The function changes and returns this query.
//     After an error it is left part-way changed.
//
// Returns:
//   - url.Values: the remaining custom fields with their keys unescaped, or
//     nil with an error. A nil query gives an empty query.
//   - error: an error wrapping [ErrConflictingKeys] for one key in two
//     spellings, or the first value error in key order. The error names its
//     key.
func SetConfigPropsFromQuery(cqr types.ConfigQueryResolver, query url.Values) (url.Values, error) {
	read := newURLQueryRead(cqr)

	if query == nil {
		read.clearRecord()

		return url.Values{}, nil
	}

	keys := slices.Sorted(maps.Keys(query))
	fields := cqr.QueryFields()

	err := applyKeys(keys, fields, queryKeySource, nil, func(key, lower string) error {
		if !slices.Contains(fields, lower) {
			return nil
		}

		values := query[key]
		delete(query, key)

		// A key built in code may hold no values, so there is nothing to set.
		if len(values) == 0 {
			return nil
		}

		return read.set(key, lower, values[0])
	})
	if err != nil {
		return nil, err
	}

	read.clearRecord()

	// Escaped keys are restored once every config key is gone, so a restored
	// key never meets the config key it was escaped from.
	for _, key := range keys {
		custom, escaped := strings.CutPrefix(key, KeyPrefix)
		if escaped && isConfigKey(custom, fields) {
			query[custom] = query[key]
			delete(query, key)
		}
	}

	return query, nil
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

// applyKeys applies keys in order through apply, after checking that they
// hold no config key in two spellings. The first error stops it.
//
// Each key is lowercased once, since send params run through it on every send.
//
// Parameters:
//   - keys: the query or param keys, in the order to apply them. Keys that
//     first picks move to the front. Both groups keep their order.
//   - configKeys: the config keys in lower case, as QueryFields returns them.
//   - source: what the keys are, such as "query parameter", named in errors.
//   - first: reports whether a key, given in lower case, goes first. Nil
//     keeps the order.
//   - apply: applies one key, given as written and in lower case. It skips a
//     key it does not apply by returning nil.
//
// Returns:
//   - error: an error wrapping [ErrConflictingKeys] for one key in two
//     spellings, or the first error from apply. The error names its key.
func applyKeys(
	keys, configKeys []string,
	source string,
	first func(lower string) bool,
	apply func(key, lower string) error,
) error {
	// Most queries and params hold few keys, so their lower-case forms fit
	// on the stack.
	var buf [8]string

	lowers := buf[:0]
	for _, key := range keys {
		lowers = append(lowers, strings.ToLower(key))
	}

	if first != nil {
		front := 0

		for i, lower := range lowers {
			if !first(lower) {
				continue
			}

			key := keys[i]
			copy(keys[front+1:i+1], keys[front:i])
			copy(lowers[front+1:i+1], lowers[front:i])
			keys[front], lowers[front] = key, lower
			front++
		}
	}

	if err := checkSpellings(keys, lowers, configKeys); err != nil {
		return err
	}

	for i, key := range keys {
		if err := apply(key, lowers[i]); err != nil {
			return fmt.Errorf("setting %s %q: %w", source, key, err)
		}
	}

	return nil
}

// checkSpellings reports a config key that keys hold in more than one
// spelling.
//
// Keys that are not config keys are left to the caller, which rejects or
// ignores them.
//
// Parameters:
//   - keys: the query or param keys, in a fixed order.
//   - lowers: the keys in lower case, in the same order.
//   - configKeys: the config keys in lower case, as QueryFields returns them.
//
// Returns:
//   - error: an error wrapping [ErrConflictingKeys] that names the first two
//     spellings of one key, or nil.
func checkSpellings(keys, lowers, configKeys []string) error {
	spellings := make(map[string]string, len(keys))

	for i, key := range keys {
		lower := lowers[i]
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

// newURLQueryRead starts reading a URL query into the config that cqr is
// bound to.
//
// Parameters:
//   - cqr: the resolver bound to the config.
//
// Returns:
//   - *urlQueryRead: the read, which reaches the config's record when cqr is a
//     [PropKeyResolver] bound to a config that embeds [URLKeys].
func newURLQueryRead(cqr types.ConfigQueryResolver) *urlQueryRead {
	read := &urlQueryRead{cqr: cqr, pkr: nil, record: nil, cleared: false}

	if pkr, isPkr := cqr.(*PropKeyResolver); isPkr {
		read.pkr = pkr
		read.record = pkr.boundURLKeys()
	}

	return read
}

// clearRecord empties the config's record, once per read.
//
// A read calls it when it sets its first key, and again after a query that
// set none succeeds, so a rejected query leaves the record of the last URL as
// it is.
func (r *urlQueryRead) clearRecord() {
	if !r.cleared {
		r.record.reset()
		r.cleared = true
	}
}

// set sets a config prop from the query, and records its key once the value
// is set.
//
// Parameters:
//   - key: the query key as written.
//   - lower: the query key in lower case.
//   - value: the query value.
//
// Returns:
//   - error: the error from the resolver's Set, or nil.
func (r *urlQueryRead) set(key, lower, value string) error {
	if err := r.cqr.Set(key, value); err != nil {
		return err
	}

	r.clearRecord()

	if r.pkr != nil {
		if field, found := r.pkr.keyFields[lower]; found {
			r.record.add(field.Name)
		}
	}

	return nil
}
