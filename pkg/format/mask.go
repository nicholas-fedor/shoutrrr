package format

import (
	"net/url"
	"reflect"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// URLMasker is implemented by a config whose URL holds secrets that its
// sensitive fields cannot mask, such as custom headers kept outside the
// config's fields, or a token type that cannot hold [MaskedValue].
type URLMasker interface {
	// MaskedURL returns the config's URL with every secret replaced by
	// [MaskedValue].
	MaskedURL() *url.URL
}

// MaskedURL returns the URL of a config with its secrets replaced by
// [MaskedValue], so the URL can be shown or logged.
//
// A config that implements [URLMasker] masks its own URL. For any other
// config, the URL is built from a copy whose [FieldInfo.Sensitive] fields
// with a value hold [MaskedValue] instead. A sensitive field that is not a
// string is cleared, since it cannot hold the placeholder.
//
// Parameters:
//   - config: the service config. It is not changed.
//
// Returns:
//   - *url.URL: the config's URL with its secrets masked.
func MaskedURL(config types.ServiceConfig) *url.URL {
	if masker, ok := config.(URLMasker); ok {
		return masker.MaskedURL()
	}

	value := reflect.ValueOf(config)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return config.GetURL()
	}

	masked := reflect.New(value.Type())
	masked.Elem().Set(value)

	// Deprecated fields move into their replacements first, so a secret set
	// through a deprecated field is masked too. A config with invalid
	// deprecated tags keeps them, and its GetURL cannot read them either.
	moveDeprecatedFields(masked.Elem())

	fields := getStructFieldInfo(masked.Elem().Type(), nil)
	for i := range fields {
		fieldValue := masked.Elem().FieldByName(fields[i].Name)
		if !fields[i].Sensitive || fieldValue.IsZero() {
			continue
		}

		if fieldValue.Kind() == reflect.String {
			fieldValue.SetString(MaskedValue)
		} else {
			fieldValue.SetZero()
		}
	}

	if maskedConfig, ok := reflect.TypeAssert[types.ServiceConfig](masked); ok {
		return maskedConfig.GetURL()
	}

	return config.GetURL()
}
