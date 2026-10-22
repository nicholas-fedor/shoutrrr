package format_test

import (
	"net/url"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
)

// benchConfigURL is an ntfy URL that sets keys of each kind the formatter
// supports: strings, an enum, booleans, and string slices. ntfy has the most
// keyed fields of any service, so its config is the costliest to reflect over.
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
