package ntfy

import "testing"

// BenchmarkAPIURL measures building the topic URL of a prepared config, as
// each send does for its request. A host with a hostname is the usual case.
// An empty host takes the default server.
func BenchmarkAPIURL(b *testing.B) {
	// hosts lists a hostname and an empty host.
	hosts := []struct {
		name string
		host string
	}{
		{name: "hostname", host: "ntfy.example.com"},
		{name: "default", host: ""},
	}

	for _, tc := range hosts {
		config := &Config{Host: tc.host, Topic: "alerts"}
		if err := config.prepare(); err != nil {
			b.Fatal(err)
		}

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				_ = config.apiURL(config.Topic)
			}
		})
	}
}

// BenchmarkPrepare measures checking a config before a send, as each send
// does.
func BenchmarkPrepare(b *testing.B) {
	config := &Config{Host: "ntfy.example.com", Topic: "alerts"}

	b.ReportAllocs()

	for b.Loop() {
		if err := config.prepare(); err != nil {
			b.Fatal(err)
		}
	}
}
