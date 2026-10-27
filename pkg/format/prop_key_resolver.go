package format

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// PropKeyResolver implements the ConfigQueryResolver interface for services that uses key tags for query props.
type PropKeyResolver struct {
	confValue reflect.Value
	keyFields map[string]*FieldInfo
	// setFields holds the names of the fields that Set has set, such as from a
	// service URL query, so KeepsDefault treats them as explicitly configured.
	setFields map[string]bool
	// deprecatedKeys maps each deprecated key to the primary key of its field.
	deprecatedKeys map[string]string
	// usedDeprecatedKeys holds the deprecated keys that Set has received.
	usedDeprecatedKeys map[string]bool
	// primarySetFields holds the names of the fields that Set has set through
	// a key that is not deprecated, so a deprecated key no longer overrides them.
	primarySetFields map[string]bool
	keys             []string
}

var (
	ErrInvalidConfigKey    = errors.New("not a valid config key")
	ErrInvalidValueForType = errors.New("invalid value for type")
)

// NewPropKeyResolver creates a new PropKeyResolver and initializes it using the provided config.
func NewPropKeyResolver(config types.ServiceConfig) PropKeyResolver {
	configNode := GetConfigFormat(config)
	items := configNode.Items

	keyFields := make(map[string]*FieldInfo, len(items))
	deprecatedKeys := map[string]string{}
	keys := make([]string, 0, len(items))

	for _, item := range items {
		field := *item.Field()
		if len(field.Keys) == 0 {
			continue // Skip fields without explicit 'key' tags
		}

		for _, key := range field.Keys {
			key = strings.ToLower(key)
			if key != "" {
				keys = append(keys, key)
				keyFields[key] = &field
			}
		}

		for _, key := range field.DeprecatedKeys {
			if key != "" {
				keys = append(keys, key)
				keyFields[key] = &field
				deprecatedKeys[key] = field.Keys[0]
			}
		}
	}

	sort.Strings(keys)

	confValue := reflect.ValueOf(config)
	if confValue.Kind() == reflect.Pointer {
		confValue = confValue.Elem()
	}

	return PropKeyResolver{
		keyFields:          keyFields,
		confValue:          confValue,
		setFields:          map[string]bool{},
		deprecatedKeys:     deprecatedKeys,
		usedDeprecatedKeys: map[string]bool{},
		primarySetFields:   map[string]bool{},
		keys:               keys,
	}
}

// GetConfigQueryResolver returns the ConfigQueryResolver interface for the config if it implements it,
// otherwise it creates and returns a PropKeyResolver that implements it.
func GetConfigQueryResolver(config types.ServiceConfig) types.ConfigQueryResolver {
	var resolver types.ConfigQueryResolver

	var ok bool
	if resolver, ok = config.(types.ConfigQueryResolver); !ok {
		pkr := NewPropKeyResolver(config)
		resolver = &pkr
	}

	return resolver
}

// Bind creates a new instance of the PropKeyResolver with the internal config reference
// set to the provided config. This should only be used for configs of the same type.
func (pkr *PropKeyResolver) Bind(config types.ServiceConfig) PropKeyResolver {
	bound := *pkr
	bound.confValue = configValue(config)

	return bound
}

// DeprecatedKeys reports the deprecated keys that [PropKeyResolver.Set] has
// received, so the service can warn about them. They usually come from the
// service URL query.
//
// Returns:
//   - []Deprecation: the deprecated keys in lower case with their primary
//     keys, sorted by key.
func (pkr *PropKeyResolver) DeprecatedKeys() []Deprecation {
	used := make([]Deprecation, 0, len(pkr.usedDeprecatedKeys))

	for _, key := range slices.Sorted(maps.Keys(pkr.usedDeprecatedKeys)) {
		used = append(used, Deprecation{Name: key, Replacement: pkr.deprecatedKeys[key]})
	}

	return used
}

// DeprecatedParams reports the deprecated keys among params. It only reads the
// resolver, so concurrent sends can call it.
//
// Parameters:
//   - params: the send params, or nil.
//
// Returns:
//   - []Deprecation: the deprecated keys in lower case with their primary
//     keys, sorted by key.
func (pkr *PropKeyResolver) DeprecatedParams(params *types.Params) []Deprecation {
	if params == nil {
		return []Deprecation{}
	}

	used := map[string]bool{}

	for key := range *params {
		if key = strings.ToLower(key); pkr.deprecatedKeys[key] != "" {
			used[key] = true
		}
	}

	deprecations := make([]Deprecation, 0, len(used))

	for _, key := range slices.Sorted(maps.Keys(used)) {
		deprecations = append(deprecations, Deprecation{Name: key, Replacement: pkr.deprecatedKeys[key]})
	}

	return deprecations
}

// Get returns the value of a config property tagged with the corresponding key.
func (pkr *PropKeyResolver) Get(key string) (string, error) {
	if field, found := pkr.keyFields[strings.ToLower(key)]; found {
		return GetConfigFieldString(pkr.confValue, field)
	}

	return "", fmt.Errorf("%w: %v", ErrInvalidConfigKey, key)
}

// IsDefault returns whether the specified key value is the default value.
// The value matches the default as written in its tag, or as the formatter
// prints it, so a bool default of "yes" matches "Yes" and an enum default of
// "high" matches "High". A property without a default tag, or with an empty
// one, defaults to its zero value, as [PropKeyResolver.SetDefaultProps] sets
// it, so "0" and "No" match it.
//
// Parameters:
//   - key: the property key, matched case-insensitively.
//   - value: the formatted property value.
//
// Returns:
//   - bool: whether value is the property's default, or false for an unknown key.
func (pkr *PropKeyResolver) IsDefault(key, value string) bool {
	field, found := pkr.keyFields[strings.ToLower(key)]
	if !found {
		return false
	}

	if field.DefaultValue == value {
		return true
	}

	// A resolver without a bound config has no type to format the default with.
	if !pkr.confValue.IsValid() {
		return false
	}

	// A new config holds the zero value, which is the default when the tag is
	// empty.
	defaults := reflect.New(pkr.confValue.Type()).Elem()
	if field.DefaultValue != "" {
		if valid, err := SetConfigField(defaults, field, field.DefaultValue); !valid || err != nil {
			return false
		}
	}

	formatted, err := GetConfigFieldString(defaults, field)

	return err == nil && formatted == value
}

// KeepsDefault reports whether a send keeps the default value of the property
// tagged with key. That holds when neither [PropKeyResolver.Set], as used for
// the service URL query, nor params set the property under any of its keys,
// and config holds the property's default value. A property set to its default
// value explicitly is therefore not kept as a default.
//
// A service uses it to fill a property from another source, such as mapping
// the message level to a priority, only when neither the service URL nor the
// send asked for a value of its own.
//
// Parameters:
//   - config: the config for this send, with params already applied.
//   - params: the send params, or nil.
//   - key: the property key, matched case-insensitively.
//
// Returns:
//   - bool: whether the property keeps its default, or false for an unknown key.
func (pkr *PropKeyResolver) KeepsDefault(config types.ServiceConfig, params *types.Params, key string) bool {
	field, found := pkr.keyFields[strings.ToLower(key)]
	if !found || pkr.setFields[field.Name] {
		return false
	}

	if params != nil {
		for paramKey := range *params {
			if paramField, isField := pkr.keyFields[strings.ToLower(paramKey)]; isField && paramField.Name == field.Name {
				return false
			}
		}
	}

	bound := pkr.Bind(config)

	value, err := bound.Get(key)

	return err == nil && bound.IsDefault(key, value)
}

// KeyIsPrimary returns whether the key is the primary (and not an alias or a
// deprecated key).
//
// Parameters:
//   - key: the property key, matched case-insensitively.
//
// Returns:
//   - bool: whether key is the first key of its property, or false for an unknown key.
func (pkr *PropKeyResolver) KeyIsPrimary(key string) bool {
	key = strings.ToLower(key)

	field, found := pkr.keyFields[key]

	return found && field.Keys[0] == key
}

// QueryFields returns a list of tagged keys.
func (pkr *PropKeyResolver) QueryFields() []string {
	return pkr.keys
}

// Set sets the value of its bound struct's property, tagged with the corresponding key.
// It records the property as explicitly set, which [PropKeyResolver.KeepsDefault] reads.
//
// A deprecated key is recorded for [PropKeyResolver.DeprecatedKeys]. It sets
// the property unless a key that is not deprecated already set it, so the
// replacement key wins whatever the order of the URL query. Its value is
// checked either way.
//
// Parameters:
//   - key: the property key, matched case-insensitively.
//   - value: the formatted property value.
//
// Returns:
//   - error: [ErrInvalidConfigKey] for an unknown key, or the value's parse error.
func (pkr *PropKeyResolver) Set(key, value string) error {
	key = strings.ToLower(key)
	field, found := pkr.keyFields[key]
	_, deprecated := pkr.deprecatedKeys[key]

	target := pkr.confValue
	if found && deprecated && pkr.primarySetFields[field.Name] {
		// The value is still parsed, into a scratch config, to report errors.
		target = reflect.New(pkr.confValue.Type()).Elem()
	}

	if err := pkr.set(target, key, value); err != nil {
		return err
	}

	if found {
		pkr.recordSet(key, field.Name, deprecated)
	}

	return nil
}

// SetDefaultProps mutates the provided config, setting the tagged fields with their default values.
// A field without a default tag, or with an empty one, is set to its zero value.
// If the provided config is nil, the internal config will be updated instead.
// The error returned is the first error that occurred. Later errors are discarded.
//
// Parameters:
//   - config: the config to update, or nil for the resolver's own config.
//
// Returns:
//   - error: the first error from parsing a default value.
func (pkr *PropKeyResolver) SetDefaultProps(config types.ServiceConfig) error {
	var firstError error

	confValue := pkr.configValueOrInternal(config)
	for key, info := range pkr.keyFields {
		// An empty value does not parse as a number, bool, duration, or enum.
		if info.DefaultValue == "" {
			confValue.FieldByName(info.Name).SetZero()

			continue
		}

		if err := pkr.set(confValue, key, info.DefaultValue); err != nil && firstError == nil {
			firstError = err
		}
	}

	return firstError
}

// UpdateConfigFromParams mutates the provided config, updating the values from its corresponding params.
// If the provided config is nil, the internal config will be updated instead.
//
// The reserved params set by [types.Params] helpers (title, message and level) are
// applied when the config declares them and skipped otherwise, so every service
// accepts them. Any other param without a matching key is an error.
// Params are applied in key order, with deprecated keys first, so a
// replacement key wins when params hold both.
// The error returned is the first error that occurred; subsequent errors are discarded.
func (pkr *PropKeyResolver) UpdateConfigFromParams(
	config types.ServiceConfig,
	params *types.Params,
) error {
	var firstError error

	confValue := pkr.configValueOrInternal(config)

	if params == nil {
		return nil
	}

	// Deprecated keys go first, so a replacement key in the same params wins.
	keys := slices.SortedFunc(maps.Keys(*params), func(a, b string) int {
		return cmp.Or(cmp.Compare(pkr.deprecatedOrder(a), pkr.deprecatedOrder(b)), strings.Compare(a, b))
	})

	for _, key := range keys {
		if pkr.isUndeclaredReserved(key) {
			continue
		}

		if err := pkr.set(confValue, key, (*params)[key]); err != nil && firstError == nil {
			firstError = err
		}
	}

	return firstError
}

// configValueOrInternal returns the reflect.Value of the config, or the internal config if config is nil.
func (pkr *PropKeyResolver) configValueOrInternal(config types.ServiceConfig) reflect.Value {
	if config != nil {
		return configValue(config)
	}

	return pkr.confValue
}

// deprecatedOrder sorts deprecated keys before the other keys.
//
// Parameters:
//   - key: the param key, matched case-insensitively.
//
// Returns:
//   - int: 0 for a deprecated key and 1 for any other key.
func (pkr *PropKeyResolver) deprecatedOrder(key string) int {
	if _, deprecated := pkr.deprecatedKeys[strings.ToLower(key)]; deprecated {
		return 0
	}

	return 1
}

// isUndeclaredReserved reports whether key is a reserved param that the config does
// not declare.
//
// Parameters:
//   - key: the param key, matched case-insensitively.
//
// Returns:
//   - bool: true when the param should be skipped.
func (pkr *PropKeyResolver) isUndeclaredReserved(key string) bool {
	key = strings.ToLower(key)
	if _, declared := pkr.keyFields[key]; declared {
		return false
	}

	switch key {
	case types.TitleKey, types.MessageKey, types.LevelKey:
		return true
	default:
		return false
	}
}

// recordSet records a property set by [PropKeyResolver.Set], allocating the
// maps of a zero-value resolver.
//
// Parameters:
//   - key: the property key in lower case.
//   - fieldName: the name of the field the key sets.
//   - deprecated: whether key is a deprecated key.
func (pkr *PropKeyResolver) recordSet(key, fieldName string, deprecated bool) {
	if pkr.setFields == nil {
		pkr.setFields = map[string]bool{}
	}

	pkr.setFields[fieldName] = true

	if !deprecated {
		if pkr.primarySetFields == nil {
			pkr.primarySetFields = map[string]bool{}
		}

		pkr.primarySetFields[fieldName] = true

		return
	}

	if pkr.usedDeprecatedKeys == nil {
		pkr.usedDeprecatedKeys = map[string]bool{}
	}

	pkr.usedDeprecatedKeys[key] = true
}

// set sets the value of a target struct tagged with the corresponding key.
func (pkr *PropKeyResolver) set(target reflect.Value, key, value string) error {
	if field, found := pkr.keyFields[strings.ToLower(key)]; found {
		valid, err := SetConfigField(target, field, value)
		if !valid && err == nil {
			return ErrInvalidValueForType
		}

		return err
	}

	return fmt.Errorf("%w: %v (valid keys: %v)", ErrInvalidConfigKey, key, pkr.keys)
}

// configValue returns the indirect reflect.Value of the config.
func configValue(config types.ServiceConfig) reflect.Value {
	return reflect.Indirect(reflect.ValueOf(config))
}
