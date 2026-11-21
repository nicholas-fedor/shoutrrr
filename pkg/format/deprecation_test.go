package format

import (
	"net/url"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// deprecatedConfig is a service config with deprecated keys and fields, as a
// service declares them after renaming an option.
type deprecatedConfig struct {
	standard.EnumlessConfig

	// SkipVerify replaces Insecure and keeps the old keys as deprecated keys.
	SkipVerify bool `default:"No" deprecatedkeys:"insecure,NoVerify" desc:"Skip verification" key:"skipverify"`
	// Insecure is the deprecated field that SkipVerify replaces.
	Insecure bool `deprecated:"SkipVerify"`
	// Token replaces OldToken and is a secret.
	Token string `default:"" key:"token" sensitive:"true"`
	// OldToken is the deprecated field that Token replaces.
	OldToken string `deprecated:"Token"`
	// Pin replaces OldPin and is a secret that cannot hold [MaskedValue].
	Pin int `default:"0" key:"pin" sensitive:"true"`
	// OldPin is the deprecated field that Pin replaces.
	OldPin int `deprecated:"Pin"`
}

// urlStub provides the URL methods of [types.ServiceConfig] for configs whose
// URLs the tests do not use.
type urlStub struct {
	standard.EnumlessConfig
}

// missingReplacementConfig names a replacement field that does not exist.
type missingReplacementConfig struct {
	urlStub

	// Old names a missing field.
	Old bool `deprecated:"Missing"`
}

// mismatchedReplacementConfig names a replacement of another type.
type mismatchedReplacementConfig struct {
	urlStub

	// New is a string.
	New string
	// Old is a bool.
	Old bool `deprecated:"New"`
}

// selfReplacementConfig names the deprecated field itself.
type selfReplacementConfig struct {
	urlStub

	// Old names itself.
	Old bool `deprecated:"Old"`
}

// unexportedReplacementConfig names an unexported replacement.
type unexportedReplacementConfig struct {
	urlStub

	// Old names an unexported field.
	Old bool `deprecated:"replacement"`
	// replacement is unexported.
	replacement bool //nolint:unused // Only the tag of Old refers to it.
}

// promotedFields holds a field that an embedding config promotes.
type promotedFields struct {
	// New is promoted into promotedReplacementConfig.
	New bool
}

// promotedReplacementConfig names a replacement promoted from an embedded
// pointer, which is nil in the tests.
type promotedReplacementConfig struct {
	urlStub
	*promotedFields

	// Old names the promoted field.
	Old bool `deprecated:"New"`
}

// chainedReplacementConfig names a replacement that is deprecated itself.
type chainedReplacementConfig struct {
	urlStub

	// Newest replaces Newer.
	Newest bool
	// Newer replaces Old and is deprecated itself.
	Newer bool `deprecated:"Newest"`
	// Old names the deprecated Newer.
	Old bool `deprecated:"Newer"`
}

// replacedReplacementConfig has a deprecated field whose replacement's keys
// another key replaces.
type replacedReplacementConfig struct {
	urlStub

	// Old names the replaced Scheme.
	Old string `deprecated:"Scheme"`
	// Scheme's key is replaced by the tls key.
	Scheme string `key:"scheme" replacedby:"tls" replacedvalues:"http=yes"`
	// TLS replaces the scheme key.
	TLS bool `key:"tls"`
}

// partlyInvalidConfig has a valid pair before an invalid one.
type partlyInvalidConfig struct {
	urlStub

	// New replaces Old.
	New bool
	// Old is a valid deprecated field.
	Old bool `deprecated:"New"`
	// Broken names a missing field.
	Broken bool `deprecated:"Missing"`
}

// sharedReplacementConfig has two deprecated fields that name one replacement.
// Zeta is declared first, so it wins when both are set, though Alpha sorts
// first by name.
type sharedReplacementConfig struct {
	urlStub

	// New replaces Zeta and Alpha.
	New string
	// Zeta is the first deprecated field that New replaces.
	Zeta string `deprecated:"New"`
	// Alpha is the second deprecated field that New replaces.
	Alpha string `deprecated:"New"`
}

// zeroMappedSharedConfig has a replaced field whose mapping can give its
// replacement the zero value, declared before a deprecated field that names
// the same replacement.
type zeroMappedSharedConfig struct {
	urlStub

	// Switch maps off onto On's zero value.
	Switch string `key:"switch" replacedby:"on" replacedvalues:"off=no,on=yes"`
	// Legacy names On as its replacement.
	Legacy bool `deprecated:"On"`
	// On replaces Switch and Legacy.
	On bool `key:"on"`
}

// firstUseConfig has a deprecated field. Only
// [TestDeprecatedFieldPairsFirstUse] uses it, so its pairs are built there.
type firstUseConfig struct {
	urlStub

	// New replaces Old.
	New string
	// Old is the deprecated field that New replaces.
	Old string `deprecated:"New"`
}

// replacedConfig has deprecated keys whose values another key takes over, as
// ntfy's scheme key does for disabletls. Scheme keeps its keys, so its default
// is still set.
type replacedConfig struct {
	URLKeys
	urlStub

	// Scheme's keys are deprecated. A value of http sets DisableTLS to yes, and
	// https sets it to no.
	Scheme string `default:"https" key:"scheme,proto" replacedby:"disabletls" replacedvalues:"http=yes,https=no"`
	// DisableTLS replaces the scheme keys.
	DisableTLS bool `default:"No" key:"disabletls"`
	// Label's key is deprecated, and its values pass to the name key as written.
	Label string `key:"label" replacedby:"name"`
	// Name replaces the label key.
	Name string `key:"name"`
}

// unknownReplacementKeyConfig names a replacement key that no field has.
type unknownReplacementKeyConfig struct {
	urlStub

	// Old names a missing key.
	Old string `key:"old" replacedby:"missing"`
}

// unkeyedReplacedConfig gives a replaced key to a field without a key tag.
type unkeyedReplacedConfig struct {
	urlStub

	// Old has no keys to replace.
	Old string `replacedby:"new"`
	// New would replace the old field's keys.
	New string `key:"new"`
}

// nonStringReplacedConfig gives a replaced key to a field that is not a string.
type nonStringReplacedConfig struct {
	urlStub

	// Old is a bool.
	Old bool `key:"old" replacedby:"new"`
	// New replaces the old key.
	New bool `key:"new"`
}

// chainedReplacedConfig names a replacement key whose own field is replaced.
type chainedReplacedConfig struct {
	urlStub

	// Old names the newer key, which is replaced itself.
	Old string `key:"old" replacedby:"newer"`
	// Newer names the newest key.
	Newer string `key:"newer" replacedby:"newest"`
	// Newest replaces the newer key.
	Newest string `key:"newest"`
}

// unparsableReplacedValueConfig maps a value onto one the replacement cannot
// hold.
type unparsableReplacedValueConfig struct {
	urlStub

	// Old maps on onto maybe, which is not a bool.
	Old string `key:"old" replacedby:"new" replacedvalues:"on=maybe"`
	// New replaces the old key.
	New bool `key:"new"`
}

// malformedReplacedValuesConfig has a value mapping without an equals sign.
type malformedReplacedValuesConfig struct {
	urlStub

	// Old has a malformed mapping.
	Old string `key:"old" replacedby:"new" replacedvalues:"on"`
	// New replaces the old key.
	New bool `key:"new"`
}

// passThroughReplacedConfig replaces a field's key and deprecated key with
// the key of an int field, without a mapping, so a value passes as written.
type passThroughReplacedConfig struct {
	urlStub

	// Old passes its value to Count as written.
	Old string `deprecatedkeys:"older" key:"old" replacedby:"count"`
	// Count replaces the old keys.
	Count int `key:"count"`
}

// TestSyncDeprecatedFields verifies how deprecated fields reach their
// replacements.
//
//   - A set deprecated field moves into a replacement at its zero value.
//   - A set replacement wins.
//   - Each set deprecated field is reported, whichever value wins.
//   - The deprecated field keeps its value, and a second sync changes nothing.
func TestSyncDeprecatedFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		config *deprecatedConfig
		want   *deprecatedConfig
		name   string
		used   []Deprecation
	}{
		{
			name:   "nothing set",
			config: &deprecatedConfig{},
			want:   &deprecatedConfig{},
			used:   []Deprecation{},
		},
		{
			name:   "only replacements set",
			config: &deprecatedConfig{SkipVerify: true, Token: "new"},
			want:   &deprecatedConfig{SkipVerify: true, Token: "new"},
			used:   []Deprecation{},
		},
		{
			name:   "deprecated fields set",
			config: &deprecatedConfig{Insecure: true, OldToken: "old", OldPin: 7},
			want:   &deprecatedConfig{SkipVerify: true, Insecure: true, Token: "old", OldToken: "old", Pin: 7, OldPin: 7},
			used: []Deprecation{
				{Name: "Insecure", Replacement: "SkipVerify"},
				{Name: "OldPin", Replacement: "Pin"},
				{Name: "OldToken", Replacement: "Token"},
			},
		},
		{
			name:   "both set",
			config: &deprecatedConfig{Token: "new", OldToken: "old"},
			want:   &deprecatedConfig{Token: "new", OldToken: "old"},
			used:   []Deprecation{{Name: "OldToken", Replacement: "Token"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			used, err := SyncDeprecatedFields(tt.config)
			require.NoError(t, err)
			assert.Equal(t, tt.used, used)
			assert.Equal(t, tt.want, tt.config)

			again, err := SyncDeprecatedFields(tt.config)
			require.NoError(t, err)
			assert.Equal(t, tt.used, again, "a second sync should report the same fields")
			assert.Equal(t, tt.want, tt.config, "a second sync should change nothing")
		})
	}
}

// TestSyncReplacedFields verifies how a field whose keys another key
// replaces reaches the replacement's field when code sets it.
//
//   - A value other than the field's default, in any case, maps onto the
//     replacement while the replacement holds its zero value.
//   - A set replacement wins, and the field is still reported.
//   - The default, in any case, and the zero value count as unset.
//   - A field without a value mapping passes its value as written.
func TestSyncReplacedFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		config *replacedConfig
		want   *replacedConfig
		name   string
		used   []Deprecation
	}{
		{name: "zero value", config: &replacedConfig{}, want: &replacedConfig{}, used: []Deprecation{}},
		{
			name:   "default",
			config: &replacedConfig{Scheme: "https"},
			want:   &replacedConfig{Scheme: "https"},
			used:   []Deprecation{},
		},
		{
			name:   "default in another case",
			config: &replacedConfig{Scheme: "HTTPS", DisableTLS: true},
			want:   &replacedConfig{Scheme: "HTTPS", DisableTLS: true},
			used:   []Deprecation{},
		},
		{
			name:   "mapped value",
			config: &replacedConfig{Scheme: "http"},
			want:   &replacedConfig{Scheme: "http", DisableTLS: true},
			used:   []Deprecation{{Name: "Scheme", Replacement: "DisableTLS"}},
		},
		{
			name:   "mapped value in another case",
			config: &replacedConfig{Scheme: "HTTP"},
			want:   &replacedConfig{Scheme: "HTTP", DisableTLS: true},
			used:   []Deprecation{{Name: "Scheme", Replacement: "DisableTLS"}},
		},
		{
			name:   "replacement set",
			config: &replacedConfig{Label: "old", Name: "new"},
			want:   &replacedConfig{Label: "old", Name: "new"},
			used:   []Deprecation{{Name: "Label", Replacement: "Name"}},
		},
		{
			name:   "value without a mapping",
			config: &replacedConfig{Label: "old"},
			want:   &replacedConfig{Label: "old", Name: "old"},
			used:   []Deprecation{{Name: "Label", Replacement: "Name"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			used, err := SyncDeprecatedFields(tt.config)
			require.NoError(t, err)
			assert.Equal(t, tt.used, used)
			assert.Equal(t, tt.want, tt.config)
		})
	}
}

// TestSyncReplacedFieldsUnmappedValue verifies that a value the mapping does
// not list fails, naming the field. That field is not copied, and a later
// deprecated field still is, so a GetURL that ignores the error keeps it.
func TestSyncReplacedFieldsUnmappedValue(t *testing.T) {
	t.Parallel()

	config := &replacedConfig{Scheme: "ftp", Label: "old"}

	used, err := SyncDeprecatedFields(config)
	require.ErrorIs(t, err, ErrInvalidValueForType)
	require.ErrorContains(t, err, "Scheme: ")
	assert.Nil(t, used)
	assert.False(t, config.DisableTLS)
	assert.Equal(t, "old", config.Name, "a later deprecated field should still be copied")
}

// TestSyncReplacedFieldsLookAlikeDefault verifies that a look-alike of the
// default, which Unicode case folding matches, is not taken as the default.
// It fails as the URL path rejects it.
func TestSyncReplacedFieldsLookAlikeDefault(t *testing.T) {
	t.Parallel()

	_, err := SyncDeprecatedFields(&replacedConfig{Scheme: "httpſ"})
	require.ErrorIs(t, err, ErrInvalidValueForType)
}

// TestSyncReplacedFieldsPassThrough verifies that a value passed as written
// moves into its replacement when the replacement can hold it. A value it
// cannot hold fails, naming the field, and a move leaves it in place. The value
// is checked even when the replacement already holds a value.
func TestSyncReplacedFieldsPassThrough(t *testing.T) {
	t.Parallel()

	valid := &passThroughReplacedConfig{Old: "5"}

	used, err := SyncDeprecatedFields(valid)
	require.NoError(t, err)
	assert.Equal(t, []Deprecation{{Name: "Old", Replacement: "Count"}}, used)
	assert.Equal(t, 5, valid.Count)

	invalid := &passThroughReplacedConfig{Old: "many"}

	_, err = SyncDeprecatedFields(invalid)
	require.ErrorIs(t, err, ErrInvalidValueForType)
	require.ErrorContains(t, err, "Old: ")
	assert.Zero(t, invalid.Count)

	moveDeprecatedFields(configValue(invalid))
	assert.Equal(t, &passThroughReplacedConfig{Old: "many"}, invalid, "a value the replacement cannot hold should stay")

	// tests lists values set beside a replacement that already holds a value,
	// which is never overwritten, with whether each value is accepted.
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "valid value", value: "5", valid: true},
		{name: "invalid value", value: "many", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &passThroughReplacedConfig{Old: tt.value, Count: 3}

			_, err := SyncDeprecatedFields(config)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrInvalidValueForType)
			}

			assert.Equal(t, 3, config.Count)
		})
	}
}

// TestEnumsOf verifies that the enum formatters come from a new config of the
// type, and that a type that is not a config has none.
func TestEnumsOf(t *testing.T) {
	t.Parallel()

	assert.Equal(t, map[string]types.EnumFormatter{}, enumsOf(reflect.TypeFor[replacedConfig]()))
	assert.Nil(t, enumsOf(reflect.TypeFor[int]()))
}

// TestSyncDeprecatedFieldsInvalid verifies that a config that is not a
// pointer to a struct fails with [ErrInvalidDeprecatedField]. A tag that names
// no usable replacement fails the same way and leaves every field unchanged.
// A replacement must be a direct field that is not deprecated itself. A
// replaced key needs a string field, a replacement key whose field is not
// replaced, and a well-formed mapping onto values the replacement can hold.
func TestSyncDeprecatedFieldsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		config types.ServiceConfig
		name   string
	}{
		{name: "string value", config: stringConfig("x")},
		{name: "pointer to a string", config: new(stringConfig)},
		{name: "struct value", config: valueConfig{}},
		{name: "nil pointer", config: (*deprecatedConfig)(nil)},
		{name: "missing replacement", config: &missingReplacementConfig{Old: true}},
		{name: "replacement of another type", config: &mismatchedReplacementConfig{Old: true}},
		{name: "replacement is itself", config: &selfReplacementConfig{Old: true}},
		{name: "unexported replacement", config: &unexportedReplacementConfig{Old: true}},
		{name: "promoted replacement", config: &promotedReplacementConfig{Old: true}},
		{name: "deprecated replacement", config: &chainedReplacementConfig{Old: true}},
		{name: "replaced replacement", config: &replacedReplacementConfig{Old: "x"}},
		{name: "unknown replacement key", config: &unknownReplacementKeyConfig{Old: "x"}},
		{name: "replaced field that is not a string", config: &nonStringReplacedConfig{Old: true}},
		{name: "replaced field without keys", config: &unkeyedReplacedConfig{Old: "x"}},
		{name: "replaced replacement key", config: &chainedReplacedConfig{Old: "x"}},
		{name: "mapped value the replacement cannot hold", config: &unparsableReplacedValueConfig{Old: "on"}},
		{name: "malformed value mapping", config: &malformedReplacedValuesConfig{Old: "on"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			used, err := SyncDeprecatedFields(tt.config)
			require.ErrorIs(t, err, ErrInvalidDeprecatedField)
			assert.Nil(t, used)
		})
	}

	// The valid pair before the broken tag must not be kept either, so each
	// sync reads the tags again and fails the same way.
	config := &partlyInvalidConfig{Old: true}

	for attempt := range 2 {
		used, err := SyncDeprecatedFields(config)
		require.ErrorIs(t, err, ErrInvalidDeprecatedField, "attempt %d", attempt)
		assert.Nil(t, used, "attempt %d", attempt)
		assert.Equal(t, &partlyInvalidConfig{Old: true}, config, "no field should change when a tag is invalid")

		_, cached := deprecatedFieldPairsByType.Load(reflect.TypeFor[partlyInvalidConfig]())
		assert.False(t, cached, "attempt %d: a type with an invalid tag should not be cached", attempt)
	}
}

// TestSyncDeprecatedFieldsSharedReplacement verifies that when two deprecated
// fields name one replacement, the first one declared that is set wins, for a
// sync and for a move, while the report stays sorted by name. A value mapped
// onto the replacement's zero value counts as set too.
func TestSyncDeprecatedFieldsSharedReplacement(t *testing.T) {
	t.Parallel()

	config := &sharedReplacementConfig{Zeta: "z", Alpha: "a"}
	used, err := SyncDeprecatedFields(config)
	require.NoError(t, err)
	assert.Equal(t, "z", config.New)
	assert.Equal(t, []Deprecation{
		{Name: "Alpha", Replacement: "New"},
		{Name: "Zeta", Replacement: "New"},
	}, used)

	moved := &sharedReplacementConfig{Zeta: "z", Alpha: "a"}
	moveDeprecatedFields(configValue(moved))
	assert.Equal(t, &sharedReplacementConfig{New: "z"}, moved)

	zero := &zeroMappedSharedConfig{Switch: "off", Legacy: true}
	_, err = SyncDeprecatedFields(zero)
	require.NoError(t, err)
	assert.False(t, zero.On, "a value mapped onto the zero value should still win")

	movedZero := &zeroMappedSharedConfig{Switch: "off", Legacy: true}
	moveDeprecatedFields(configValue(movedZero))
	assert.Equal(t, &zeroMappedSharedConfig{}, movedZero)
}

// TestDeprecatedFieldPairsFirstUse verifies that concurrent first calls for a
// type all get its pairs and store them, and that a later call reuses the
// stored pairs instead of reading the tags again. Run it with -race.
func TestDeprecatedFieldPairsFirstUse(t *testing.T) {
	t.Parallel()

	const workers = 16

	structType := reflect.TypeFor[firstUseConfig]()

	// A repeated run, such as with -count=2, starts with the pairs stored by
	// the run before. Only this test reads this type, so clearing it is safe.
	deprecatedFieldPairsByType.Delete(structType)

	want := []deprecatedFieldPair{{
		deprecation:      Deprecation{Name: "Old", Replacement: "New"},
		replaced:         nil,
		replacementField: nil,
		deprecated:       2,
		replacement:      1,
	}}

	var wg sync.WaitGroup

	results := make([][]deprecatedFieldPair, workers)
	errs := make([]error, workers)

	for i := range workers {
		wg.Go(func() {
			results[i], errs[i] = deprecatedFieldPairs(structType)
		})
	}

	wg.Wait()

	for i := range workers {
		require.NoError(t, errs[i], "call %d", i)
		assert.Equal(t, want, results[i], "call %d", i)
	}

	// The entry is read before any other call, so only the workers can have
	// stored it.
	cached, found := deprecatedFieldPairsByType.Load(structType)
	require.True(t, found, "the first calls should store the pairs")

	stored, isPairs := cached.([]deprecatedFieldPair)
	require.True(t, isPairs)
	require.Equal(t, want, stored)

	again, err := deprecatedFieldPairs(structType)
	require.NoError(t, err)
	require.Len(t, again, 1)
	assert.Same(t, &stored[0], &again[0], "a later call should reuse the stored pairs")
}

// TestMoveDeprecatedFields verifies that moving copies each set deprecated
// field into a replacement at its zero value and then clears it, and that a
// struct with an invalid tag is left unchanged. A replaced field moves its
// mapped value, and a value without a mapping stays.
func TestMoveDeprecatedFields(t *testing.T) {
	t.Parallel()

	config := &deprecatedConfig{Insecure: true, Token: "new", OldToken: "old"}
	moveDeprecatedFields(configValue(config))
	assert.Equal(t, &deprecatedConfig{SkipVerify: true, Token: "new"}, config)

	invalid := &partlyInvalidConfig{Old: true}
	moveDeprecatedFields(configValue(invalid))
	assert.Equal(t, &partlyInvalidConfig{Old: true}, invalid)

	replaced := &replacedConfig{Scheme: "http", Label: "old"}
	moveDeprecatedFields(configValue(replaced))
	assert.Equal(t, &replacedConfig{DisableTLS: true, Name: "old"}, replaced)

	unmapped := &replacedConfig{Scheme: "ftp"}
	moveDeprecatedFields(configValue(unmapped))
	assert.Equal(t, &replacedConfig{Scheme: "ftp"}, unmapped, "a value without a mapping should stay")
}

// TestDeprecatedConfigGetURL verifies the pattern a service follows in
// GetURL. A value set only through a deprecated field reaches the URL under
// the replacement key, and deprecated keys never appear. The config itself is
// not changed.
func TestDeprecatedConfigGetURL(t *testing.T) {
	t.Parallel()

	config := &deprecatedConfig{Insecure: true}

	assert.Equal(t, "skipverify=Yes", config.GetURL().RawQuery)
	assert.Equal(t, &deprecatedConfig{Insecure: true}, config)
}

// GetURL returns a URL with the config's query, built from a copy whose
// deprecated fields are synced, as a service's GetURL does.
//
// Returns:
//   - *url.URL: the config URL.
func (c *deprecatedConfig) GetURL() *url.URL {
	synced := *c
	_, _ = SyncDeprecatedFields(&synced)
	resolver := NewPropKeyResolver(&synced)

	return &url.URL{Scheme: "deprecated", RawQuery: BuildQuery(&resolver)}
}

// SetURL ignores the URL. The tests set fields directly.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (c *deprecatedConfig) SetURL(*url.URL) error { return nil }

// GetURL returns an empty URL. The tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (urlStub) GetURL() *url.URL { return &url.URL{} }

// SetURL ignores the URL. The tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (urlStub) SetURL(*url.URL) error { return nil }
