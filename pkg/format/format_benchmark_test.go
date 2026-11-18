package format_test

import (
	"net/url"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/gotify"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// benchConfigURL is an ntfy URL that sets keys of each kind the formatter
// supports.
//
// The kinds are strings, an enum, booleans, and string slices. ntfy has the
// most keyed fields of any service, so its config is the costliest to reflect
// over.
const benchConfigURL = "ntfy://user:pass@ntfy.example.com/alerts?title=Backup&priority=high" +
	"&tags=warning,skull&markdown=yes&cache=no&click=https://example.com" +
	"&actions=view,Open,https://example.com;http,Ping,https://example.com/ping"

// BenchmarkNewPropKeyResolver measures building a resolver, which reflects over
// the config's fields. Every service URL that is read or written builds one.
func BenchmarkNewPropKeyResolver(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = format.NewPropKeyResolver(&ntfy.Config{})
	}
}

// BenchmarkSetDefaultProps measures setting every keyed field of a config to
// its default, as each Initialize does. ntfy has as many keyed fields as any
// service, deprecated keys among them.
func BenchmarkSetDefaultProps(b *testing.B) {
	resolver := format.NewPropKeyResolver(&ntfy.Config{})
	config := &ntfy.Config{}

	b.ReportAllocs()

	for b.Loop() {
		if err := resolver.SetDefaultProps(config); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPropKeyResolverSet measures setting a config key from its string,
// for each kind of field.
func BenchmarkPropKeyResolverSet(b *testing.B) {
	// values lists a key of each field kind, with a value to set.
	values := []struct{ kind, key, value string }{
		{kind: "string", key: "title", value: "Backup"},
		{kind: "enum", key: "priority", value: "high"},
		{kind: "bool", key: "markdown", value: "yes"},
		{kind: "slice", key: "tags", value: "warning,skull,fire"},
	}

	for _, kv := range values {
		b.Run(kv.kind, func(b *testing.B) {
			resolver := format.NewPropKeyResolver(&ntfy.Config{})

			b.ReportAllocs()

			for b.Loop() {
				if err := resolver.Set(kv.key, kv.value); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkUpdateConfigFromParams measures applying send params to a copy of
// the config, as every send that has params does. The known case also carries
// params that are not config keys, as a webhook's payload fields are.
func BenchmarkUpdateConfigFromParams(b *testing.B) {
	resolver := format.NewPropKeyResolver(&ntfy.Config{})

	// cases lists the update functions with the params each applies.
	cases := []struct {
		update func(types.ServiceConfig, *types.Params) error
		params types.Params
		name   string
	}{
		{
			name:   "params",
			params: types.Params{"title": "Backup", "Priority": "high"},
			update: func(config types.ServiceConfig, params *types.Params) error {
				return resolver.UpdateConfigFromParams(config, params)
			},
		},
		{
			name:   "known params",
			params: types.Params{"title": "Backup", "Priority": "high", "source": "backup", "host": "db1"},
			update: resolver.UpdateConfigFromKnownParams,
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				config := ntfy.Config{}
				if err := tc.update(&config, &tc.params); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkDeprecatedParams measures reporting the deprecated keys among send
// params, as services with deprecated keys do on each send. The usual send
// holds none.
func BenchmarkDeprecatedParams(b *testing.B) {
	resolver := format.NewPropKeyResolver(&ntfy.Config{})

	// cases lists params without and with a deprecated key.
	cases := []struct {
		params types.Params
		name   string
	}{
		{name: "none", params: types.Params{"title": "Backup", "level": "warning"}},
		{name: "deprecated", params: types.Params{"title": "Backup", "DisableTLSVerification": "yes"}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				_ = resolver.DeprecatedParams(&tc.params)
			}
		})
	}
}

// BenchmarkSyncDeprecatedFields measures copying the deprecated fields of a
// config into their replacements, as Gotify, MQTT, and ntfy do on each send
// and each GetURL. Each sync runs on a fresh copy of the config, as a send
// does, so a set deprecated field is copied into its replacement every time.
// The usual config sets no deprecated field.
func BenchmarkSyncDeprecatedFields(b *testing.B) {
	// cases lists ntfy configs without and with a deprecated field set.
	cases := []struct {
		name   string
		config ntfy.Config
		used   int
	}{
		{name: "unset", config: ntfy.Config{Topic: "alerts"}, used: 0},
		{
			name: "set",
			//nolint:staticcheck // The benchmark sets the deprecated field on purpose.
			config: ntfy.Config{Topic: "alerts", DisableTLSVerification: true},
			used:   1,
		},
	}

	for i := range cases {
		tc := &cases[i]

		b.Run(tc.name, func(b *testing.B) {
			config := new(tc.config)

			used, err := format.SyncDeprecatedFields(config)
			if err != nil {
				b.Fatal(err)
			}

			if len(used) != tc.used {
				b.Fatalf("%d deprecated fields reported, want %d", len(used), tc.used)
			}

			b.ReportAllocs()

			for b.Loop() {
				*config = tc.config
				_, _ = format.SyncDeprecatedFields(config)
			}
		})
	}
}

// BenchmarkKeepsDefault measures checking whether a send keeps a property's
// default, as services do on each send to map the message level.
//
// Discord's color and ntfy's priority have default tags that differ in text
// from the printed value, so the check compares against the parsed default.
// Gotify's priority default matches its printed value.
func BenchmarkKeepsDefault(b *testing.B) {
	// configs lists the service configs and the key each send checks.
	configs := []struct {
		config types.ServiceConfig
		name   string
		key    string
	}{
		{config: &discord.Config{}, name: "discord color", key: "color"},
		{config: &ntfy.Config{}, name: "ntfy priority", key: "priority"},
		{config: &gotify.Config{}, name: "gotify priority", key: "priority"},
	}

	// params are send params that set the level and a title, as a router send
	// with a level does.
	params := types.Params{types.LevelKey: "warn", types.TitleKey: "Backup"}

	for _, tc := range configs {
		b.Run(tc.name, func(b *testing.B) {
			resolver := format.NewPropKeyResolver(tc.config)
			if err := resolver.SetDefaultProps(tc.config); err != nil {
				b.Fatal(err)
			}

			if !resolver.KeepsDefault(tc.config, &params, tc.key) {
				b.Fatalf("%s: the default config does not keep its default", tc.name)
			}

			b.ReportAllocs()

			for b.Loop() {
				_ = resolver.KeepsDefault(tc.config, &params, tc.key)
			}
		})
	}
}

// BenchmarkBuildQuery measures writing a config's keys as a URL query, leaving
// out the defaults.
func BenchmarkBuildQuery(b *testing.B) {
	config := benchConfig(b)
	resolver := format.NewPropKeyResolver(config)

	b.ReportAllocs()

	for b.Loop() {
		_ = format.BuildQuery(&resolver)
	}
}

// BenchmarkConfigURLRoundTrip measures reading a config from its URL and
// writing it back, as a service does when it is located and saved.
func BenchmarkConfigURLRoundTrip(b *testing.B) {
	serviceURL, err := url.Parse(benchConfigURL)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		config := &ntfy.Config{}
		if err := config.SetURL(serviceURL); err != nil {
			b.Fatal(err)
		}

		_ = config.GetURL()
	}
}

// benchConfig returns the config [benchConfigURL] sets.
//
// Parameters:
//   - b: the benchmark.
//
// Returns:
//   - *ntfy.Config: the config read from the URL.
func benchConfig(b *testing.B) *ntfy.Config {
	b.Helper()

	serviceURL, err := url.Parse(benchConfigURL)
	if err != nil {
		b.Fatal(err)
	}

	config := &ntfy.Config{}
	if err := config.SetURL(serviceURL); err != nil {
		b.Fatal(err)
	}

	return config
}

// BenchmarkCheckEnumValue measures checking an enum field of a config, as
// each send of a service with one does.
//
// A defined value must not allocate.
func BenchmarkCheckEnumValue(b *testing.B) {
	// modes is an enum with three names, as Signal's text modes have.
	modes := format.CreateEnumFormatter([]string{"None", "Normal", "Styled"})

	// values lists the first and last values of the enum, and one past its last.
	values := []struct {
		name  string
		value int
	}{
		{name: "first", value: 0},
		{name: "last", value: 2},
		{name: "undefined", value: 3},
	}

	for _, value := range values {
		b.Run(value.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				_ = format.CheckEnumValue(modes, value.value)
			}
		})
	}
}
