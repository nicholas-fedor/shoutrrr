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
	"sync"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// PropKeyResolver implements the ConfigQueryResolver interface for services that uses key tags for query props.
type PropKeyResolver struct {
	// keyFields maps each key and deprecated key, in lower case, to its field.
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
	// defaults holds the config's default values, built on first use and
	// shared with the resolvers that Bind returns. It is nil in a resolver not
	// made by NewPropKeyResolver, which builds the defaults on each use instead.
	defaults *configDefaults
	// confValue is the config the resolver reads and sets.
	confValue reflect.Value
	// keys lists every key and deprecated key in lower case, sorted.
	keys []string
	// fields lists each keyed field once, sorted by field name.
	fields []*FieldInfo
}

// configDefaults is a config holding the default value of each keyed field.
//
// It is built once and only read afterwards, so concurrent sends can share it.
type configDefaults struct {
	// err is the parse error of the first field, by field name, whose default
	// tag does not parse.
	err error
	// invalid holds the names of the fields whose default tag does not parse.
	// Such a field never holds its default, as its default cannot be compared.
	invalid map[string]bool
	// value is the config struct, with each keyed field set to its default and
	// a field without a default tag left at its zero value.
	value reflect.Value
	// copied lists the fields whose default is copied from value.
	copied []*FieldInfo
	// parsed lists the fields whose default holds a list, map, or other
	// reference. Their default is parsed again for each config, so no config
	// shares memory with value.
	parsed []*FieldInfo
	// once builds the defaults on first use.
	once sync.Once
}

// Errors returned when config props are read or set.
var (
	// ErrInvalidConfigKey indicates a key that names no config prop.
	ErrInvalidConfigKey = errors.New("not a valid config key")
	// ErrInvalidValueForType indicates a value that does not parse as its
	// prop's type.
	ErrInvalidValueForType = errors.New("invalid value for type")
	// ErrConflictingKeys indicates a URL query or send params that give one
	// config key in two spellings, such as disabletls and DisableTLS.
	ErrConflictingKeys = errors.New("config key given in more than one spelling")
)

// NewPropKeyResolver creates a new PropKeyResolver and initializes it using the provided config.
//
// The resolver knows the fields with a `key` tag. Each key and deprecated key
// matches in any case.
//
// Parameters:
//   - config: the config to resolve keys for, which the resolver is bound to.
//
// Returns:
//   - PropKeyResolver: the resolver.
func NewPropKeyResolver(config types.ServiceConfig) PropKeyResolver {
	configNode := GetConfigFormat(config)
	items := configNode.Items

	keyFields := make(map[string]*FieldInfo, len(items))
	deprecatedKeys := map[string]string{}
	keys := make([]string, 0, len(items))
	fields := make([]*FieldInfo, 0, len(items))

	for _, item := range items {
		field := *item.Field()
		if len(field.Keys) == 0 {
			continue // Skip fields without explicit 'key' tags
		}

		keyCount := len(keys)

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

		// A field whose keys are all empty, as from key:"", has no key to set.
		if len(keys) > keyCount {
			fields = append(fields, &field)
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
		fields:             fields,
		defaults:           &configDefaults{err: nil, invalid: nil, value: reflect.Value{}, copied: nil, parsed: nil, once: sync.Once{}},
	}
}

// GetConfigQueryResolver returns a query resolver for a config.
//
// A config that implements [types.ConfigQueryResolver] is its own resolver.
// Any other config gets a [PropKeyResolver].
//
// Parameters:
//   - config: the config to resolve keys for.
//
// Returns:
//   - types.ConfigQueryResolver: the resolver.
func GetConfigQueryResolver(config types.ServiceConfig) types.ConfigQueryResolver {
	var resolver types.ConfigQueryResolver

	var ok bool
	if resolver, ok = config.(types.ConfigQueryResolver); !ok {
		pkr := NewPropKeyResolver(config)
		resolver = &pkr
	}

	return resolver
}

// Bind returns a copy of the resolver bound to another config.
//
// The config must have the same type as the one the resolver was made for. The
// copy shares the record of set fields and deprecated keys with the original,
// and the config's defaults.
//
// Parameters:
//   - config: the config to bind.
//
// Returns:
//   - PropKeyResolver: the bound copy.
func (pkr *PropKeyResolver) Bind(config types.ServiceConfig) PropKeyResolver {
	bound := *pkr
	bound.confValue = configValue(config)

	return bound
}

// DeprecatedKeys reports the deprecated keys that [PropKeyResolver.Set] has
// received.
//
// A service warns about them. They usually come from the service URL query.
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

// DeprecatedParams reports the deprecated keys among params.
//
// It only reads the resolver, so concurrent sends can call it.
//
// Parameters:
//   - params: the send params, or nil.
//
// Returns:
//   - []Deprecation: the deprecated keys in lower case with their primary
//     keys, sorted by key.
func (pkr *PropKeyResolver) DeprecatedParams(params *types.Params) []Deprecation {
	deprecations := []Deprecation{}
	if params == nil || len(pkr.deprecatedKeys) == 0 {
		return deprecations
	}

	// Services check their params on every send, and a send seldom holds a
	// deprecated key, so nothing is allocated until one turns up.
	for key := range *params {
		name := strings.ToLower(key)

		replacement, deprecated := pkr.deprecatedKeys[name]
		if !deprecated || slices.ContainsFunc(deprecations, func(d Deprecation) bool { return d.Name == name }) {
			continue
		}

		deprecations = append(deprecations, Deprecation{Name: name, Replacement: replacement})
	}

	slices.SortFunc(deprecations, func(a, b Deprecation) int { return strings.Compare(a.Name, b.Name) })

	return deprecations
}

// Get returns the value of a config property tagged with the corresponding key.
//
// Parameters:
//   - key: the property key, matched case-insensitively.
//
// Returns:
//   - string: the formatted value.
//   - error: [ErrInvalidConfigKey] for an unknown key, or the formatting
//     error, or nil.
func (pkr *PropKeyResolver) Get(key string) (string, error) {
	if field, found := pkr.keyFields[strings.ToLower(key)]; found {
		return GetConfigFieldString(pkr.confValue, field)
	}

	return "", fmt.Errorf("%w: %v", ErrInvalidConfigKey, key)
}

// IsDefault reports whether a value is the default of the property tagged with
// key.
//
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

	return pkr.fieldIsDefault(field, value)
}

// KeepsDefault reports whether a send keeps the default value of the property
// tagged with key.
//
// That holds when neither [PropKeyResolver.Set], as used for
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

	value, err := GetConfigFieldString(configValue(config), field)

	return err == nil && pkr.fieldIsDefault(field, value)
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
//
// Returns:
//   - []string: every key and deprecated key in lower case, sorted. The
//     slice is the resolver's own, so callers must not change it.
func (pkr *PropKeyResolver) QueryFields() []string {
	return pkr.keys
}

// Set sets the value of its bound struct's property, tagged with the corresponding key.
//
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

// SetDefaultProps sets each tagged field of a config to its default value.
//
// A field without a default tag, or with an empty one, is set to its zero
// value. A field whose default does not parse keeps its value, and the other
// fields are still set.
//
// Parameters:
//   - config: the config to update, or nil for the resolver's own config.
//
// Returns:
//   - error: the parse error of the first field, by field name, whose
//     default did not parse when the defaults were built, else the first
//     error from parsing a list, map, or other reference default again, or
//     nil.
func (pkr *PropKeyResolver) SetDefaultProps(config types.ServiceConfig) error {
	// A resolver without a bound config has no keyed fields.
	if !pkr.confValue.IsValid() {
		return nil
	}

	defaults := pkr.defaultConfig()
	confValue := pkr.configValueOrInternal(config)
	copied, parsed := defaults.copied, defaults.parsed

	// The cached defaults copy only into a config of their own type. A config
	// of another type can give a field of the same name another type, so each
	// of its defaults is set by name from the tag instead.
	if confValue.Type() != defaults.value.Type() {
		copied, parsed = nil, pkr.fields
	}

	for _, field := range copied {
		field.value(confValue).Set(field.value(defaults.value))
	}

	// A list, map, or other reference default is parsed again into each
	// config. A [types.ConfigProp] from outside this package may reject a
	// value it accepted when the defaults were built, so its error is kept.
	err := defaults.err

	for _, field := range parsed {
		// An empty value does not parse as a number, bool, duration, or enum.
		if field.DefaultValue == "" {
			field.value(confValue).SetZero()

			continue
		}

		_, parseErr := SetConfigField(confValue, field, field.DefaultValue)
		if err == nil {
			err = parseErr
		}
	}

	return err
}

// UpdateConfigFromKnownParams updates a config from the params whose keys are
// config keys and ignores the rest.
//
// Keys match in any case. A service whose other params fill its payload, such
// as a webhook body, uses it. The params are applied as
// [PropKeyResolver.UpdateConfigFromParams] applies them.
//
// Parameters:
//   - config: the config to update, or nil for the resolver's own config.
//   - params: the send params, or nil for none.
//
// Returns:
//   - error: an error wrapping [ErrConflictingKeys] for one key in two
//     spellings, or the first value error. The error names its key.
func (pkr *PropKeyResolver) UpdateConfigFromKnownParams(config types.ServiceConfig, params *types.Params) error {
	if params == nil {
		return nil
	}

	// Keys that are not config keys cannot hold one key in two spellings, so
	// leaving them out first changes nothing but the work done. Up to eight
	// keys fit on the stack.
	var buf [8]string

	keys := buf[:0]

	for key := range *params {
		if _, found := pkr.keyFields[strings.ToLower(key)]; found {
			keys = append(keys, key)
		}
	}

	return pkr.applyParams(config, *params, keys, nil)
}

// UpdateConfigFromParams updates a config from send params.
//
// The reserved params set by [types.Params] helpers (title, message and level) are
// applied when the config declares them and skipped otherwise, so every service
// accepts them. The params named in payloadKeys fill the service's payload and
// are skipped too. They match exactly, as the payload reads them. Any other
// param without a matching key is an error.
//
// Params that give one key in two spellings, such as disabletls and DisableTLS,
// are rejected with [ErrConflictingKeys] before any is applied. Params are
// applied in key order, with deprecated keys first, so a replacement key wins
// when params hold both. The first error stops the update.
//
// Parameters:
//   - config: the config to update, or nil for the resolver's own config.
//   - params: the send params, or nil for none.
//   - payloadKeys: the param keys that fill the payload instead of the config.
//
// Returns:
//   - error: an error wrapping [ErrConflictingKeys] for one key in two
//     spellings, or the first unknown key or value error. The error names
//     its key.
func (pkr *PropKeyResolver) UpdateConfigFromParams(
	config types.ServiceConfig,
	params *types.Params,
	payloadKeys ...string,
) error {
	if params == nil {
		return nil
	}

	// Up to eight keys fit on the stack.
	var buf [8]string

	keys := buf[:0]
	for key := range *params {
		keys = append(keys, key)
	}

	return pkr.applyParams(config, *params, keys, payloadKeys)
}

// applyParams updates a config from the send params under keys, as
// [PropKeyResolver.UpdateConfigFromParams] describes.
//
// Parameters:
//   - config: the config to update, or nil for the resolver's own config.
//   - params: the send params.
//   - keys: the keys of params to apply. The function reorders them.
//   - payloadKeys: the param keys that fill the payload instead of the config.
//
// Returns:
//   - error: an error wrapping [ErrConflictingKeys] for one key in two
//     spellings, or the first unknown key or value error. The error names
//     its key.
func (pkr *PropKeyResolver) applyParams(
	config types.ServiceConfig,
	params types.Params,
	keys, payloadKeys []string,
) error {
	confValue := pkr.configValueOrInternal(config)

	slices.Sort(keys)

	var first func(string) bool
	if len(pkr.deprecatedKeys) > 0 {
		first = pkr.isDeprecated
	}

	return applyKeys(keys, pkr.keys, paramKeySource, first, func(key, lower string) error {
		if pkr.isUndeclaredReserved(lower) || slices.Contains(payloadKeys, key) {
			return nil
		}

		return pkr.set(confValue, lower, params[key])
	})
}

// configValueOrInternal returns the reflect.Value of the config, or the internal config if config is nil.
//
// Parameters:
//   - config: the config, or nil.
//
// Returns:
//   - reflect.Value: the config's struct value.
func (pkr *PropKeyResolver) configValueOrInternal(config types.ServiceConfig) reflect.Value {
	if config != nil {
		return configValue(config)
	}

	return pkr.confValue
}

// defaultConfig returns the config's defaults, building them on first use.
//
// A resolver not made by [NewPropKeyResolver] has no shared defaults, so it
// builds them on each call.
//
// Returns:
//   - *configDefaults: the defaults of the resolver's config type.
func (pkr *PropKeyResolver) defaultConfig() *configDefaults {
	if pkr.defaults == nil {
		defaults := &configDefaults{err: nil, invalid: nil, value: reflect.Value{}, copied: nil, parsed: nil, once: sync.Once{}}
		defaults.build(pkr.confValue.Type(), pkr.fields)

		return defaults
	}

	pkr.defaults.once.Do(func() { pkr.defaults.build(pkr.confValue.Type(), pkr.fields) })

	return pkr.defaults
}

// fieldIsDefault reports whether a formatted value is the default of a field.
//
// The value matches the default as written in its tag, or as the formatter
// prints it, as [PropKeyResolver.IsDefault] describes.
//
// Parameters:
//   - field: the field to check.
//   - value: the formatted value.
//
// Returns:
//   - bool: whether value is the field's default.
func (pkr *PropKeyResolver) fieldIsDefault(field *FieldInfo, value string) bool {
	if field.DefaultValue == value {
		return true
	}

	// A resolver without a bound config has no type to format the default with.
	if !pkr.confValue.IsValid() {
		return false
	}

	defaults := pkr.defaultConfig()
	if defaults.invalid[field.Name] {
		return false
	}

	formatted, err := GetConfigFieldString(defaults.value, field)

	return err == nil && formatted == value
}

// isDeprecated reports whether a key is a deprecated key.
//
// Parameters:
//   - lower: the key in lower case.
//
// Returns:
//   - bool: true for a deprecated key.
func (pkr *PropKeyResolver) isDeprecated(lower string) bool {
	_, found := pkr.deprecatedKeys[lower]

	return found
}

// isUndeclaredReserved reports whether key is a reserved param that the config does
// not declare.
//
// Parameters:
//   - key: the param key in lower case.
//
// Returns:
//   - bool: true when the param should be skipped.
func (pkr *PropKeyResolver) isUndeclaredReserved(key string) bool {
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
//
// Parameters:
//   - target: the config struct value to set.
//   - key: the property key in lower case.
//   - value: the formatted property value.
//
// Returns:
//   - error: an error wrapping [ErrInvalidConfigKey] that lists the valid
//     keys, [ErrInvalidValueForType], or the value's parse error, or nil.
func (pkr *PropKeyResolver) set(target reflect.Value, key, value string) error {
	if field, found := pkr.keyFields[key]; found {
		valid, err := SetConfigField(target, field, value)
		if !valid && err == nil {
			return ErrInvalidValueForType
		}

		return err
	}

	return fmt.Errorf("%w: %v (valid keys: %v)", ErrInvalidConfigKey, key, pkr.keys)
}

// build sets each keyed field of a new config of configType to its default.
//
// A field without a default tag, or with an empty one, keeps its zero value. A
// field whose default tag does not parse is recorded as invalid, and the first
// one's parse error is kept. Each valid field is listed as copied or parsed by
// whether its default can be copied without sharing memory.
//
// Parameters:
//   - configType: the config struct type.
//   - fields: the keyed fields of the config, each once, sorted by field name.
func (d *configDefaults) build(configType reflect.Type, fields []*FieldInfo) {
	d.value = reflect.New(configType).Elem()
	d.invalid = map[string]bool{}

	for _, field := range fields {
		if field.DefaultValue == "" {
			d.copied = append(d.copied, field)

			continue
		}

		valid, err := SetConfigField(d.value, field, field.DefaultValue)
		if err == nil && !valid {
			err = ErrInvalidValueForType
		}

		switch {
		case err != nil:
			d.invalid[field.Name] = true
			d.err = cmp.Or(d.err, err)
		case isScalarKind(field.Type.Kind()):
			d.copied = append(d.copied, field)
		default:
			d.parsed = append(d.parsed, field)
		}
	}
}

// configValue returns the indirect reflect.Value of the config.
//
// Parameters:
//   - config: the config, or a pointer to it.
//
// Returns:
//   - reflect.Value: the config's struct value.
func configValue(config types.ServiceConfig) reflect.Value {
	return reflect.Indirect(reflect.ValueOf(config))
}

// isScalarKind reports whether a value of kind holds no reference to shared
// memory, so a copy of it is independent of the original.
//
// Parameters:
//   - kind: the kind of the value.
//
// Returns:
//   - bool: true for booleans, numbers, and strings.
func isScalarKind(kind reflect.Kind) bool {
	return kind == reflect.Bool || kind == reflect.String || util.IsNumeric(kind)
}
